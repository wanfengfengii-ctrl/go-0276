// Package coverage maintains the coverage grid and biological metrics ledger:
// coverage cells keyed by zone/inflorescence/observation-point, observations,
// fixed-point device readings and derived metrics with integer conservation.
package coverage

import (
	"errors"
	"math"
	"math/bits"
)

// ErrDivisionByZero is returned when a fixed-point divisor is zero.
var ErrDivisionByZero = errors.New("fixed point division by zero")

// ErrOverflow is returned when a fixed-point operation overflows int64.
var ErrOverflow = errors.New("fixed point overflow")

// Scale is the number of fractional decimal digits in fixed-point values.
const Scale = 1_000_000

// MulDiv computes (a * b) / c using a 128-bit intermediate product to avoid
// overflow, rounding half away from zero. It rejects a zero divisor and reports
// any result that cannot be represented in int64.
func MulDiv(a, b, c int64) (int64, error) {
	if c == 0 {
		return 0, ErrDivisionByZero
	}
	neg := (a < 0) != (b < 0) != (c < 0)
	ua := uint64(absI64(a))
	ub := uint64(absI64(b))
	uc := uint64(absI64(c))
	hi, lo := bits.Mul64(ua, ub)
	quo, rem := bits.Div64(hi, lo, uc)
	// Round half away from zero (round up in magnitude).
	if rem*2 >= uc {
		quo++
	}
	if quo > math.MaxInt64 {
		return 0, ErrOverflow
	}
	q := int64(quo)
	if neg {
		q = -q
	}
	return q, nil
}

// RoundRatio returns (num * Scale) / den with half-away-from-zero rounding,
// representing a ratio as a fixed-point value with Scale fractional digits.
func RoundRatio(num, den int64) (int64, error) {
	return MulDiv(num, Scale, den)
}

func absI64(x int64) int64 {
	if x < 0 {
		if x == math.MinInt64 {
			return math.MaxInt64
		}
		return -x
	}
	return x
}
