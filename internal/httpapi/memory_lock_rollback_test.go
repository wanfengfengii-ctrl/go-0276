package httpapi

import (
	"errors"
	"testing"

	"tomato-bumblebee-pollination-rotation/internal/store"
)

func TestModel_MemoryLockRollbackLeavesNoPartialCycle(t *testing.T) {
	tests := []struct {
		name       string
		start, end int64
	}{
		{name: "second window ends before it starts", start: 100, end: 99},
		{name: "second window has zero duration", start: 100, end: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, db := newTestService(t)
			req := validLockRequest("rollback-" + tt.name)
			req.Windows[1].Start = tt.start
			req.Windows[1].End = tt.end

			if _, err := svc.CreateAndLock(req); err == nil {
				t.Fatal("CreateAndLock accepted an invalid second window")
			} else if apiErr, ok := err.(*APIError); !ok || apiErr.Code != CodeInvalidRequest {
				t.Fatalf("CreateAndLock error = %v, want %s", err, CodeInvalidRequest)
			}

			if _, err := svc.GetCycle(req.CycleID); err == nil {
				t.Fatal("failed lock left a queryable partial cycle")
			} else if apiErr, ok := err.(*APIError); !ok || apiErr.Code != CodeNotFound {
				t.Fatalf("GetCycle error = %v, want %s", err, CodeNotFound)
			}

			if err := db.View(func(st *store.State) error {
				counts := map[string]int{
					"Cycles": len(st.Cycles), "Configs": len(st.Configs),
					"Snapshots": len(st.Snapshots), "Occupancies": len(st.Occupancies),
					"Budgets": len(st.Budgets), "CoverageCells": len(st.CoverageCells),
					"Evidence": len(st.Evidence), "Operations": len(st.Operations),
				}
				for table, count := range counts {
					if count != 0 {
						return errors.New(table + " changed after rejected lock")
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			req.Windows[1].End = req.Windows[1].Start + 100
			if _, err := svc.CreateAndLock(req); err != nil {
				t.Fatalf("reusing cycle_id after rejected lock: %v", err)
			}
		})
	}
}
