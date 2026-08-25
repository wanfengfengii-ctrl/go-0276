package httpapi

import "testing"

func TestSupplementMergesUncoveredCellsStably(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-supp")); err != nil {
		t.Fatal(err)
	}
	// Fill only z1 point 1; the rest remain uncovered.
	req := validObservation("z1", "b1", 1)
	req.OperationID = "obs-supp-1"
	if _, err := svc.Observe("c-supp", req); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ComputeSupplement("c-supp", CompleteRequest{OperationID: "supp-1", LogicalTime: 1500})
	if err != nil {
		t.Fatalf("supplement: %v", err)
	}
	if len(res.Ranges) != 2 {
		t.Fatalf("ranges = %+v, want 2", res.Ranges)
	}
	if res.Ranges[0].ZoneID != "z1" || res.Ranges[0].PointStart != 2 || res.Ranges[0].PointEnd != 3 {
		t.Fatalf("z1 range = %+v", res.Ranges[0])
	}
	if res.Ranges[1].ZoneID != "z2" || res.Ranges[1].PointStart != 1 || res.Ranges[1].PointEnd != 3 {
		t.Fatalf("z2 range = %+v", res.Ranges[1])
	}
	if res.State != "supplementing" {
		t.Fatalf("state = %q, want supplementing", res.State)
	}
}

func TestSupplementRejectsSecondActiveTask(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-supp2")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ComputeSupplement("c-supp2", CompleteRequest{OperationID: "supp-a", LogicalTime: 1000}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.ComputeSupplement("c-supp2", CompleteRequest{OperationID: "supp-b", LogicalTime: 1001})
	if err == nil {
		t.Fatal("expected second supplement to be rejected")
	}
}
