package coverage

import "errors"

// Stable error codes for derived metric failures.
const (
	MetricDivisionByZero = "METRIC_DIVISION_BY_ZERO"
	MetricOverflow       = "METRIC_OVERFLOW"
	MetricNegative       = "METRIC_NEGATIVE_VALUE"
	MetricOK             = "METRIC_OK"
)

// ReturnRate returns the fixed-point ratio returning/outgoing bees. It fails
// with a stable code when outgoing is zero or any operand is negative.
func ReturnRate(returning, outgoing int64) (DerivedMetric, error) {
	if returning < 0 || outgoing < 0 {
		return DerivedMetric{}, errors.New(MetricNegative)
	}
	if outgoing == 0 {
		return DerivedMetric{}, errors.New(MetricDivisionByZero)
	}
	v, err := RoundRatio(returning, outgoing)
	if err != nil {
		return DerivedMetric{}, errors.New(MetricOverflow)
	}
	return DerivedMetric{Name: "return_rate", Value: v, Scale: Scale, Rounding: "half_away", Valid: true, FailureCode: MetricOK}, nil
}

// BiteMarkRate returns the fixed-point ratio bite-mark/sampled inflorescences.
func BiteMarkRate(bite, sampled int64) (DerivedMetric, error) {
	if bite < 0 || sampled < 0 {
		return DerivedMetric{}, errors.New(MetricNegative)
	}
	if sampled == 0 {
		return DerivedMetric{}, errors.New(MetricDivisionByZero)
	}
	v, err := RoundRatio(bite, sampled)
	if err != nil {
		return DerivedMetric{}, errors.New(MetricOverflow)
	}
	return DerivedMetric{Name: "bite_mark_rate", Value: v, Scale: Scale, Rounding: "half_away", Valid: true, FailureCode: MetricOK}, nil
}

// UnpollinatedRate returns the fixed-point ratio unpollinated/sampled.
func UnpollinatedRate(unpollinated, sampled int64) (DerivedMetric, error) {
	if unpollinated < 0 || sampled < 0 {
		return DerivedMetric{}, errors.New(MetricNegative)
	}
	if sampled == 0 {
		return DerivedMetric{}, errors.New(MetricDivisionByZero)
	}
	v, err := RoundRatio(unpollinated, sampled)
	if err != nil {
		return DerivedMetric{}, errors.New(MetricOverflow)
	}
	return DerivedMetric{Name: "unpollinated_rate", Value: v, Scale: Scale, Rounding: "half_away", Valid: true, FailureCode: MetricOK}, nil
}

// WeightDelta returns the fixed-point difference between two scaled weights. It
// reports overflow if the subtraction cannot be represented.
func WeightDelta(current, baseline int64) (DerivedMetric, error) {
	if current < 0 || baseline < 0 {
		return DerivedMetric{}, errors.New(MetricNegative)
	}
	if (baseline > 0 && current > 0 && current > baseline && current-baseline < 0) ||
		(current < baseline && baseline-current < 0) {
		return DerivedMetric{}, errors.New(MetricOverflow)
	}
	d := current - baseline
	return DerivedMetric{Name: "weight_delta", Value: d, Scale: Scale, Rounding: "exact", Valid: true, FailureCode: MetricOK}, nil
}

// ContinuousWeightLoss detects whether a sequence of weights is strictly
// decreasing and by how much in total.
func ContinuousWeightLoss(weights []int64) (bool, int64) {
	if len(weights) < 2 {
		return false, 0
	}
	var drop int64
	for i := 1; i < len(weights); i++ {
		if weights[i] >= weights[i-1] {
			return false, 0
		}
		drop += weights[i-1] - weights[i]
	}
	return true, drop
}

// EnvironmentCheck evaluates a single environment reading against a locked
// threshold and returns a code describing whether it is within bounds.
func EnvironmentCheck(value, low, high int64) (string, bool) {
	if low > high {
		return MetricOverflow, false
	}
	if value < low || value > high {
		return "ENVIRONMENT_OUT_OF_RANGE", false
	}
	return MetricOK, true
}
