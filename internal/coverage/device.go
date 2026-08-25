package coverage

// DeviceType enumerates the device adapters the roaming flow may call.
type DeviceType string

const (
	DeviceEnvironmentProbe DeviceType = "environment_probe"
	DeviceHiveScale        DeviceType = "hive_scale"
	DeviceCounterAdapter   DeviceType = "counter_adapter"
)

// DeviceResultStatus is the explicit outcome of a device call. The device
// boundary accepts only an explicit success value or an explicit failure
// category; it never fabricates a reading.
type DeviceResultStatus string

const (
	DeviceSuccess       DeviceResultStatus = "success"
	DeviceRejected      DeviceResultStatus = "rejected"
	DeviceDisconnected  DeviceResultStatus = "disconnected"
	DeviceTimeout       DeviceResultStatus = "timeout"
	DeviceMalformed     DeviceResultStatus = "malformed"
	DevicePermanentFail DeviceResultStatus = "permanent_failure"
)

// DeviceCall is one persisted invocation of a device adapter.
type DeviceCall struct {
	CycleID          string
	CallSeq          int64
	DeviceType       DeviceType
	ZoneID           string
	ObservationPoint int64
	LogicalTime      int64
	Status           DeviceResultStatus
	Result           string
}

// FailureScript is a deterministic, controllable device failure script used to
// drive rejections, disconnects, timeouts and malformed responses for tests.
type FailureScript struct {
	DeviceType DeviceType
	CycleID    string
	Outcomes   []DeviceResultStatus
	Index      int
}

// Next returns the next scripted outcome, cycling through the configured
// outcomes deterministically. An empty script always succeeds.
func (f *FailureScript) Next() DeviceResultStatus {
	if len(f.Outcomes) == 0 {
		return DeviceSuccess
	}
	out := f.Outcomes[f.Index%len(f.Outcomes)]
	f.Index++
	return out
}
