package httpapi

import (
	"strings"
	"testing"
)

func TestModel_FreezeRecoveryCompletionBarrier(t *testing.T) {
	tests := []struct {
		name              string
		recoverFirst      bool
		registerLateEvent bool
		recoverLatest     bool
		wantComplete      bool
		wantRecoveryGen   int64
	}{
		{
			name:            "recovered barrier permits completion",
			recoverFirst:    true,
			wantComplete:    true,
			wantRecoveryGen: 0,
		},
		{
			name:            "unrecovered barrier blocks completion",
			wantRecoveryGen: 0,
		},
		{
			name:              "late event invalidates earlier recovery",
			recoverFirst:      true,
			registerLateEvent: true,
			wantRecoveryGen:   1,
		},
		{
			name:              "latest recovery lifts earlier and current barriers",
			recoverFirst:      true,
			registerLateEvent: true,
			recoverLatest:     true,
			wantComplete:      true,
			wantRecoveryGen:   1,
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTestService(t)
			cycleID := "model-freeze-close-" + itoa(int64(i))
			if _, err := svc.CreateAndLock(validLockRequest(cycleID)); err != nil {
				t.Fatalf("lock cycle: %v", err)
			}
			for _, zone := range []string{"z1", "z2"} {
				batch := "b1"
				if zone == "z2" {
					batch = "b2"
				}
				for _, point := range []int64{1, 2} {
					observation := validObservation(zone, batch, point)
					observation.OperationID = "observation-" + itoa(int64(i)) + "-" + zone + "-" + itoa(point)
					if _, err := svc.Observe(cycleID, observation); err != nil {
						t.Fatalf("observe %s/%d: %v", zone, point, err)
					}
				}
			}

			if _, err := svc.RegisterPesticideEvent(PesticideEventRequest{
				CycleID:       cycleID,
				AppliedZoneID: "z1",
				OccurredAt:    10,
				ArrivedAt:     10,
				ReentryHours:  24,
				OperationID:   "pesticide-first-" + itoa(int64(i)),
			}); err != nil {
				t.Fatalf("register first pesticide event: %v", err)
			}

			recover := func(label string, logicalTime int64) {
				t.Helper()
				for check := int64(1); check <= 2; check++ {
					result, err := svc.RecoveryCheck(cycleID, RecoveryCheckRequest{
						Withdrawn:       true,
						ResidueVerified: true,
						ReentryReached:  true,
						OperationID:     "recovery-" + label + "-" + itoa(int64(i)) + "-" + itoa(check),
						LogicalTime:     logicalTime + check,
					})
					if err != nil {
						t.Fatalf("recovery check %s/%d: %v", label, check, err)
					}
					if !result.Ready {
						t.Fatalf("recovery check %s/%d was not ready", label, check)
					}
				}
			}

			if tc.recoverFirst {
				recover("first", 100)
			}
			if tc.registerLateEvent {
				result, err := svc.RegisterPesticideEvent(PesticideEventRequest{
					CycleID:       cycleID,
					AppliedZoneID: "z2",
					OccurredAt:    20,
					ArrivedAt:     30,
					ReentryHours:  24,
					OperationID:   "pesticide-late-" + itoa(int64(i)),
				})
				if err != nil {
					t.Fatalf("register late pesticide event: %v", err)
				}
				if !result.Late {
					t.Fatal("second pesticide event was not classified as late")
				}
			}
			if tc.recoverLatest {
				recover("latest", 200)
			}

			view, err := svc.GetCycle(cycleID)
			if err != nil {
				t.Fatalf("get cycle: %v", err)
			}
			if view.RecoveryGen != tc.wantRecoveryGen {
				t.Fatalf("recovery generation = %d, want %d", view.RecoveryGen, tc.wantRecoveryGen)
			}

			for reviewer := int64(1); reviewer <= 2; reviewer++ {
				if _, err := svc.SubmitReview(cycleID, ReviewRequest{
					ReviewerID:  "r" + itoa(reviewer),
					OperationID: "post-freeze-review-" + itoa(int64(i)) + "-" + itoa(reviewer),
					LogicalTime: 300 + reviewer,
				}); err != nil {
					t.Fatalf("submit post-freeze review %d: %v", reviewer, err)
				}
			}

			result, err := svc.Complete(cycleID, CompleteRequest{
				OperationID: "complete-" + itoa(int64(i)),
				LogicalTime: 400,
			})
			if tc.wantComplete {
				if err != nil {
					t.Fatalf("complete after recovery: %v", err)
				}
				if result.State != "completed" || result.Kind != "completed" || result.CredentialID == "" {
					t.Fatalf("completion result = %+v, want completed terminal credential", result)
				}
				return
			}

			if err == nil {
				t.Fatalf("complete unexpectedly succeeded: %+v", result)
			}
			apiErr, ok := err.(*APIError)
			if !ok || apiErr.Code != CodeInvalidTransition || !strings.Contains(apiErr.Message, "active freeze barrier") {
				t.Fatalf("complete error = %v, want active freeze barrier", err)
			}
		})
	}
}
