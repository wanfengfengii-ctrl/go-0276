package coverage

// RetryStatus is the lifecycle status of a deterministic retry call.
type RetryStatus string

const (
	RetryPending       RetryStatus = "pending"
	RetrySuccess       RetryStatus = "success"
	RetryPermanentFail RetryStatus = "permanent_fail"
)

// RetryAttempt records one attempt to execute a device call.
type RetryAttempt struct {
	Attempt     int
	Status      RetryStatus
	LogicalTime int64
	Result      string
}

// RetryKey uniquely identifies a retryable device call by cycle, device type,
// zone, observation point and original call sequence.
type RetryKey struct {
	CycleID          string
	DeviceType       DeviceType
	ZoneID           string
	ObservationPoint int64
	CallSeq          int64
}

// RetryPolicy derives the maximum attempts and logical backoff from the locked
// rule snapshot. The backoff schedule is a deterministic linear backoff: the
// nth attempt occurs at base + n*step logical time.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   int64
	StepDelay   int64
}

// BackoffTime returns the logical time at which the given attempt (1-based)
// should run.
func (p RetryPolicy) BackoffTime(attempt int) int64 {
	if attempt < 1 {
		attempt = 1
	}
	return p.BaseDelay + int64(attempt-1)*p.StepDelay
}
