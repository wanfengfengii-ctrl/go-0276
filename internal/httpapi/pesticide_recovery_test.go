package httpapi

import (
	"testing"
)

func TestPesticideFreezePropagatesAdjacentZones(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-pest")); err != nil {
		t.Fatal(err)
	}
	res, err := svc.RegisterPesticideEvent(PesticideEventRequest{
		CycleID:       "c-pest",
		AppliedZoneID: "z1",
		OccurredAt:    50,
		ArrivedAt:     50,
		ReentryHours:  24,
		OperationID:   "pest-1",
	})
	if err != nil {
		t.Fatalf("pesticide event: %v", err)
	}
	if len(res.AffectedZones) != 2 {
		t.Fatalf("affected zones = %v, want [z1 z2]", res.AffectedZones)
	}
	view, _ := svc.GetCycle("c-pest")
	if view.State != "safety_frozen" {
		t.Fatalf("state = %q, want safety_frozen", view.State)
	}
}

func TestFrozenObservationIsQuarantined(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-frozen")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterPesticideEvent(PesticideEventRequest{
		CycleID: "c-frozen", AppliedZoneID: "z1", OccurredAt: 10, ArrivedAt: 10, ReentryHours: 24, OperationID: "pest-f",
	}); err != nil {
		t.Fatal(err)
	}
	req := validObservation("z1", "b1", 1)
	req.LogicalTime = 20
	req.OperationID = "obs-frozen"
	_, err := svc.Observe("c-frozen", req)
	if err == nil {
		t.Fatal("expected frozen observation to be rejected")
	}
	view, _ := svc.GetCycle("c-frozen")
	if view.CoverageFilled != 0 {
		t.Fatalf("frozen observation must not fill coverage")
	}
}

func TestLatePesticideEventBumpsRecoveryGeneration(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-late")); err != nil {
		t.Fatal(err)
	}
	res, err := svc.RegisterPesticideEvent(PesticideEventRequest{
		CycleID: "c-late", AppliedZoneID: "z2", OccurredAt: 30, ArrivedAt: 90, ReentryHours: 24, OperationID: "pest-late",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Late {
		t.Fatal("expected late event")
	}
	if res.RecoveryGen != 1 {
		t.Fatalf("recovery gen = %d, want 1", res.RecoveryGen)
	}
}

func TestRecoveryCheckRestoresFlight(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-rec")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterPesticideEvent(PesticideEventRequest{
		CycleID: "c-rec", AppliedZoneID: "z1", OccurredAt: 10, ArrivedAt: 10, ReentryHours: 24, OperationID: "pest-r",
	}); err != nil {
		t.Fatal(err)
	}
	// First check satisfies all gates -> pending_recovery_check.
	r1, err := svc.RecoveryCheck("c-rec", RecoveryCheckRequest{Withdrawn: true, ResidueVerified: true, ReentryReached: true, OperationID: "rec-1", LogicalTime: 100})
	if err != nil {
		t.Fatalf("recovery check 1: %v", err)
	}
	if r1.State != "pending_recovery_check" {
		t.Fatalf("state after check 1 = %q, want pending_recovery_check", r1.State)
	}
	// Second check -> restored flight.
	r2, err := svc.RecoveryCheck("c-rec", RecoveryCheckRequest{Withdrawn: true, ResidueVerified: true, ReentryReached: true, OperationID: "rec-2", LogicalTime: 101})
	if err != nil {
		t.Fatalf("recovery check 2: %v", err)
	}
	if r2.State != "in_flight" {
		t.Fatalf("state after check 2 = %q, want in_flight", r2.State)
	}
}
