package coverage

import (
	"errors"
	"testing"
)

func validObservation() Observation {
	return Observation{
		VisitTotal:                 6,
		PerInflorescence:           []int64{2, 2, 2},
		BiteMarkInflorescences:     1,
		UnpollinatedInflorescences: 1,
		ReturningBees:              5,
		OutgoingBees:               5,
		DeadBees:                   0,
		SampledInflorescences:      3,
	}
}

func TestObservationValid(t *testing.T) {
	if err := validObservation().Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestObservationVisitConservationViolation(t *testing.T) {
	o := validObservation()
	o.VisitTotal = 5
	if err := o.Validate(); !errors.Is(err, ErrVisitConservation) {
		t.Fatalf("expected ErrVisitConservation, got %v", err)
	}
}

func TestObservationNegativeCount(t *testing.T) {
	o := validObservation()
	o.DeadBees = -1
	if err := o.Validate(); !errors.Is(err, ErrNegativeCount) {
		t.Fatalf("expected ErrNegativeCount, got %v", err)
	}
}

func TestObservationBiteMarkExceedsSampled(t *testing.T) {
	o := validObservation()
	o.BiteMarkInflorescences = 4
	if err := o.Validate(); !errors.Is(err, ErrInflorescenceBounds) {
		t.Fatalf("expected ErrInflorescenceBounds, got %v", err)
	}
}

func TestObservationBiteAndUnpollinatedSumExceedsSampled(t *testing.T) {
	o := validObservation()
	o.BiteMarkInflorescences = 2
	o.UnpollinatedInflorescences = 2
	if err := o.Validate(); !errors.Is(err, ErrInflorescenceBounds) {
		t.Fatalf("expected ErrInflorescenceBounds, got %v", err)
	}
}

func TestObservationNegativePerInflorescence(t *testing.T) {
	o := validObservation()
	o.PerInflorescence = []int64{3, 3, -1}
	if err := o.Validate(); !errors.Is(err, ErrNegativeCount) {
		t.Fatalf("expected ErrNegativeCount, got %v", err)
	}
}
