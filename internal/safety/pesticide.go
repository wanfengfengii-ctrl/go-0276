// Package safety implements the pesticide freeze/recovery and close arbitration
// component: pesticide events, freeze barriers, recovery credentials, reviews
// and the terminal arbitration that feeds close competition.

package safety

// PesticideEvent records a pesticide application with both its occurrence and
// arrival logical times. Late events keep the original occurrence time.
type PesticideEvent struct {
	ID              string
	CycleID         string
	AppliedZoneID   string
	OccurredAt      int64
	ArrivedAt       int64
	ReentryHours    int64
	ReentryDeadline int64
	DriftLayers     int
}

// IsLate reports whether the event arrived after its occurrence time.
func (e PesticideEvent) IsLate() bool { return e.ArrivedAt > e.OccurredAt }

// Deadline computes the reentry deadline from the occurrence time and reentry
// hours, or returns the explicitly set deadline when non-zero.
func (e PesticideEvent) Deadline() int64 {
	if e.ReentryDeadline != 0 {
		return e.ReentryDeadline
	}
	return e.OccurredAt + e.ReentryHours
}
