package httpapi

import (
	"errors"
	"testing"

	"tomato-bumblebee-pollination-rotation/internal/store"
)

func TestModel_MultiGenerationRecoveryUsesCurrentCredential(t *testing.T) {
	cases := []struct {
		name       string
		lateRounds int
	}{
		{name: "late event after first recovery", lateRounds: 1},
		{name: "successive late generations", lateRounds: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cycleID := "model-recovery-" + itoa(int64(tc.lateRounds))
			svc, db := newTestService(t)
			if _, err := svc.CreateAndLock(validLockRequest(cycleID)); err != nil {
				t.Fatalf("lock cycle: %v", err)
			}

			freeze := func(operationID string, occurredAt, arrivedAt int64, wantGen int64) {
				t.Helper()
				got, err := svc.RegisterPesticideEvent(PesticideEventRequest{
					CycleID:       cycleID,
					AppliedZoneID: "z1",
					OccurredAt:    occurredAt,
					ArrivedAt:     arrivedAt,
					ReentryHours:  24,
					OperationID:   operationID,
				})
				if err != nil {
					t.Fatalf("register pesticide event %q: %v", operationID, err)
				}
				if got.RecoveryGen != wantGen {
					t.Fatalf("event %q generation = %d, want %d", operationID, got.RecoveryGen, wantGen)
				}
				view, err := svc.GetCycle(cycleID)
				if err != nil {
					t.Fatalf("get frozen cycle: %v", err)
				}
				if view.State != "safety_frozen" || view.RecoveryGen != wantGen {
					t.Fatalf("after event %q cycle = %+v, want safety_frozen generation %d", operationID, view, wantGen)
				}
			}

			recoverFlight := func(gen int64, operationPrefix string) {
				t.Helper()
				firstRequest := RecoveryCheckRequest{
					Withdrawn:       true,
					ResidueVerified: true,
					ReentryReached:  true,
					OperationID:     operationPrefix + "-first",
					LogicalTime:     200 + gen*10,
				}
				first, err := svc.RecoveryCheck(cycleID, firstRequest)
				if err != nil {
					t.Fatalf("generation %d first recovery check: %v", gen, err)
				}
				if !first.Ready || first.RecoveryGen != gen || first.State != "pending_recovery_check" {
					t.Fatalf("generation %d first recovery result = %+v", gen, first)
				}

				replay, err := svc.RecoveryCheck(cycleID, firstRequest)
				if err != nil {
					t.Fatalf("generation %d replay: %v", gen, err)
				}
				if replay != first {
					t.Fatalf("generation %d replay = %+v, want original %+v", gen, replay, first)
				}
				view, err := svc.GetCycle(cycleID)
				if err != nil {
					t.Fatalf("get cycle after replay: %v", err)
				}
				if view.State != "pending_recovery_check" {
					t.Fatalf("generation %d replay advanced state to %q", gen, view.State)
				}

				secondRequest := firstRequest
				secondRequest.OperationID = operationPrefix + "-second"
				secondRequest.LogicalTime++
				second, err := svc.RecoveryCheck(cycleID, secondRequest)
				if err != nil {
					t.Fatalf("generation %d second recovery check: %v", gen, err)
				}
				if !second.Ready || second.RecoveryGen != gen || second.State != "in_flight" {
					t.Fatalf("generation %d second recovery result = %+v, want ready in_flight", gen, second)
				}
			}

			freeze("initial-freeze", 10, 10, 0)
			recoverFlight(0, "initial-recovery")
			for round := 1; round <= tc.lateRounds; round++ {
				gen := int64(round)
				freeze("late-freeze-"+itoa(gen), 20+gen, 100+gen, gen)
				recoverFlight(gen, "late-recovery-"+itoa(gen))
			}

			if err := db.View(func(st *store.State) error {
				credentials := st.RecoveryCredentials[cycleID]
				if len(credentials) != tc.lateRounds+1 {
					t.Fatalf("credentials = %+v, want one for each generation 0..%d", credentials, tc.lateRounds)
				}
				for _, credential := range credentials {
					if credential.RecoveryGen < int64(tc.lateRounds) {
						if credential.InvalidatedByGen == 0 || credential.Ready() {
							t.Fatalf("historical credential was reused or lost invalidation: %+v", credential)
						}
						continue
					}
					if credential.RecoveryGen == int64(tc.lateRounds) && (!credential.Ready() || credential.InvalidatedByGen != 0) {
						t.Fatalf("current credential is not independently ready: %+v", credential)
					}
				}
				return nil
			}); err != nil {
				t.Fatalf("inspect recovery credentials: %v", err)
			}

			if _, err := svc.Cancel(cycleID, CancelRequest{OperationID: "terminal-cancel", LogicalTime: 500}); err != nil {
				t.Fatalf("cancel recovered cycle: %v", err)
			}
			_, err := svc.RecoveryCheck(cycleID, RecoveryCheckRequest{
				Withdrawn:       true,
				ResidueVerified: true,
				ReentryReached:  true,
				OperationID:     "recovery-after-terminal",
				LogicalTime:     501,
			})
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Code != CodeInvalidTransition {
				t.Fatalf("recovery after terminal state error = %v, want %s", err, CodeInvalidTransition)
			}
		})
	}
}
