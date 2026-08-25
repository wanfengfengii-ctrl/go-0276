package cycle

// State is the lifecycle state of a pollination cycle aggregate.
type State uint8

const (
	// StatePendingLock is the initial state before rules are locked.
	StatePendingLock State = iota
	// StatePendingAdmit means rules are locked and colonies may be admitted.
	StatePendingAdmit
	// StateInFlight means colonies are admitted and roaming.
	StateInFlight
	// StateSafetyFrozen means a pesticide freeze barrier is active.
	StateSafetyFrozen
	// StatePendingRecoveryCheck means freeze recovery checks are awaited.
	StatePendingRecoveryCheck
	// StateSupplementing means a deterministic supplement task is active.
	StateSupplementing
	// StatePendingCloseReview means two independent reviews are awaited.
	StatePendingCloseReview
	// StateCompleted is a terminal state with a unique credential.
	StateCompleted
	// StateColonyWithdrawn is a terminal state.
	StateColonyWithdrawn
	// StateCancelled is a terminal state.
	StateCancelled
)

// String returns a stable, human-readable name for the state.
func (s State) String() string {
	switch s {
	case StatePendingLock:
		return "pending_lock"
	case StatePendingAdmit:
		return "pending_admit"
	case StateInFlight:
		return "in_flight"
	case StateSafetyFrozen:
		return "safety_frozen"
	case StatePendingRecoveryCheck:
		return "pending_recovery_check"
	case StateSupplementing:
		return "supplementing"
	case StatePendingCloseReview:
		return "pending_close_review"
	case StateCompleted:
		return "completed"
	case StateColonyWithdrawn:
		return "colony_withdrawn"
	case StateCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// IsTerminal reports whether the state admits no further transitions.
func (s State) IsTerminal() bool {
	switch s {
	case StateCompleted, StateColonyWithdrawn, StateCancelled:
		return true
	default:
		return false
	}
}

// CanTransitionTo reports whether s may legally advance to target.
func (s State) CanTransitionTo(target State) bool {
	if s.IsTerminal() {
		return false
	}
	switch s {
	case StatePendingLock:
		return target == StatePendingAdmit
	case StatePendingAdmit:
		return target == StateInFlight || target == StateCancelled
	case StateInFlight:
		return target == StateSafetyFrozen || target == StateSupplementing ||
			target == StatePendingCloseReview || target == StateColonyWithdrawn ||
			target == StateCancelled
	case StateSafetyFrozen:
		return target == StatePendingRecoveryCheck || target == StateCancelled
	case StatePendingRecoveryCheck:
		return target == StateInFlight || target == StateCancelled
	case StateSupplementing:
		return target == StateInFlight || target == StatePendingCloseReview ||
			target == StateCancelled
	case StatePendingCloseReview:
		return target == StateCompleted || target == StateColonyWithdrawn ||
			target == StateCancelled
	default:
		return false
	}
}
