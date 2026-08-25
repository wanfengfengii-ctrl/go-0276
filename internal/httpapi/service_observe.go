package httpapi

import (
	"encoding/json"
	"fmt"

	"tomato-bumblebee-pollination-rotation/internal/coverage"
	"tomato-bumblebee-pollination-rotation/internal/cycle"
	"tomato-bumblebee-pollination-rotation/internal/occupancy"
	"tomato-bumblebee-pollination-rotation/internal/rules"
	"tomato-bumblebee-pollination-rotation/internal/store"
)

// Admit registers a colony into a zone for a window. It verifies the current
// zone, occupancy window and task generation, then records the occupancy.
func (s *Service) Admit(id string, req AdmitRequest) (CycleView, error) {
	digest, err := cycle.NormalizeContent(req)
	if err != nil {
		return CycleView{}, NewError(CodeInvalidRequest, "cannot normalize request")
	}
	var view CycleView
	var replay []byte
	err = s.store.Update(func(st *store.State) error {
		if rec, ok := st.Operations[req.OperationID]; ok {
			if rec.Digest != digest {
				return NewError(CodeOperationContentConflict, "operation content conflict")
			}
			replay = []byte(rec.Response)
			return nil
		}
		c, ok := st.Cycles[id]
		if !ok {
			return NewError(CodeNotFound, "cycle not found")
		}
		if c.State.IsTerminal() {
			return NewError(CodeInvalidTransition, "cycle is terminal")
		}
		snap := st.Snapshots[id]
		occ := occupancy.WindowOccupancy{
			ID:         fmt.Sprintf("%s:%s:%d", id, req.ColonyID, req.WindowStart),
			CycleID:    id,
			ColonyID:   req.ColonyID,
			ZoneID:     req.ZoneID,
			Window:     occupancy.Interval{Start: req.WindowStart, End: req.WindowEnd},
			Purpose:    occupancy.PurposeAdmit,
			Generation: c.TaskGeneration,
			Version:    1,
		}
		if !occ.Window.Valid() {
			return NewError(CodeInvalidRequest, "invalid window")
		}
		if err := s.checkZoneColony(&snap, req.ZoneID, req.ColonyID); err != nil {
			return err
		}
		graph := occupancy.NewConflictGraph()
		for _, o := range st.Occupancies[id] {
			graph.Add(o)
		}
		if overlaps := graph.FindColonyOverlaps(occ); len(overlaps) > 0 {
			return NewError(CodeInvalidTransition, "colony window overlap")
		}
		st.Occupancies[id] = append(st.Occupancies[id], occ)
		if c.State == cycle.StatePendingAdmit {
			c.State = cycle.StateInFlight
		}
		ev := cycle.NewEvent(id, cycle.EvidenceAdmitted, req.LogicalTime, c.TaskGeneration, digest, "admitted "+req.ColonyID)
		ev.Seq = c.EvidenceSeq + 1
		c.EvidenceSeq = ev.Seq
		st.Evidence[id] = append(st.Evidence[id], ev)
		c.EvidenceRoot = cycle.EvidenceRoot(st.Evidence[id])
		st.Cycles[id] = c
		view = s.buildCycleView(st, id)
		resp, _ := json.Marshal(view)
		st.Operations[req.OperationID] = cycle.OperationRecord{OperationID: req.OperationID, Digest: digest, Response: string(resp), AppliedAt: req.LogicalTime}
		return nil
	})
	if err != nil {
		return CycleView{}, err
	}
	if replay != nil {
		_ = json.Unmarshal(replay, &view)
	}
	return view, nil
}

func (s *Service) checkZoneColony(snap *rules.RuleSnapshot, zoneID, colonyID string) error {
	found := false
	for _, z := range snap.Zones {
		if z.ID == zoneID {
			found = true
			break
		}
	}
	if !found {
		return NewError(CodeInvalidRequest, "zone not in snapshot: "+zoneID)
	}
	for _, c := range snap.Colonies {
		if c.ID == colonyID {
			return nil
		}
	}
	return NewError(CodeInvalidRequest, "colony not in snapshot: "+colonyID)
}

// Observe validates a roaming observation, performs fixed-point derived
// computation, atomically debits the flight budget and fills the coverage cell.
// Illegal observations only append rejected evidence.
func (s *Service) Observe(id string, req ObservationRequest) (ObservationResult, error) {
	digest, err := cycle.NormalizeContent(req)
	if err != nil {
		return ObservationResult{}, NewError(CodeInvalidRequest, "cannot normalize request")
	}
	var result ObservationResult
	var replay []byte
	var quarantineErr error
	var quarantineEvent cycle.EvidenceEvent
	err = s.store.Update(func(st *store.State) error {
		if rec, ok := st.Operations[req.OperationID]; ok {
			if rec.Digest != digest {
				return NewError(CodeOperationContentConflict, "operation content conflict")
			}
			replay = []byte(rec.Response)
			return nil
		}
		c, ok := st.Cycles[id]
		if !ok {
			return NewError(CodeNotFound, "cycle not found")
		}
		if c.State.IsTerminal() {
			return NewError(CodeInvalidTransition, "cycle is terminal")
		}
		cfg := st.Configs[id]
		effectiveGen := c.TaskGeneration + c.RecoveryGen
		if req.Generation != 0 && req.Generation != effectiveGen {
			ev := cycle.NewEvent(id, cycle.EvidenceRejected, req.LogicalTime, req.Generation, digest, "wrong generation")
			ev.Seq = c.EvidenceSeq + 1
			c.EvidenceSeq = ev.Seq
			st.Evidence[id] = append(st.Evidence[id], ev)
			st.Cycles[id] = c
			result = ObservationResult{Status: "rejected"}
			return NewError(CodeInvalidRequest, "observation generation mismatch")
		}

		// Freeze barrier check. A frozen observation must fail per the barrier
		// version, but its quarantine evidence is an append-only audit record that
		// must persist even though the operation is rejected: the evidence stream
		// and the recovery evidence root both have to include it, even after a
		// restart. Because Update rolls back every mutation when the callback
		// returns an error, the quarantine event cannot be appended here — it is
		// committed in a dedicated transaction after this one rolls back. Return a
		// sentinel so this transaction leaves no partial business state, while the
		// caller commits the evidence separately and surfaces the rejection.
		frozen := false
		for _, b := range st.Barriers[id] {
			if b.Affects(req.ZoneID, req.LogicalTime) {
				frozen = true
				break
			}
		}
		if frozen {
			result = ObservationResult{Status: "quarantine"}
			quarantineErr = NewError(CodeInvalidTransition, "zone frozen")
			quarantineEvent = cycle.NewEvent(id, cycle.EvidenceQuarantine, req.LogicalTime, effectiveGen, digest, "frozen zone")
			return errQuarantineCommit
		}

		obs := coverage.Observation{
			ZoneID:                     req.ZoneID,
			InflorescenceBatchID:       req.InflorescenceBatchID,
			ObservationPoint:           req.ObservationPoint,
			VisitTotal:                 req.VisitTotal,
			PerInflorescence:           req.PerInflorescence,
			BiteMarkInflorescences:     req.BiteMarkInflorescences,
			UnpollinatedInflorescences: req.UnpollinatedInflorescences,
			ReturningBees:              req.ReturningBees,
			OutgoingBees:               req.OutgoingBees,
			DeadBees:                   req.DeadBees,
			SampledInflorescences:      req.SampledInflorescences,
		}
		if err := obs.Validate(); err != nil {
			ev := cycle.NewEvent(id, cycle.EvidenceRejected, req.LogicalTime, effectiveGen, digest, err.Error())
			ev.Seq = c.EvidenceSeq + 1
			c.EvidenceSeq = ev.Seq
			st.Evidence[id] = append(st.Evidence[id], ev)
			st.Cycles[id] = c
			result = ObservationResult{Status: "rejected"}
			return NewError(CodeInvalidRequest, "observation invalid: "+err.Error())
		}

		// Fixed-point derived metrics.
		derived, err := s.computeDerived(req)
		if err != nil {
			ev := cycle.NewEvent(id, cycle.EvidenceRejected, req.LogicalTime, effectiveGen, digest, err.Error())
			ev.Seq = c.EvidenceSeq + 1
			c.EvidenceSeq = ev.Seq
			st.Evidence[id] = append(st.Evidence[id], ev)
			st.Cycles[id] = c
			result = ObservationResult{Status: "rejected"}
			return NewError(CodeInvalidRequest, err.Error())
		}
		if code, ok := coverage.EnvironmentCheck(req.Temperature, cfg.EnvTempLow, cfg.EnvTempHigh); !ok {
			_ = code
			ev := cycle.NewEvent(id, cycle.EvidenceRejected, req.LogicalTime, effectiveGen, digest, "environment out of range")
			ev.Seq = c.EvidenceSeq + 1
			c.EvidenceSeq = ev.Seq
			st.Evidence[id] = append(st.Evidence[id], ev)
			st.Cycles[id] = c
			result = ObservationResult{Status: "rejected"}
			return NewError(CodeInvalidRequest, "environment out of range")
		}

		cellKey := store.CellKey(id, req.ZoneID, req.InflorescenceBatchID, req.ObservationPoint)
		cell, exists := st.CoverageCells[cellKey]
		if !exists {
			ev := cycle.NewEvent(id, cycle.EvidenceRejected, req.LogicalTime, effectiveGen, digest, "unknown observation point")
			ev.Seq = c.EvidenceSeq + 1
			c.EvidenceSeq = ev.Seq
			st.Evidence[id] = append(st.Evidence[id], ev)
			st.Cycles[id] = c
			result = ObservationResult{Status: "rejected"}
			return NewError(CodeInvalidRequest, "unknown coverage cell")
		}
		if cell.Filled {
			return NewError(CodeObservationDuplicate, "observation duplicate conflict")
		}

		// Budget debit.
		acct := st.Budgets[id+":budget"]
		if !acct.Debit(1, effectiveGen, req.LogicalTime) {
			return NewError(CodeInvalidRequest, "insufficient flight budget")
		}
		st.Budgets[id+":budget"] = acct

		cell.Filled = true
		cell.EvidenceSeq = c.EvidenceSeq + 1
		cell.Generation = effectiveGen
		st.CoverageCells[cellKey] = cell

		ev := cycle.NewEvent(id, cycle.EvidenceObservation, req.LogicalTime, effectiveGen, digest, "observation")
		ev.Seq = c.EvidenceSeq + 1
		c.EvidenceSeq = ev.Seq
		st.Evidence[id] = append(st.Evidence[id], ev)
		c.EvidenceRoot = cycle.EvidenceRoot(st.Evidence[id])
		st.Cycles[id] = c

		result = ObservationResult{
			Status:      "valid",
			EvidenceSeq: ev.Seq,
			Derived:     derived,
		}
		resp, _ := json.Marshal(result)
		st.Operations[req.OperationID] = cycle.OperationRecord{OperationID: req.OperationID, Digest: digest, Response: string(resp), AppliedAt: req.LogicalTime}
		return nil
	})
	if err == errQuarantineCommit {
		// The observation was rejected by a freeze barrier. The rejection itself
		// carries no business mutation, so the outer transaction was rolled back
		// on purpose. The quarantine evidence, however, is an append-only audit
		// record that must survive: commit it in its own transaction so that the
		// evidence stream and root digest include the frozen observation even
		// after a restart.
		if commitErr := s.store.Update(func(st *store.State) error {
			c, ok := st.Cycles[id]
			if !ok {
				return NewError(CodeNotFound, "cycle not found")
			}
			quarantineEvent.Seq = c.EvidenceSeq + 1
			c.EvidenceSeq = quarantineEvent.Seq
			st.Evidence[id] = append(st.Evidence[id], quarantineEvent)
			c.EvidenceRoot = cycle.EvidenceRoot(st.Evidence[id])
			st.Cycles[id] = c
			return nil
		}); commitErr != nil {
			return ObservationResult{}, commitErr
		}
		return result, quarantineErr
	}
	if err != nil {
		return ObservationResult{}, err
	}
	if replay != nil {
		_ = json.Unmarshal(replay, &result)
	}
	return result, nil
}

// errQuarantineCommit is a sentinel returned from the observation transaction
// when a frozen observation must be rejected but its quarantine evidence must
// still be committed. It is intercepted by Observe and never surfaced to the
// caller.
var errQuarantineCommit = newQuarantineSentinel()

type quarantineSentinel struct{}

func newQuarantineSentinel() error { return &quarantineSentinel{} }

func (e *quarantineSentinel) Error() string { return "quarantine evidence commit pending" }

func (s *Service) computeDerived(req ObservationRequest) ([]DerivedMetricView, error) {
	rr, err := coverage.ReturnRate(req.ReturningBees, req.OutgoingBees)
	if err != nil {
		return nil, fmt.Errorf("return rate: %w", err)
	}
	br, err := coverage.BiteMarkRate(req.BiteMarkInflorescences, req.SampledInflorescences)
	if err != nil {
		return nil, fmt.Errorf("bite mark rate: %w", err)
	}
	ur, err := coverage.UnpollinatedRate(req.UnpollinatedInflorescences, req.SampledInflorescences)
	if err != nil {
		return nil, fmt.Errorf("unpollinated rate: %w", err)
	}
	return []DerivedMetricView{
		{Name: rr.Name, Value: rr.Value, Scale: rr.Scale, Valid: rr.Valid},
		{Name: br.Name, Value: br.Value, Scale: br.Scale, Valid: br.Valid},
		{Name: ur.Name, Value: ur.Value, Scale: ur.Scale, Valid: ur.Valid},
	}, nil
}

// RetryDevice executes a deterministic device retry. It never fabricates a
// reading on failure; a successful retry records the driver result. Failed
// attempts remain pending until the retry budget is exhausted, at which point
// the call is permanently failed.
func (s *Service) RetryDevice(retryKey string) (DeviceRetryResult, error) {
	var result DeviceRetryResult
	err := s.store.Update(func(st *store.State) error {
		call, ok := st.DeviceCalls[retryKey]
		if !ok {
			return NewError(CodeNotFound, "device call not found")
		}
		attempts := st.RetryAttempts[retryKey]
		if len(attempts) > 0 {
			last := attempts[len(attempts)-1]
			if last.Status == coverage.RetrySuccess {
				result = DeviceRetryResult{Status: "success", Attempts: len(attempts), Result: last.Result}
				return nil
			}
			if last.Status == coverage.RetryPermanentFail {
				result = DeviceRetryResult{Status: "permanent_fail", Attempts: len(attempts)}
				return nil
			}
		}
		cfg := st.Configs[call.CycleID]
		policy := coverage.RetryPolicy{MaxAttempts: cfg.RetryMaxAttempts, BaseDelay: cfg.RetryBaseDelay, StepDelay: cfg.RetryStepDelay}
		key := coverage.RetryKey{
			CycleID:          call.CycleID,
			DeviceType:       call.DeviceType,
			ZoneID:           call.ZoneID,
			ObservationPoint: call.ObservationPoint,
			CallSeq:          call.CallSeq,
		}
		status, res := s.driver.Call(key)
		attempt := len(attempts) + 1
		logicalTime := policy.BackoffTime(attempt)
		if status == coverage.DeviceSuccess {
			st.RetryAttempts[retryKey] = append(attempts, coverage.RetryAttempt{Attempt: attempt, Status: coverage.RetrySuccess, LogicalTime: logicalTime, Result: res})
			call.Status = coverage.DeviceSuccess
			call.Result = res
			st.DeviceCalls[retryKey] = call
			result = DeviceRetryResult{Status: "success", Attempts: attempt, Result: res}
			return nil
		}
		if attempt >= policy.MaxAttempts {
			st.RetryAttempts[retryKey] = append(attempts, coverage.RetryAttempt{Attempt: attempt, Status: coverage.RetryPermanentFail, LogicalTime: logicalTime, Result: string(status)})
			call.Status = coverage.DevicePermanentFail
			st.DeviceCalls[retryKey] = call
			result = DeviceRetryResult{Status: "permanent_fail", Attempts: attempt}
			return nil
		}
		st.RetryAttempts[retryKey] = append(attempts, coverage.RetryAttempt{Attempt: attempt, Status: coverage.RetryPending, LogicalTime: logicalTime, Result: string(status)})
		result = DeviceRetryResult{Status: string(status), Attempts: attempt}
		return nil
	})
	if err != nil {
		return DeviceRetryResult{}, err
	}
	return result, nil
}

// RegisterDeviceCall records an initial failed device call and returns its
// deterministic retry key. It is used when an observation detects an adapter
// failure, so the failure is persisted before any retry is attempted.
func (s *Service) RegisterDeviceCall(id string, dt coverage.DeviceType, zone string, point int64, callSeq int64) (string, error) {
	retryKey := store.RetryKey(id, string(dt), zone, point, callSeq)
	err := s.store.Update(func(st *store.State) error {
		if _, ok := st.Cycles[id]; !ok {
			return NewError(CodeNotFound, "cycle not found")
		}
		st.DeviceCalls[retryKey] = coverage.DeviceCall{
			CycleID:          id,
			CallSeq:          callSeq,
			DeviceType:       dt,
			ZoneID:           zone,
			ObservationPoint: point,
			Status:           coverage.DeviceRejected,
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return retryKey, nil
}
