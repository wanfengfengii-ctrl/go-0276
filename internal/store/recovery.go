package store

import (
	"sort"

	"tomato-bumblebee-pollination-rotation/internal/coverage"
	"tomato-bumblebee-pollination-rotation/internal/cycle"
)

// RecoveryReport summarizes what a restart must resume: unfinished cycles,
// pending deterministic device retries and undecided terminal arbitration.
type RecoveryReport struct {
	UnfinishedCycles []string
	PendingRetries   []coverage.RetryKey
	UndecidedCycles  []string
}

// PendingDeviceCall is a device call that still needs a retry after restart.
type PendingDeviceCall struct {
	CycleID          string
	DeviceType       string
	ZoneID           string
	ObservationPoint int64
	CallSeq          int64
	Attempts         int
}

// Recover scans persisted state and produces the set of work items the recovery
// worker must resume after a crash. It never mutates state.
func Recover(s *State) RecoveryReport {
	var report RecoveryReport
	for id, c := range s.Cycles {
		if c.State.IsTerminal() {
			continue
		}
		report.UnfinishedCycles = append(report.UnfinishedCycles, id)
		if _, ok := s.Terminals[id]; !ok && c.State == cycle.StatePendingCloseReview {
			report.UndecidedCycles = append(report.UndecidedCycles, id)
		}
	}
	for key, attempts := range s.RetryAttempts {
		if len(attempts) == 0 {
			continue
		}
		last := attempts[len(attempts)-1]
		if last.Status == coverage.RetryPending {
			call, ok := s.DeviceCalls[key]
			if !ok {
				continue
			}
			report.PendingRetries = append(report.PendingRetries, coverage.RetryKey{
				CycleID:          call.CycleID,
				DeviceType:       call.DeviceType,
				ZoneID:           call.ZoneID,
				ObservationPoint: call.ObservationPoint,
				CallSeq:          call.CallSeq,
			})
		}
	}
	sort.Strings(report.UnfinishedCycles)
	sort.Strings(report.UndecidedCycles)
	return report
}

// PendingCalls returns pending device calls across the state, deterministically
// ordered by cycle and original call sequence.
func PendingCalls(s *State) []PendingDeviceCall {
	var out []PendingDeviceCall
	for key, call := range s.DeviceCalls {
		attempts := s.RetryAttempts[key]
		if len(attempts) > 0 && attempts[len(attempts)-1].Status != coverage.RetryPending {
			continue
		}
		out = append(out, PendingDeviceCall{
			CycleID:          call.CycleID,
			DeviceType:       string(call.DeviceType),
			ZoneID:           call.ZoneID,
			ObservationPoint: call.ObservationPoint,
			CallSeq:          call.CallSeq,
			Attempts:         len(attempts),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CycleID != out[j].CycleID {
			return out[i].CycleID < out[j].CycleID
		}
		return out[i].CallSeq < out[j].CallSeq
	})
	return out
}
