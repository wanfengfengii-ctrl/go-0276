package coverage

import "errors"

// ErrVisitConservation is returned when the visit total does not equal the sum
// of per-inflorescence visit counts.
var ErrVisitConservation = errors.New("visit total violates conservation")

// ErrInflorescenceBounds is returned when bite-mark or unpollinated counts
// violate the sampled-inflorescence bounds.
var ErrInflorescenceBounds = errors.New("inflorescence counts out of bounds")

// ErrNegativeCount is returned when any count is negative.
var ErrNegativeCount = errors.New("negative observation count")

// Validate checks the integer conservation rules of an observation. A legal
// observation must satisfy: visit total == sum of per-inflorescence visits;
// bite-mark and unpollinated counts are non-negative, each at most the sampled
// count, and their sum at most the sampled count; and all bee counts are
// non-negative. Illegal observations must never form coverage or debit budget.
func (o Observation) Validate() error {
	if o.VisitTotal < 0 || o.BiteMarkInflorescences < 0 ||
		o.UnpollinatedInflorescences < 0 || o.ReturningBees < 0 ||
		o.OutgoingBees < 0 || o.DeadBees < 0 || o.SampledInflorescences < 0 {
		return ErrNegativeCount
	}

	var sum int64
	for _, v := range o.PerInflorescence {
		if v < 0 {
			return ErrNegativeCount
		}
		sum += v
	}
	if sum != o.VisitTotal {
		return ErrVisitConservation
	}

	if o.BiteMarkInflorescences > o.SampledInflorescences ||
		o.UnpollinatedInflorescences > o.SampledInflorescences ||
		o.BiteMarkInflorescences+o.UnpollinatedInflorescences > o.SampledInflorescences {
		return ErrInflorescenceBounds
	}
	return nil
}
