package httpapi

import (
	"sync"
	"testing"
)

func TestConcurrentAdmitSingleWinner(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-admit")); err != nil {
		t.Fatal(err)
	}
	// Admit c1 into z2 at a non-overlapping window; concurrent identical admits
	// must yield exactly one winner (the loser hits a colony-window overlap).
	const n = 6
	start := make(chan struct{})
	var wg sync.WaitGroup
	wins := 0
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			req := AdmitRequest{ColonyID: "c1", ZoneID: "z2", WindowStart: 200, WindowEnd: 300, OperationID: "adm-" + itoa(int64(idx)), LogicalTime: 1000}
			if _, err := svc.Admit("c-admit", req); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if wins != 1 {
		t.Fatalf("expected exactly one winner, got %d", wins)
	}
}

func TestConcurrentObservationsConserveBudget(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-budget")); err != nil {
		t.Fatal(err)
	}
	cells := []struct {
		zone, batch string
		point       int64
	}{
		{"z1", "b1", 1}, {"z1", "b1", 2}, {"z2", "b2", 1}, {"z2", "b2", 2},
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, len(cells))
	for i, cell := range cells {
		wg.Add(1)
		go func(idx int, zone, batch string, point int64) {
			defer wg.Done()
			<-start
			req := validObservation(zone, batch, point)
			req.OperationID = "obs-budget-" + itoa(int64(idx))
			_, errs[idx] = svc.Observe("c-budget", req)
		}(i, cell.zone, cell.batch, cell.point)
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("observation %d: %v", i, err)
		}
	}
	view, _ := svc.GetCycle("c-budget")
	if view.BudgetDebited != 4 || view.BudgetRemaining != 6 {
		t.Fatalf("budget not conserved: debited=%d remaining=%d", view.BudgetDebited, view.BudgetRemaining)
	}
}
