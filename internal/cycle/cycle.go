package cycle

// PollinationCycle is the aggregate root for a single pollination cycle. It is
// the consistency boundary for zones, inflorescence batches, colonies, queen
// generations, device readings, pesticide events and reviewers.
type PollinationCycle struct {
	ID             string
	State          State
	TaskGeneration int64
	RecoveryGen    int64
	LogicalTime    int64
	RuleDigest     string
	EvidenceSeq    int64
	EvidenceRoot   string
	TerminalVer    int64
}

// CycleSnapshot is an immutable view of a cycle referenced after locking.
type CycleSnapshot struct {
	CycleID         string
	State           State
	TaskGeneration  int64
	RecoveryGen     int64
	RuleDigest      string
	EvidenceRoot    string
	TerminalVersion int64
}

// Advance moves the cycle to target only if the transition is legal. It returns
// false when the transition is rejected so callers surface a stable error.
func (c *PollinationCycle) Advance(target State) bool {
	if !c.State.CanTransitionTo(target) {
		return false
	}
	c.State = target
	return true
}

// Snapshot returns an immutable view of the current aggregate state.
func (c *PollinationCycle) Snapshot() CycleSnapshot {
	return CycleSnapshot{
		CycleID:         c.ID,
		State:           c.State,
		TaskGeneration:  c.TaskGeneration,
		RecoveryGen:     c.RecoveryGen,
		RuleDigest:      c.RuleDigest,
		EvidenceRoot:    c.EvidenceRoot,
		TerminalVersion: c.TerminalVer,
	}
}
