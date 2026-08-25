package httpapi

import (
	"strings"
	"testing"

	"tomato-bumblebee-pollination-rotation/internal/rules"
	"tomato-bumblebee-pollination-rotation/internal/store"
)

func TestLockCycleEstablishesSnapshotAndBudget(t *testing.T) {
	svc, _ := newTestService(t)
	req := validLockRequest("c-lock-1")
	view, err := svc.CreateAndLock(req)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	if view.State != "pending_admit" {
		t.Fatalf("state = %q, want pending_admit", view.State)
	}
	if view.CoverageCells != 4 {
		t.Fatalf("coverage cells = %d, want 4", view.CoverageCells)
	}
	if view.BudgetInitial != 10 || view.BudgetRemaining != 10 {
		t.Fatalf("budget = %d/%d, want 10/10", view.BudgetInitial, view.BudgetRemaining)
	}
	if view.EvidenceCount != 1 {
		t.Fatalf("evidence count = %d, want 1", view.EvidenceCount)
	}
	if view.RuleDigest == "" {
		t.Fatal("rule digest must be set")
	}
}

func TestLockCycleRejectsStaleCertificate(t *testing.T) {
	c := rules.NewMemoryCatalog()
	c.AddGreenhouse(rules.Greenhouse{ID: "g1", Version: 1, Zones: []string{"z1"}})
	c.AddZone(rules.Zone{ID: "z1", GreenhouseID: "g1"})
	c.AddColony(rules.Colony{ID: "c1", GreenhouseID: "g1"})
	c.AddCertificate(rules.HealthCertificate{ColonyID: "c1", Digest: "d", ValidFrom: 100, ValidUntil: 200})
	c.AddBatch(rules.InflorescenceBatch{ID: "b1", ZoneID: "z1", SampledInflorescences: 4})
	c.AddReviewer(rules.Reviewer{ID: "r1", Qualified: true})

	st := store.NewMemoryStore()
	svc := NewService(st, c, NewScriptDriver())
	req := LockRequest{
		CycleID: "c-stale", GreenhouseID: "g1", RuleVersion: 1,
		ColonyIDs: []string{"c1"}, BatchIDs: []string{"b1"}, ReviewerIDs: []string{"r1"},
		ObservationPoints: []int64{1}, Budget: 5,
		Windows:     []WindowSpec{{ColonyID: "c1", ZoneID: "z1", Start: 0, End: 100}},
		OperationID: "op", LogicalTime: 500,
	}
	_, err := svc.CreateAndLock(req)
	if err == nil {
		t.Fatal("expected stale certificate failure")
	}
	ae, ok := err.(*APIError)
	if !ok || ae.Code != CodeInvalidRequest {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestIdempotentLockReplay(t *testing.T) {
	svc, _ := newTestService(t)
	req := validLockRequest("c-idem")
	if _, err := svc.CreateAndLock(req); err != nil {
		t.Fatalf("first lock: %v", err)
	}
	view2, err := svc.CreateAndLock(req)
	if err != nil {
		t.Fatalf("replay lock: %v", err)
	}
	if view2.State != "pending_admit" {
		t.Fatalf("replay state = %q", view2.State)
	}
}

func TestIdempotentLockContentConflict(t *testing.T) {
	svc, _ := newTestService(t)
	req := validLockRequest("c-conflict")
	if _, err := svc.CreateAndLock(req); err != nil {
		t.Fatalf("first lock: %v", err)
	}
	req.Budget = 999
	_, err := svc.CreateAndLock(req)
	if err == nil {
		t.Fatal("expected content conflict")
	}
	if !strings.Contains(err.Error(), CodeOperationContentConflict) {
		t.Fatalf("expected %s, got %v", CodeOperationContentConflict, err)
	}
}
