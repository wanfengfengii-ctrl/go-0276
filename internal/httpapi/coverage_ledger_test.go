package httpapi

import (
	"strings"
	"testing"
)

func TestObservationRejectsMissingCell(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-miss")); err != nil {
		t.Fatal(err)
	}
	req := validObservation("z1", "b1", 99) // point 99 not in matrix
	req.OperationID = "obs-miss"
	_, err := svc.Observe("c-miss", req)
	if err == nil || !strings.Contains(err.Error(), CodeInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
	view, _ := svc.GetCycle("c-miss")
	if view.CoverageFilled != 0 || view.BudgetDebited != 0 {
		t.Fatalf("no coverage/budget should change: filled=%d debited=%d", view.CoverageFilled, view.BudgetDebited)
	}
}

func TestObservationRejectsDuplicateConflict(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-dup")); err != nil {
		t.Fatal(err)
	}
	req := validObservation("z1", "b1", 1)
	req.OperationID = "obs-dup-1"
	if _, err := svc.Observe("c-dup", req); err != nil {
		t.Fatalf("first observation: %v", err)
	}
	req2 := validObservation("z1", "b1", 1)
	req2.VisitTotal = 7
	req2.PerInflorescence = []int64{2, 2, 3}
	req2.OperationID = "obs-dup-2"
	_, err := svc.Observe("c-dup", req2)
	if err == nil || !strings.Contains(err.Error(), CodeObservationDuplicate) {
		t.Fatalf("expected duplicate conflict, got %v", err)
	}
}

func TestObservationRejectsConservationViolation(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-cons")); err != nil {
		t.Fatal(err)
	}
	req := validObservation("z1", "b1", 1)
	req.VisitTotal = 5 // does not equal per-inflorescence sum (6)
	req.OperationID = "obs-cons"
	_, err := svc.Observe("c-cons", req)
	if err == nil || !strings.Contains(err.Error(), CodeInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
	view, _ := svc.GetCycle("c-cons")
	if view.CoverageFilled != 0 || view.BudgetDebited != 0 {
		t.Fatalf("budget/coverage must not change on invalid observation")
	}
}

func TestObservationRejectsWrongGeneration(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-gen")); err != nil {
		t.Fatal(err)
	}
	req := validObservation("z1", "b1", 1)
	req.Generation = 99
	req.OperationID = "obs-gen"
	_, err := svc.Observe("c-gen", req)
	if err == nil || !strings.Contains(err.Error(), CodeInvalidRequest) {
		t.Fatalf("expected wrong-generation rejection, got %v", err)
	}
}

func TestValidObservationFillsCoverageAndDebitsBudget(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-valid")); err != nil {
		t.Fatal(err)
	}
	req := validObservation("z1", "b1", 1)
	req.OperationID = "obs-valid"
	res, err := svc.Observe("c-valid", req)
	if err != nil {
		t.Fatalf("observation: %v", err)
	}
	if res.Status != "valid" {
		t.Fatalf("status = %q, want valid", res.Status)
	}
	view, _ := svc.GetCycle("c-valid")
	if view.CoverageFilled != 1 || view.BudgetDebited != 1 || view.BudgetRemaining != 9 {
		t.Fatalf("coverage/budget mismatch: filled=%d debited=%d remaining=%d", view.CoverageFilled, view.BudgetDebited, view.BudgetRemaining)
	}
}
