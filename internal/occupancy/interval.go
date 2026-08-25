// Package occupancy manages time-window and colony occupancy: left-closed
// right-open intervals, the partition conflict graph, flight budget accounts
// and cross-zone rotation adjudication.
package occupancy

// Interval is a half-open time window [Start, End). Overlaps are judged using
// start <= other.end && other.start <= end semantics on the open right bound.
type Interval struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// Valid reports whether the interval is well-formed (End > Start).
func (i Interval) Valid() bool {
	return i.End > i.Start
}

// Overlaps reports whether two half-open intervals intersect. Adjacent windows
// (a.End == b.Start) do not overlap.
func (i Interval) Overlaps(o Interval) bool {
	return i.Start < o.End && o.Start < i.End
}

// Contains reports whether t lies within the half-open interval.
func (i Interval) Contains(t int64) bool {
	return t >= i.Start && t < i.End
}
