package httpapi

import (
	"encoding/json"
	"fmt"

	"tomato-bumblebee-pollination-rotation/internal/coverage"
	"tomato-bumblebee-pollination-rotation/internal/cycle"
	"tomato-bumblebee-pollination-rotation/internal/occupancy"
	"tomato-bumblebee-pollination-rotation/internal/rules"
	"tomato-bumblebee-pollination-rotation/internal/safety"
	"tomato-bumblebee-pollination-rotation/internal/store"
)

// RegisterPesticideEvent propagates a freeze barrier from an applied zone across
// the drift adjacency graph, freezes affected occupancies and, for late events,
// invalidates prior recovery credentials and bumps the recovery generation.
func (s *Service) RegisterPesticideEvent(req PesticideEventRequest) (PesticideEventResult, error) {
	digest, err := cycle.NormalizeContent(req)
	if err != nil {
		return PesticideEventResult{}, NewError(CodeInvalidRequest, "cannot normalize request")
	}
	var result PesticideEventResult
	var replay []byte
	err = s.store.Update(func(st *store.State) error {
		if rec, ok := st.Operations[req.OperationID]; ok {
			if rec.Digest != digest {
				return NewError(CodeOperationContentConflict, "operation content conflict")
			}
			replay = []byte(rec.Response)
			return nil
		}
		c, ok := st.Cycles[req.CycleID]
		if !ok {
			return NewError(CodeNotFound, "cycle not found")
		}
		if c.State.IsTerminal() {
			return NewError(CodeInvalidTransition, "cycle is terminal")
		}
		snap := st.Snapshots[req.CycleID]
		cfg := st.Configs[req.CycleID]

		event := safety.PesticideEvent{
			ID:            fmt.Sprintf("%s:pest:%d", req.CycleID, len(st.Pesticides[req.CycleID])+1),
			CycleID:       req.CycleID,
			AppliedZoneID: req.AppliedZoneID,
			OccurredAt:    req.OccurredAt,
			ArrivedAt:     req.ArrivedAt,
			ReentryHours:  req.ReentryHours,
		}
		deadline := event.Deadline()
		if req.ReentryHours == 0 && cfg.ReentryHours != 0 {
			deadline = req.OccurredAt + cfg.ReentryHours
		}
		event.ReentryDeadline = deadline

		adj := adjacencyEdges(snap.Adjacencies)
		affected := safety.PropagateFreeze(req.AppliedZoneID, adj, cfg.DriftLayers)

		recoveryGen := c.RecoveryGen
		if event.IsLate() {
			recoveryGen++
			for i := range st.RecoveryCredentials[req.CycleID] {
				st.RecoveryCredentials[req.CycleID][i].Invalidate(recoveryGen, "late pesticide event")
			}
		}
		c.RecoveryGen = recoveryGen
		c.State = cycle.StateSafetyFrozen

		barrier := safety.FreezeBarrier{
			ID:            fmt.Sprintf("%s:freeze:%d", req.CycleID, len(st.Barriers[req.CycleID])+1),
			CycleID:       req.CycleID,
			AffectedZones: affected,
			WindowStart:   req.OccurredAt,
			WindowEnd:     deadline,
			RecoveryGen:   recoveryGen,
		}
		st.Pesticides[req.CycleID] = append(st.Pesticides[req.CycleID], event)
		st.Barriers[req.CycleID] = append(st.Barriers[req.CycleID], barrier)

		ev := cycle.NewEvent(req.CycleID, cycle.EvidencePesticide, req.ArrivedAt, recoveryGen, digest, "pesticide event")
		ev.Seq = c.EvidenceSeq + 1
		c.EvidenceSeq = ev.Seq
		st.Evidence[req.CycleID] = append(st.Evidence[req.CycleID], ev)
		ev2 := cycle.NewEvent(req.CycleID, cycle.EvidenceFreeze, req.ArrivedAt, recoveryGen, digest, "freeze barrier")
		ev2.Seq = c.EvidenceSeq + 1
		c.EvidenceSeq = ev2.Seq
		st.Evidence[req.CycleID] = append(st.Evidence[req.CycleID], ev2)
		c.EvidenceRoot = cycle.EvidenceRoot(st.Evidence[req.CycleID])
		st.Cycles[req.CycleID] = c

		result = PesticideEventResult{
			EventID:       event.ID,
			AffectedZones: affected,
			RecoveryGen:   recoveryGen,
			Late:          event.IsLate(),
		}
		resp, _ := json.Marshal(result)
		st.Operations[req.OperationID] = cycle.OperationRecord{OperationID: req.OperationID, Digest: digest, Response: string(resp), AppliedAt: req.ArrivedAt}
		return nil
	})
	if err != nil {
		return PesticideEventResult{}, err
	}
	if replay != nil {
		_ = json.Unmarshal(replay, &result)
	}
	return result, nil
}

func adjacencyEdges(adj []rules.ZoneAdjacency) []safety.Adjacency {
	out := make([]safety.Adjacency, 0, len(adj))
	for _, a := range adj {
		out = append(out, safety.Adjacency{From: a.FromZoneID, To: a.ToZoneID})
	}
	return out
}

// RecoveryCheck records withdrawal, residue verification and reentry status and,
// once all gates are satisfied, advances the cycle toward restored flight.
func (s *Service) RecoveryCheck(id string, req RecoveryCheckRequest) (RecoveryCheckResult, error) {
	digest, err := cycle.NormalizeContent(req)
	if err != nil {
		return RecoveryCheckResult{}, NewError(CodeInvalidRequest, "cannot normalize request")
	}
	var result RecoveryCheckResult
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
		cred := s.findOrCreateCredential(st, id, c.RecoveryGen)
		cred.Withdrawn = req.Withdrawn
		cred.ResidueVerified = req.ResidueVerified
		cred.ReentryReached = req.ReentryReached
		s.upsertCredential(st, id, cred)

		ready := cred.Ready()
		switch c.State {
		case cycle.StateSafetyFrozen:
			if ready {
				c.State = cycle.StatePendingRecoveryCheck
			}
		case cycle.StatePendingRecoveryCheck:
			if ready {
				c.State = cycle.StateInFlight
			}
		}
		ev := cycle.NewEvent(id, cycle.EvidenceRecovery, req.LogicalTime, c.RecoveryGen, digest, "recovery check")
		ev.Seq = c.EvidenceSeq + 1
		c.EvidenceSeq = ev.Seq
		st.Evidence[id] = append(st.Evidence[id], ev)
		c.EvidenceRoot = cycle.EvidenceRoot(st.Evidence[id])
		st.Cycles[id] = c

		result = RecoveryCheckResult{Ready: ready, State: c.State.String(), RecoveryGen: c.RecoveryGen}
		resp, _ := json.Marshal(result)
		st.Operations[req.OperationID] = cycle.OperationRecord{OperationID: req.OperationID, Digest: digest, Response: string(resp), AppliedAt: req.LogicalTime}
		return nil
	})
	if err != nil {
		return RecoveryCheckResult{}, err
	}
	if replay != nil {
		_ = json.Unmarshal(replay, &result)
	}
	return result, nil
}

func (s *Service) findOrCreateCredential(st *store.State, id string, gen int64) safety.RecoveryCredential {
	for _, c := range st.RecoveryCredentials[id] {
		if c.RecoveryGen == gen {
			return c
		}
	}
	return safety.RecoveryCredential{ID: fmt.Sprintf("%s:cred:%d", id, gen), CycleID: id, RecoveryGen: gen}
}

func (s *Service) upsertCredential(st *store.State, id string, cred safety.RecoveryCredential) {
	for i, c := range st.RecoveryCredentials[id] {
		if c.ID == cred.ID {
			st.RecoveryCredentials[id][i] = cred
			return
		}
	}
	st.RecoveryCredentials[id] = append(st.RecoveryCredentials[id], cred)
}

// SubmitReview records an independent review signing the current evidence root.
func (s *Service) SubmitReview(id string, req ReviewRequest) (ReviewResult, error) {
	digest, err := cycle.NormalizeContent(req)
	if err != nil {
		return ReviewResult{}, NewError(CodeInvalidRequest, "cannot normalize request")
	}
	var result ReviewResult
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
		if !snap.ReviewerQualified(req.ReviewerID) {
			return NewError(CodeInvalidRequest, "reviewer not qualified: "+req.ReviewerID)
		}
		for _, r := range st.Reviews[id] {
			if r.ReviewerID == req.ReviewerID {
				return NewError(CodeInvalidRequest, "reviewer already signed")
			}
		}
		review := safety.Review{CycleID: id, ReviewerID: req.ReviewerID, EvidenceRoot: c.EvidenceRoot, SignedAt: req.LogicalTime}
		st.Reviews[id] = append(st.Reviews[id], review)
		ev := cycle.NewEvent(id, cycle.EvidenceReview, req.LogicalTime, c.RecoveryGen, digest, "review "+req.ReviewerID)
		ev.Seq = c.EvidenceSeq + 1
		c.EvidenceSeq = ev.Seq
		st.Evidence[id] = append(st.Evidence[id], ev)
		st.Cycles[id] = c

		reviews := safety.ReviewsFor(st.Reviews[id], id, c.EvidenceRoot)
		if len(reviews) >= 2 && safety.ReviewsIndependent(reviews[0], reviews[1]) {
			if c.State == cycle.StateInFlight || c.State == cycle.StateSupplementing {
				c.State = cycle.StatePendingCloseReview
				st.Cycles[id] = c
			}
		}
		result = ReviewResult{Accepted: true, State: c.State.String(), ReviewerCount: len(st.Reviews[id])}
		resp, _ := json.Marshal(result)
		st.Operations[req.OperationID] = cycle.OperationRecord{OperationID: req.OperationID, Digest: digest, Response: string(resp), AppliedAt: req.LogicalTime}
		return nil
	})
	if err != nil {
		return ReviewResult{}, err
	}
	if replay != nil {
		_ = json.Unmarshal(replay, &result)
	}
	return result, nil
}

// ComputeSupplement derives the deterministic supplement scope from uncovered
// cells, low return-rate cells and continuous weight-loss cells, then re-occupies
// the affected zones and observation points.
func (s *Service) ComputeSupplement(id string, req CompleteRequest) (SupplementResult, error) {
	digest, err := cycle.NormalizeContent(req)
	if err != nil {
		return SupplementResult{}, NewError(CodeInvalidRequest, "cannot normalize request")
	}
	var result SupplementResult
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
		if c.State == cycle.StateSupplementing {
			return NewError(CodeInvalidTransition, "supplement already active")
		}
		triggers := s.supplementTriggers(st, id)
		if len(triggers) == 0 {
			result = SupplementResult{Ranges: nil, State: c.State.String()}
			resp, _ := json.Marshal(result)
			st.Operations[req.OperationID] = cycle.OperationRecord{OperationID: req.OperationID, Digest: digest, Response: string(resp), AppliedAt: req.LogicalTime}
			return nil
		}
		scope := coverage.MergeSupplementTriggers(triggers)
		for _, r := range scope.Ranges {
			occ := occupancy.WindowOccupancy{
				ID:         fmt.Sprintf("%s:supp:%s:%d", id, r.ZoneID, r.PointStart),
				CycleID:    id,
				ColonyID:   "",
				ZoneID:     r.ZoneID,
				Window:     occupancy.Interval{Start: r.PointStart, End: r.PointEnd},
				Purpose:    occupancy.PurposeSupplement,
				Generation: c.TaskGeneration + c.RecoveryGen,
				Version:    1,
			}
			st.Occupancies[id] = append(st.Occupancies[id], occ)
		}
		c.State = cycle.StateSupplementing
		ev := cycle.NewEvent(id, cycle.EvidenceSupplement, req.LogicalTime, c.RecoveryGen, digest, "supplement")
		ev.Seq = c.EvidenceSeq + 1
		c.EvidenceSeq = ev.Seq
		st.Evidence[id] = append(st.Evidence[id], ev)
		c.EvidenceRoot = cycle.EvidenceRoot(st.Evidence[id])
		st.Cycles[id] = c

		result = SupplementResult{Ranges: scopeView(scope), State: c.State.String()}
		resp, _ := json.Marshal(result)
		st.Operations[req.OperationID] = cycle.OperationRecord{OperationID: req.OperationID, Digest: digest, Response: string(resp), AppliedAt: req.LogicalTime}
		return nil
	})
	if err != nil {
		return SupplementResult{}, err
	}
	if replay != nil {
		_ = json.Unmarshal(replay, &result)
	}
	return result, nil
}

func (s *Service) supplementTriggers(st *store.State, id string) []coverage.SupplementTrigger {
	var triggers []coverage.SupplementTrigger
	for key, cell := range st.CoverageCells {
		if !prefixMatch(key, id) {
			continue
		}
		if !cell.Filled {
			triggers = append(triggers, coverage.SupplementTrigger{
				ZoneID:               cell.Key.ZoneID,
				InflorescenceBatchID: cell.Key.InflorescenceBatchID,
				ObservationPoint:     cell.Key.ObservationPoint,
				Reason:               "uncovered",
			})
		}
	}
	return triggers
}

func prefixMatch(key, id string) bool {
	return len(key) > len(id) && key[:len(id)] == id
}

func scopeView(scope coverage.SupplementScope) []SupplementRangeView {
	out := make([]SupplementRangeView, 0, len(scope.Ranges))
	for _, r := range scope.Ranges {
		out = append(out, SupplementRangeView{
			ZoneID:     r.ZoneID,
			BatchID:    r.InflorescenceBatchID,
			PointStart: r.PointStart,
			PointEnd:   r.PointEnd,
		})
	}
	return out
}

// Complete attempts terminal competition for "completed" after verifying all
// closing preconditions.
func (s *Service) Complete(id string, req CompleteRequest) (TerminalResult, error) {
	return s.terminal(id, cycle.TerminalCompleted, cycle.StateCompleted, req.OperationID, req.LogicalTime)
}

// Cancel attempts terminal competition for "cancelled".
func (s *Service) Cancel(id string, req CancelRequest) (TerminalResult, error) {
	return s.terminal(id, cycle.TerminalCancelled, cycle.StateCancelled, req.OperationID, req.LogicalTime)
}

// Withdraw attempts terminal competition for "colony withdrawn".
func (s *Service) Withdraw(id string, req WithdrawRequest) (TerminalResult, error) {
	return s.terminal(id, cycle.TerminalWithdrawn, cycle.StateColonyWithdrawn, req.OperationID, req.LogicalTime)
}

func (s *Service) terminal(id string, kind cycle.TerminalKind, state cycle.State, opID string, logTime int64) (TerminalResult, error) {
	content := map[string]any{"cycle_id": id, "kind": terminalKindName(kind), "operation_id": opID}
	digest, err := cycle.NormalizeContent(content)
	if err != nil {
		return TerminalResult{}, NewError(CodeInvalidRequest, "cannot normalize request")
	}
	var result TerminalResult
	var replay []byte
	err = s.store.Update(func(st *store.State) error {
		if rec, ok := st.Operations[opID]; ok {
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
		var existingKind *string
		if t, decided := st.Terminals[id]; decided {
			k := terminalKindName(t.Kind)
			existingKind = &k
		}
		outcome := safety.ResolveArbitration(existingKind, terminalKindName(kind))
		if !outcome.Won {
			return NewError(CodeTerminalAlreadyDecided, "terminal already decided")
		}
		if c.State.IsTerminal() {
			return NewError(CodeTerminalAlreadyDecided, "terminal already decided")
		}
		if kind == cycle.TerminalCompleted {
			if err := s.checkClosePreconditions(st, id); err != nil {
				return err
			}
		}
		cred := cycle.TerminalCredential{
			CycleID:      id,
			Kind:         kind,
			Version:      c.TerminalVer + 1,
			EvidenceRoot: c.EvidenceRoot,
			CredentialID: fmt.Sprintf("%s:terminal:%d", id, c.TerminalVer+1),
		}
		if kind == cycle.TerminalCompleted {
			reviews := safety.ReviewsFor(st.Reviews[id], id, c.EvidenceRoot)
			if len(reviews) >= 2 {
				cred.ReviewerOne = reviews[0].ReviewerID
				cred.ReviewerTwo = reviews[1].ReviewerID
			}
		}
		st.Terminals[id] = cred
		c.State = state
		c.TerminalVer = cred.Version
		ev := cycle.NewEvent(id, cycle.EvidenceTerminal, logTime, c.RecoveryGen, digest, "terminal "+terminalKindName(kind))
		ev.Seq = c.EvidenceSeq + 1
		c.EvidenceSeq = ev.Seq
		st.Evidence[id] = append(st.Evidence[id], ev)
		c.EvidenceRoot = cycle.EvidenceRoot(st.Evidence[id])
		st.Cycles[id] = c

		result = TerminalResult{State: c.State.String(), Kind: terminalKindName(kind), CredentialID: cred.CredentialID}
		resp, _ := json.Marshal(result)
		st.Operations[opID] = cycle.OperationRecord{OperationID: opID, Digest: digest, Response: string(resp), AppliedAt: logTime}
		return nil
	})
	if err != nil {
		return TerminalResult{}, err
	}
	if replay != nil {
		_ = json.Unmarshal(replay, &result)
	}
	return result, nil
}

func (s *Service) checkClosePreconditions(st *store.State, id string) error {
	if s.countFilled(st, id) != s.countCells(st, id) {
		return NewError(CodeInvalidTransition, "coverage not closed")
	}
	acct := st.Budgets[id+":budget"]
	if !acct.Conserved() {
		return NewError(CodeInvalidTransition, "budget not conserved")
	}
	for _, b := range st.Barriers[id] {
		_ = b
		return NewError(CodeInvalidTransition, "active freeze barrier")
	}
	c := st.Cycles[id]
	if c.State == cycle.StateSupplementing {
		return NewError(CodeInvalidTransition, "active supplement task")
	}
	reviews := safety.ReviewsFor(st.Reviews[id], id, c.EvidenceRoot)
	if len(reviews) < 2 || !safety.ReviewsIndependent(reviews[0], reviews[1]) {
		return NewError(CodeInvalidTransition, "insufficient independent reviews")
	}
	return nil
}

// Rotate adjudicates a cross-zone rotation and, when safe, releases the old
// occupancy and establishes a new one.
func (s *Service) Rotate(id string, req RotateRequest) (CycleView, error) {
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
		if !snap.Compatible(req.FromZoneID, req.ToZoneID) {
			return NewError(CodeInvalidTransition, "zones incompatible")
		}
		var others []occupancy.WindowOccupancy
		for _, o := range st.Occupancies[id] {
			if o.ColonyID != req.ColonyID {
				others = append(others, o)
			}
		}
		if risk := occupancy.CarrierRisk(others, req.ToZoneID, snap.Compatible); len(risk) > 0 {
			return NewError(CodeInvalidTransition, "cross-zone carry risk")
		}
		for _, b := range st.Barriers[id] {
			if b.Affects(req.ToZoneID, req.WindowStart) {
				return NewError(CodeInvalidTransition, "target zone frozen")
			}
		}
		graph := occupancy.NewConflictGraph()
		for _, o := range st.Occupancies[id] {
			graph.Add(o)
		}
		candidate := occupancy.WindowOccupancy{
			ColonyID: req.ColonyID,
			ZoneID:   req.ToZoneID,
			Window:   occupancy.Interval{Start: req.WindowStart, End: req.WindowEnd},
		}
		if overlaps := graph.FindColonyOverlaps(candidate); len(overlaps) > 0 {
			return NewError(CodeInvalidTransition, "colony window overlap on rotation")
		}
		acct := st.Budgets[id+":budget"]
		if acct.Remaining < 1 {
			return NewError(CodeInvalidTransition, "insufficient budget for rotation")
		}

		// Release old occupancies for the colony, then establish the new one.
		var kept []occupancy.WindowOccupancy
		for _, o := range st.Occupancies[id] {
			if o.ColonyID == req.ColonyID {
				continue
			}
			kept = append(kept, o)
		}
		newOcc := occupancy.WindowOccupancy{
			ID:         fmt.Sprintf("%s:%s:%d", id, req.ColonyID, req.WindowStart),
			CycleID:    id,
			ColonyID:   req.ColonyID,
			ZoneID:     req.ToZoneID,
			Window:     occupancy.Interval{Start: req.WindowStart, End: req.WindowEnd},
			Purpose:    occupancy.PurposeRotate,
			Generation: c.TaskGeneration + c.RecoveryGen,
			Version:    1,
		}
		kept = append(kept, newOcc)
		st.Occupancies[id] = kept

		ev := cycle.NewEvent(id, cycle.EvidenceAdmitted, req.LogicalTime, c.TaskGeneration+c.RecoveryGen, digest, "rotate "+req.ColonyID)
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

// Evidence returns the ordered evidence stream for a cycle.
func (s *Service) Evidence(id string) ([]EvidenceView, error) {
	var out []EvidenceView
	var found bool
	err := s.store.View(func(st *store.State) error {
		if _, ok := st.Cycles[id]; !ok {
			return nil
		}
		found = true
		events := append([]cycle.EvidenceEvent(nil), st.Evidence[id]...)
		cycle.SortEvents(events)
		for _, ev := range events {
			out = append(out, EvidenceView{
				Seq:         ev.Seq,
				Kind:        string(ev.Kind),
				LogicalTime: ev.LogicalTime,
				Generation:  ev.Generation,
				Digest:      ev.Digest,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, NewError(CodeNotFound, "cycle not found")
	}
	return out, nil
}
