package coverage

import "testing"

func TestReturnRateFixedPoint(t *testing.T) {
	m, err := ReturnRate(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if m.Value != 333333 {
		t.Fatalf("return rate = %d, want 333333", m.Value)
	}
}

func TestReturnRateDivisionByZero(t *testing.T) {
	if _, err := ReturnRate(1, 0); err == nil {
		t.Fatal("expected division by zero")
	}
}

func TestReturnRateNegative(t *testing.T) {
	if _, err := ReturnRate(-1, 3); err == nil {
		t.Fatal("expected negative rejection")
	}
}

func TestBiteMarkRateBoundary(t *testing.T) {
	m, err := BiteMarkRate(2, 4)
	if err != nil {
		t.Fatal(err)
	}
	if m.Value != 500000 {
		t.Fatalf("bite mark rate = %d, want 500000", m.Value)
	}
}

func TestWeightDeltaAndContinuousLoss(t *testing.T) {
	d, err := WeightDelta(90, 100)
	if err != nil {
		t.Fatal(err)
	}
	if d.Value != -10 {
		t.Fatalf("weight delta = %d, want -10", d.Value)
	}
	loss, drop := ContinuousWeightLoss([]int64{100, 95, 90, 88})
	if !loss || drop != 12 {
		t.Fatalf("continuous loss = %v drop=%d, want true 12", loss, drop)
	}
	notLoss, _ := ContinuousWeightLoss([]int64{100, 95, 96})
	if notLoss {
		t.Fatal("non-decreasing sequence must not be a continuous loss")
	}
}

func TestEnvironmentCheckBounds(t *testing.T) {
	if code, ok := EnvironmentCheck(25, 0, 40); !ok || code != MetricOK {
		t.Fatalf("in-range value rejected: %s %v", code, ok)
	}
	if _, ok := EnvironmentCheck(50, 0, 40); ok {
		t.Fatal("out-of-range value accepted")
	}
	if _, ok := EnvironmentCheck(10, 40, 0); ok {
		t.Fatal("inverted threshold accepted")
	}
}
