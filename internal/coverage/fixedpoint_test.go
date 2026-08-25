package coverage

import (
	"errors"
	"testing"
)

func TestMulDivBasic(t *testing.T) {
	got, err := MulDiv(3, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got != 6 {
		t.Fatalf("MulDiv(3,4,2) = %d, want 6", got)
	}
}

func TestMulDivRoundHalfAwayFromZero(t *testing.T) {
	cases := []struct {
		name    string
		a, b, c int64
		want    int64
	}{
		{"round up positive", 1, 1, 2, 1},
		{"round down positive", 1, 1, 3, 0},
		{"round away negative", -1, 1, 2, -1},
		{"exact", 6, 5, 3, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MulDiv(tc.a, tc.b, tc.c)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("MulDiv(%d,%d,%d) = %d, want %d", tc.a, tc.b, tc.c, got, tc.want)
			}
		})
	}
}

func TestMulDivDivisionByZero(t *testing.T) {
	_, err := MulDiv(1, 2, 0)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected ErrDivisionByZero, got %v", err)
	}
}

func TestRoundRatioScale(t *testing.T) {
	got, err := RoundRatio(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	// 1/3 * 1e6 rounded half away from zero.
	if got != 333333 {
		t.Fatalf("RoundRatio(1,3) = %d, want 333333", got)
	}
}

func TestRoundRatioNegative(t *testing.T) {
	got, err := RoundRatio(-1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got != -333333 {
		t.Fatalf("RoundRatio(-1,3) = %d, want -333333", got)
	}
}
