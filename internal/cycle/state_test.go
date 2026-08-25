package cycle

import "testing"

func TestStateStringStable(t *testing.T) {
	cases := []struct {
		s    State
		want string
	}{
		{StatePendingLock, "pending_lock"},
		{StatePendingAdmit, "pending_admit"},
		{StateInFlight, "in_flight"},
		{StateSafetyFrozen, "safety_frozen"},
		{StatePendingRecoveryCheck, "pending_recovery_check"},
		{StateSupplementing, "supplementing"},
		{StatePendingCloseReview, "pending_close_review"},
		{StateCompleted, "completed"},
		{StateColonyWithdrawn, "colony_withdrawn"},
		{StateCancelled, "cancelled"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Fatalf("String(%d) = %q, want %q", tc.s, got, tc.want)
		}
	}
}

func TestTerminalStates(t *testing.T) {
	for _, s := range []State{StateCompleted, StateColonyWithdrawn, StateCancelled} {
		if !s.IsTerminal() {
			t.Fatalf("state %v should be terminal", s)
		}
	}
	if StateInFlight.IsTerminal() {
		t.Fatal("in_flight must not be terminal")
	}
}

func TestCanTransitionHappyPath(t *testing.T) {
	path := []State{
		StatePendingLock,
		StatePendingAdmit,
		StateInFlight,
		StateSafetyFrozen,
		StatePendingRecoveryCheck,
		StateInFlight,
		StatePendingCloseReview,
		StateCompleted,
	}
	for i := 0; i < len(path)-1; i++ {
		if !path[i].CanTransitionTo(path[i+1]) {
			t.Fatalf("transition %v -> %v should be allowed", path[i], path[i+1])
		}
	}
}

func TestCanTransitionRejectsTerminalToAnything(t *testing.T) {
	if StateCompleted.CanTransitionTo(StateInFlight) {
		t.Fatal("completed must not transition further")
	}
	if StateCancelled.CanTransitionTo(StatePendingAdmit) {
		t.Fatal("cancelled must not transition further")
	}
}

func TestAdvanceRejectsIllegalTransition(t *testing.T) {
	c := &PollinationCycle{ID: "c1", State: StatePendingAdmit}
	if c.Advance(StateCompleted) {
		t.Fatal("illegal transition must be rejected")
	}
	if c.State != StatePendingAdmit {
		t.Fatalf("state changed to %v", c.State)
	}
}

func TestAdvanceAppliesLegalTransition(t *testing.T) {
	c := &PollinationCycle{ID: "c1", State: StatePendingLock}
	if !c.Advance(StatePendingAdmit) {
		t.Fatal("legal transition must be applied")
	}
	if c.State != StatePendingAdmit {
		t.Fatalf("state = %v, want pending_admit", c.State)
	}
}
