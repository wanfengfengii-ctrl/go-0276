package occupancy

import "testing"

func TestIntervalValid(t *testing.T) {
	cases := []struct {
		name string
		in   Interval
		want bool
	}{
		{"positive", Interval{1, 2}, true},
		{"zero", Interval{2, 2}, false},
		{"inverted", Interval{3, 1}, false},
		{"negative span", Interval{0, -1}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Valid(); got != tc.want {
				t.Fatalf("Valid() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIntervalOverlapsHalfOpen(t *testing.T) {
	base := Interval{10, 20}
	cases := []struct {
		name string
		o    Interval
		want bool
	}{
		{"contained", Interval{12, 15}, true},
		{"left overlap", Interval{5, 15}, true},
		{"right overlap", Interval{15, 25}, true},
		{"adjacent left", Interval{0, 10}, false},
		{"adjacent right", Interval{20, 30}, false},
		{"disjoint", Interval{30, 40}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := base.Overlaps(tc.o); got != tc.want {
				t.Fatalf("Overlaps(%+v) = %v, want %v", tc.o, got, tc.want)
			}
		})
	}
}

func TestIntervalContainsEndpoint(t *testing.T) {
	in := Interval{5, 10}
	if !in.Contains(5) {
		t.Fatal("left endpoint must be inclusive")
	}
	if in.Contains(10) {
		t.Fatal("right endpoint must be exclusive")
	}
	if !in.Contains(7) {
		t.Fatal("interior value must be contained")
	}
}

func TestBudgetConservation(t *testing.T) {
	acct := FlightBudgetAccount{Initial: 100, Remaining: 100}
	if !acct.Debit(30, 1, 0) {
		t.Fatal("expected debit to succeed")
	}
	if !acct.Debit(70, 1, 1) {
		t.Fatal("expected second debit to succeed")
	}
	if acct.Debit(1, 1, 2) {
		t.Fatal("expected overdraw to fail")
	}
	if acct.Remaining != 0 || acct.Debited != 100 {
		t.Fatalf("unexpected balance: remaining=%d debited=%d", acct.Remaining, acct.Debited)
	}
	if !acct.Conserved() {
		t.Fatal("account must satisfy conservation invariant")
	}
}

func TestBudgetRejectsNegativeDebit(t *testing.T) {
	acct := FlightBudgetAccount{Initial: 10, Remaining: 10}
	if acct.Debit(-5, 1, 0) {
		t.Fatal("negative debit must be rejected")
	}
	if acct.Remaining != 10 {
		t.Fatalf("balance changed: %d", acct.Remaining)
	}
}
