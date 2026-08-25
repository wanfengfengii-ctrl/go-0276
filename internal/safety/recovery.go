package safety

// RecoveryCredential records whether a colony withdrawal, residue check and
// reentry time have all been satisfied before recovery.
type RecoveryCredential struct {
	ID               string
	CycleID          string
	RecoveryGen      int64
	Withdrawn        bool
	ResidueVerified  bool
	ReentryReached   bool
	InvalidatedByGen int64
	InvalidReason    string
}

// Ready reports whether the credential satisfies all recovery gates and has not
// been invalidated by a later recovery generation.
func (c RecoveryCredential) Ready() bool {
	return c.InvalidatedByGen == 0 && c.Withdrawn && c.ResidueVerified && c.ReentryReached
}

// Invalidate marks the credential invalid because a later generation superseded it.
func (c *RecoveryCredential) Invalidate(byGen int64, reason string) {
	c.InvalidatedByGen = byGen
	c.InvalidReason = reason
}
