package httpapi

import (
	"errors"
	"path/filepath"
	"testing"

	"tomato-bumblebee-pollination-rotation/internal/store"
)

func TestModel_FrozenObservationPersistsQuarantineEvidenceAcrossRestart(t *testing.T) {
	tests := []struct {
		name    string
		zoneID  string
		batchID string
	}{
		{name: "pesticide application zone", zoneID: "z1", batchID: "b1"},
		{name: "drift affected zone", zoneID: "z2", batchID: "b2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cycleID := "cycle-frozen-persistence"
			path := filepath.Join(t.TempDir(), "pollination.db")
			db, err := store.OpenSQLStore(path)
			if err != nil {
				t.Fatalf("open store: %v", err)
			}
			svc := NewService(db, defaultCatalog(), NewScriptDriver())

			if _, err := svc.CreateAndLock(validLockRequest(cycleID)); err != nil {
				t.Fatalf("lock cycle: %v", err)
			}
			if _, err := svc.RegisterPesticideEvent(PesticideEventRequest{
				CycleID:       cycleID,
				AppliedZoneID: "z1",
				OccurredAt:    10,
				ArrivedAt:     10,
				ReentryHours:  24,
				OperationID:   "pesticide-freeze",
			}); err != nil {
				t.Fatalf("register pesticide event: %v", err)
			}
			before, err := svc.GetCycle(cycleID)
			if err != nil {
				t.Fatalf("get cycle before observation: %v", err)
			}

			req := validObservation(tt.zoneID, tt.batchID, 1)
			req.OperationID = "frozen-observation"
			req.LogicalTime = 20
			result, err := svc.Observe(cycleID, req)
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Code != CodeInvalidTransition {
				t.Fatalf("Observe error = %v, want %s", err, CodeInvalidTransition)
			}
			if result.Status != "quarantine" {
				t.Fatalf("Observe status = %q, want quarantine", result.Status)
			}

			assertPersisted := func(stage string, gotRoot string) string {
				t.Helper()
				view, err := svc.GetCycle(cycleID)
				if err != nil {
					t.Fatalf("%s: get cycle: %v", stage, err)
				}
				if view.CoverageFilled != 0 {
					t.Fatalf("%s: coverage filled = %d, want 0", stage, view.CoverageFilled)
				}
				if view.BudgetRemaining != view.BudgetInitial || view.BudgetDebited != 0 {
					t.Fatalf("%s: budget = initial %d, remaining %d, debited %d; want unchanged", stage, view.BudgetInitial, view.BudgetRemaining, view.BudgetDebited)
				}
				if view.EvidenceCount != 4 {
					t.Fatalf("%s: evidence count = %d, want 4", stage, view.EvidenceCount)
				}
				if view.EvidenceRoot == before.EvidenceRoot {
					t.Fatalf("%s: evidence root did not advance after quarantined observation", stage)
				}
				if gotRoot != "" && view.EvidenceRoot != gotRoot {
					t.Fatalf("%s: evidence root = %q, want persisted root %q", stage, view.EvidenceRoot, gotRoot)
				}

				evidence, err := svc.Evidence(cycleID)
				if err != nil {
					t.Fatalf("%s: get evidence: %v", stage, err)
				}
				wantKinds := []string{"locked", "pesticide_event", "freeze_barrier", "quarantine"}
				if len(evidence) != len(wantKinds) {
					t.Fatalf("%s: evidence length = %d, want %d", stage, len(evidence), len(wantKinds))
				}
				for i, wantKind := range wantKinds {
					if evidence[i].Seq != int64(i+1) || evidence[i].Kind != wantKind {
						t.Fatalf("%s: evidence[%d] = {seq:%d kind:%q}, want {seq:%d kind:%q}", stage, i, evidence[i].Seq, evidence[i].Kind, i+1, wantKind)
					}
				}
				return view.EvidenceRoot
			}

			root := assertPersisted("before restart", "")
			if err := db.Close(); err != nil {
				t.Fatalf("close store: %v", err)
			}
			db, err = store.OpenSQLStore(path)
			if err != nil {
				t.Fatalf("reopen store: %v", err)
			}
			defer db.Close()
			svc = NewService(db, defaultCatalog(), NewScriptDriver())
			assertPersisted("after restart", root)
		})
	}
}
