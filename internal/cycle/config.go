package cycle

// LockConfig captures the locked parameters of a cycle beyond the rule snapshot:
// retry policy, environment thresholds, drift layers, reentry hours and the
// observation points that make up the coverage matrix.
type LockConfig struct {
	CycleID           string
	RuleDigest        string
	TaskGeneration    int64
	RetryMaxAttempts  int
	RetryBaseDelay    int64
	RetryStepDelay    int64
	EnvTempLow        int64
	EnvTempHigh       int64
	EnvHumidityLow    int64
	EnvHumidityHigh   int64
	EnvCO2Low         int64
	EnvCO2High        int64
	DriftLayers       int
	ReentryHours      int64
	ObservationPoints []int64
}
