package httpapi

import "time"

// WindowSpec is one colony-to-zone admission window in a lock request.
type WindowSpec struct {
	ColonyID string `json:"colony_id"`
	ZoneID   string `json:"zone_id"`
	Start    int64  `json:"start"`
	End      int64  `json:"end"`
}

// EnvThresholds are the locked environment bounds for a cycle.
type EnvThresholds struct {
	TempLow  int64 `json:"temp_low"`
	TempHigh int64 `json:"temp_high"`
	HumLow   int64 `json:"hum_low"`
	HumHigh  int64 `json:"hum_high"`
	CO2Low   int64 `json:"co2_low"`
	CO2High  int64 `json:"co2_high"`
}

// RetrySpec is the locked retry policy.
type RetrySpec struct {
	MaxAttempts int   `json:"max_attempts"`
	BaseDelay   int64 `json:"base_delay"`
	StepDelay   int64 `json:"step_delay"`
}

// LockRequest is the body of POST /v1/cycles and /lock.
type LockRequest struct {
	CycleID           string        `json:"cycle_id"`
	GreenhouseID      string        `json:"greenhouse_id"`
	RuleVersion       int64         `json:"rule_version"`
	ColonyIDs         []string      `json:"colony_ids"`
	BatchIDs          []string      `json:"batch_ids"`
	ReviewerIDs       []string      `json:"reviewer_ids"`
	ObservationPoints []int64       `json:"observation_points"`
	Budget            int64         `json:"budget"`
	Windows           []WindowSpec  `json:"windows"`
	EnvThresholds     EnvThresholds `json:"env_thresholds"`
	RetrySpec         RetrySpec     `json:"retry_spec"`
	DriftLayers       int           `json:"drift_layers"`
	ReentryHours      int64         `json:"reentry_hours"`
	OperationID       string        `json:"operation_id"`
	LogicalTime       int64         `json:"logical_time"`
}

// AdmitRequest is the body of POST /v1/cycles/{id}/admit.
type AdmitRequest struct {
	ColonyID    string `json:"colony_id"`
	ZoneID      string `json:"zone_id"`
	WindowStart int64  `json:"window_start"`
	WindowEnd   int64  `json:"window_end"`
	OperationID string `json:"operation_id"`
	LogicalTime int64  `json:"logical_time"`
}

// RotateRequest is the body of POST /v1/cycles/{id}/rotate.
type RotateRequest struct {
	ColonyID    string `json:"colony_id"`
	FromZoneID  string `json:"from_zone_id"`
	ToZoneID    string `json:"to_zone_id"`
	WindowStart int64  `json:"window_start"`
	WindowEnd   int64  `json:"window_end"`
	OperationID string `json:"operation_id"`
	LogicalTime int64  `json:"logical_time"`
}

// WithdrawRequest is the body of POST /v1/cycles/{id}/withdraw.
type WithdrawRequest struct {
	ColonyID    string `json:"colony_id"`
	OperationID string `json:"operation_id"`
	LogicalTime int64  `json:"logical_time"`
}

// CancelRequest is the body of POST /v1/cycles/{id}/cancel.
type CancelRequest struct {
	OperationID string `json:"operation_id"`
	LogicalTime int64  `json:"logical_time"`
}

// ObservationRequest is the body of POST /v1/cycles/{id}/observations.
type ObservationRequest struct {
	ZoneID                     string  `json:"zone_id"`
	InflorescenceBatchID       string  `json:"inflorescence_batch_id"`
	ObservationPoint           int64   `json:"observation_point"`
	VisitTotal                 int64   `json:"visit_total"`
	PerInflorescence           []int64 `json:"per_inflorescence"`
	BiteMarkInflorescences     int64   `json:"bite_mark_inflorescences"`
	UnpollinatedInflorescences int64   `json:"unpollinated_inflorescences"`
	ReturningBees              int64   `json:"returning_bees"`
	OutgoingBees               int64   `json:"outgoing_bees"`
	DeadBees                   int64   `json:"dead_bees"`
	SampledInflorescences      int64   `json:"sampled_inflorescences"`
	Weight                     int64   `json:"weight"`
	Temperature                int64   `json:"temperature"`
	Humidity                   int64   `json:"humidity"`
	CO2                        int64   `json:"co2"`
	Generation                 int64   `json:"generation"`
	OperationID                string  `json:"operation_id"`
	LogicalTime                int64   `json:"logical_time"`
}

// PesticideEventRequest is the body of POST /v1/pesticide-events.
type PesticideEventRequest struct {
	CycleID       string `json:"cycle_id"`
	AppliedZoneID string `json:"applied_zone_id"`
	OccurredAt    int64  `json:"occurred_at"`
	ArrivedAt     int64  `json:"arrived_at"`
	ReentryHours  int64  `json:"reentry_hours"`
	OperationID   string `json:"operation_id"`
}

// RecoveryCheckRequest is the body of POST /v1/cycles/{id}/recovery-checks.
type RecoveryCheckRequest struct {
	Withdrawn       bool   `json:"withdrawn"`
	ResidueVerified bool   `json:"residue_verified"`
	ReentryReached  bool   `json:"reentry_reached"`
	OperationID     string `json:"operation_id"`
	LogicalTime     int64  `json:"logical_time"`
}

// ReviewRequest is the body of POST /v1/cycles/{id}/reviews.
type ReviewRequest struct {
	ReviewerID   string `json:"reviewer_id"`
	EvidenceRoot string `json:"evidence_root"`
	OperationID  string `json:"operation_id"`
	LogicalTime  int64  `json:"logical_time"`
}

// CompleteRequest is the body of POST /v1/cycles/{id}/complete.
type CompleteRequest struct {
	OperationID string `json:"operation_id"`
	LogicalTime int64  `json:"logical_time"`
}

// CycleView is the public snapshot of a cycle.
type CycleView struct {
	CycleID         string `json:"cycle_id"`
	State           string `json:"state"`
	TaskGeneration  int64  `json:"task_generation"`
	RecoveryGen     int64  `json:"recovery_generation"`
	RuleDigest      string `json:"rule_digest"`
	EvidenceRoot    string `json:"evidence_root"`
	EvidenceCount   int    `json:"evidence_count"`
	BudgetInitial   int64  `json:"budget_initial"`
	BudgetRemaining int64  `json:"budget_remaining"`
	BudgetDebited   int64  `json:"budget_debited"`
	CoverageCells   int    `json:"coverage_cells"`
	CoverageFilled  int    `json:"coverage_filled"`
	TerminalVersion int64  `json:"terminal_version"`
	TerminalKind    string `json:"terminal_kind,omitempty"`
}

// ObservationResult is the response for a submitted observation.
type ObservationResult struct {
	Status      string              `json:"status"`
	EvidenceSeq int64               `json:"evidence_seq,omitempty"`
	Derived     []DerivedMetricView `json:"derived,omitempty"`
}

// DerivedMetricView is a derived metric in the observation response.
type DerivedMetricView struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
	Scale int64  `json:"scale"`
	Valid bool   `json:"valid"`
}

// DeviceRetryResult is the response for a device retry.
type DeviceRetryResult struct {
	Status   string `json:"status"`
	Attempts int    `json:"attempts"`
	Result   string `json:"result,omitempty"`
}

// PesticideEventResult is the response for registering a pesticide event.
type PesticideEventResult struct {
	EventID       string   `json:"event_id"`
	AffectedZones []string `json:"affected_zones"`
	RecoveryGen   int64    `json:"recovery_generation"`
	Late          bool     `json:"late"`
}

// RecoveryCheckResult is the response for a recovery check.
type RecoveryCheckResult struct {
	Ready       bool   `json:"ready"`
	State       string `json:"state"`
	RecoveryGen int64  `json:"recovery_generation"`
}

// SupplementResult is the response for a supplement request.
type SupplementResult struct {
	Ranges []SupplementRangeView `json:"ranges"`
	State  string                `json:"state"`
}

// SupplementRangeView is one merged supplement range.
type SupplementRangeView struct {
	ZoneID     string `json:"zone_id"`
	BatchID    string `json:"batch_id"`
	PointStart int64  `json:"point_start"`
	PointEnd   int64  `json:"point_end"`
}

// ReviewResult is the response for a review.
type ReviewResult struct {
	Accepted      bool   `json:"accepted"`
	State         string `json:"state"`
	ReviewerCount int    `json:"reviewer_count"`
}

// TerminalResult is the response for complete/withdraw/cancel.
type TerminalResult struct {
	State        string `json:"state"`
	Kind         string `json:"kind"`
	CredentialID string `json:"credential_id,omitempty"`
}

// EvidenceView is one evidence event in the ordered stream.
type EvidenceView struct {
	Seq         int64  `json:"seq"`
	Kind        string `json:"kind"`
	LogicalTime int64  `json:"logical_time"`
	Generation  int64  `json:"generation"`
	Digest      string `json:"digest"`
}

// HealthView is the response of GET /healthz.
type HealthView struct {
	Status           string `json:"status"`
	Cycles           int    `json:"cycles"`
	PendingRetries   int    `json:"pending_retries"`
	UnfinishedCycles int    `json:"unfinished_cycles"`
	Now              string `json:"now"`
}

func healthNow() string { return time.Now().UTC().Format(time.RFC3339) }
