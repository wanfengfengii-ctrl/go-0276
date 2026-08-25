package coverage

// CoverageKey identifies a coverage cell by zone, inflorescence batch and
// observation point. Each locked key may only be filled once by a valid
// observation matching the current window and generation.
type CoverageKey struct {
	ZoneID               string
	InflorescenceBatchID string
	ObservationPoint     int64
}

// CoverageCell is the state of a single coverage grid slot.
type CoverageCell struct {
	Key         CoverageKey
	Filled      bool
	EvidenceSeq int64
	Generation  int64
}

// FixedPointReading is an integer device reading with an explicit dimension.
type FixedPointReading struct {
	Value int64
	Unit  string
	Scale int64
}

// DerivedMetric is a computed metric with rounding metadata.
type DerivedMetric struct {
	Name        string
	Value       int64
	Scale       int64
	Rounding    string
	Valid       bool
	FailureCode string
}

// Observation is a raw roaming collection for one observation point.
type Observation struct {
	ZoneID                     string
	InflorescenceBatchID       string
	ObservationPoint           int64
	VisitTotal                 int64
	PerInflorescence           []int64
	BiteMarkInflorescences     int64
	UnpollinatedInflorescences int64
	ReturningBees              int64
	OutgoingBees               int64
	DeadBees                   int64
	SampledInflorescences      int64
}
