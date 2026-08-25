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

// DeviceDriver abstracts the deterministic device adapters (environment probe,
// hive scale and counter adapter). Tests inject a scripted driver to force
// rejections, disconnects, timeouts and malformed responses.
type DeviceDriver interface {
	Call(key coverage.RetryKey) (coverage.DeviceResultStatus, string)
}

// ScriptDriver is a deterministic DeviceDriver backed by per-device failure
// scripts. A nil script always succeeds.
type ScriptDriver struct {
	scripts map[coverage.DeviceType]*coverage.FailureScript
}

// NewScriptDriver returns an empty driver (all devices succeed).
func NewScriptDriver() *ScriptDriver {
	return &ScriptDriver{scripts: map[coverage.DeviceType]*coverage.FailureScript{}}
}

// Set installs a failure script for a device type.
func (d *ScriptDriver) Set(dt coverage.DeviceType, s *coverage.FailureScript) {
	d.scripts[dt] = s
}

// Call implements DeviceDriver.
func (d *ScriptDriver) Call(key coverage.RetryKey) (coverage.DeviceResultStatus, string) {
	s := d.scripts[key.DeviceType]
	if s == nil {
		return coverage.DeviceSuccess, "ok"
	}
	status := s.Next()
	return status, string(status)
}

// Service composes the domain packages with the persistence store behind the
// HTTP API. Every write is executed inside a single store transaction so that
// conflicting or failing operations leave no partial state.
type Service struct {
	store   store.Store
	catalog rules.Catalog
	driver  DeviceDriver
}

// NewService constructs the application service.
func NewService(st store.Store, catalog rules.Catalog, driver DeviceDriver) *Service {
	if driver == nil {
		driver = NewScriptDriver()
	}
	return &Service{store: st, catalog: catalog, driver: driver}
}

// Store exposes the underlying store for handlers that need read-only views.
func (s *Service) Store() store.Store { return s.store }

// CreateAndLock validates the lock request against the catalog, builds an
// immutable snapshot, and in one transaction establishes the cycle, occupancies,
// budget account, coverage cells and the lock evidence event.
func (s *Service) CreateAndLock(req LockRequest) (CycleView, error) {
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
		snap, err := rules.BuildSnapshot(s.catalog, req.GreenhouseID, req.RuleVersion, req.ColonyIDs, req.BatchIDs, req.ReviewerIDs, req.LogicalTime)
		if err != nil {
			return NewError(CodeInvalidRequest, err.Error())
		}
		for _, rid := range req.ReviewerIDs {
			if !snap.ReviewerQualified(rid) {
				return NewError(CodeInvalidRequest, "unqualified reviewer: "+rid)
			}
		}
		if _, exists := st.Cycles[req.CycleID]; exists {
			return NewError(CodeInvalidRequest, "cycle already exists: "+req.CycleID)
		}
		c := cycle.PollinationCycle{
			ID:             req.CycleID,
			State:          cycle.StatePendingAdmit,
			TaskGeneration: 1,
			RecoveryGen:    0,
			LogicalTime:    req.LogicalTime,
			RuleDigest:     snap.Digest,
		}
		st.Cycles[req.CycleID] = c
		st.Snapshots[req.CycleID] = snap
		st.Configs[req.CycleID] = cycle.LockConfig{
			CycleID:           req.CycleID,
			RuleDigest:        snap.Digest,
			TaskGeneration:    1,
			RetryMaxAttempts:  req.RetrySpec.MaxAttempts,
			RetryBaseDelay:    req.RetrySpec.BaseDelay,
			RetryStepDelay:    req.RetrySpec.StepDelay,
			EnvTempLow:        req.EnvThresholds.TempLow,
			EnvTempHigh:       req.EnvThresholds.TempHigh,
			EnvHumidityLow:    req.EnvThresholds.HumLow,
			EnvHumidityHigh:   req.EnvThresholds.HumHigh,
			EnvCO2Low:         req.EnvThresholds.CO2Low,
			EnvCO2High:        req.EnvThresholds.CO2High,
			DriftLayers:       req.DriftLayers,
			ReentryHours:      req.ReentryHours,
			ObservationPoints: append([]int64(nil), req.ObservationPoints...),
		}

		// Establish occupancies.
		for _, w := range req.Windows {
			occ := occupancy.WindowOccupancy{
				ID:         fmt.Sprintf("%s:%s:%d", req.CycleID, w.ColonyID, w.Start),
				CycleID:    req.CycleID,
				ColonyID:   w.ColonyID,
				ZoneID:     w.ZoneID,
				Window:     occupancy.Interval{Start: w.Start, End: w.End},
				Purpose:    occupancy.PurposeAdmit,
				Generation: 1,
				Version:    1,
			}
			if !occ.Window.Valid() {
				return NewError(CodeInvalidRequest, "invalid window for colony "+w.ColonyID)
			}
			st.Occupancies[req.CycleID] = append(st.Occupancies[req.CycleID], occ)
		}

		// Budget account.
		st.Budgets[req.CycleID+":budget"] = occupancy.FlightBudgetAccount{
			ID:        req.CycleID + ":budget",
			CycleID:   req.CycleID,
			Initial:   req.Budget,
			Remaining: req.Budget,
		}

		// Coverage cells: zone x batch x observation point.
		for _, b := range snap.Batches {
			for _, p := range req.ObservationPoints {
				key := store.CellKey(req.CycleID, b.ZoneID, b.ID, p)
				st.CoverageCells[key] = coverage.CoverageCell{
					Key: coverage.CoverageKey{
						ZoneID:               b.ZoneID,
						InflorescenceBatchID: b.ID,
						ObservationPoint:     p,
					},
				}
			}
		}

		lockEvent := cycle.NewEvent(req.CycleID, cycle.EvidenceLocked, req.LogicalTime, 1, snap.Digest, "locked")
		lockEvent.Seq = 1
		st.Evidence[req.CycleID] = []cycle.EvidenceEvent{lockEvent}
		c.EvidenceRoot = cycle.EvidenceRoot(st.Evidence[req.CycleID])
		c.EvidenceSeq = 1
		st.Cycles[req.CycleID] = c

		view = s.buildCycleView(st, req.CycleID)
		resp, _ := json.Marshal(view)
		st.Operations[req.OperationID] = cycle.OperationRecord{
			OperationID: req.OperationID,
			Digest:      digest,
			Response:    string(resp),
			AppliedAt:   req.LogicalTime,
		}
		return nil
	})
	if err != nil {
		return CycleView{}, err
	}
	if replay != nil {
		_ = json.Unmarshal(replay, &view)
		return view, nil
	}
	return view, nil
}

// GetCycle returns the public view of a cycle.
func (s *Service) GetCycle(id string) (CycleView, error) {
	var view CycleView
	var found bool
	err := s.store.View(func(st *store.State) error {
		if _, ok := st.Cycles[id]; !ok {
			return nil
		}
		view = s.buildCycleView(st, id)
		found = true
		return nil
	})
	if err != nil {
		return CycleView{}, err
	}
	if !found {
		return CycleView{}, NewError(CodeNotFound, "cycle not found")
	}
	return view, nil
}

func (s *Service) buildCycleView(st *store.State, id string) CycleView {
	c := st.Cycles[id]
	cfg := st.Configs[id]
	view := CycleView{
		CycleID:         c.ID,
		State:           c.State.String(),
		TaskGeneration:  c.TaskGeneration,
		RecoveryGen:     c.RecoveryGen,
		RuleDigest:      c.RuleDigest,
		EvidenceRoot:    c.EvidenceRoot,
		EvidenceCount:   len(st.Evidence[id]),
		CoverageCells:   s.countCells(st, id),
		CoverageFilled:  s.countFilled(st, id),
		TerminalVersion: c.TerminalVer,
	}
	if a, ok := st.Budgets[id+":budget"]; ok {
		view.BudgetInitial = a.Initial
		view.BudgetRemaining = a.Remaining
		view.BudgetDebited = a.Debited
	}
	if t, ok := st.Terminals[id]; ok {
		view.TerminalKind = terminalKindName(t.Kind)
	}
	_ = cfg
	return view
}

func terminalKindName(k cycle.TerminalKind) string {
	switch k {
	case cycle.TerminalCompleted:
		return "completed"
	case cycle.TerminalWithdrawn:
		return "withdrawn"
	case cycle.TerminalCancelled:
		return "cancelled"
	default:
		return ""
	}
}

func (s *Service) countCells(st *store.State, id string) int {
	n := 0
	for k := range st.CoverageCells {
		if len(k) > len(id) && k[:len(id)] == id {
			n++
		}
	}
	return n
}

func (s *Service) countFilled(st *store.State, id string) int {
	n := 0
	for k, cell := range st.CoverageCells {
		if len(k) > len(id) && k[:len(id)] == id && cell.Filled {
			n++
		}
	}
	return n
}
