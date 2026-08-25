package cycle

// TerminalKind enumerates the mutually exclusive terminal outcomes.
type TerminalKind uint8

const (
	TerminalNone TerminalKind = iota
	TerminalCompleted
	TerminalWithdrawn
	TerminalCancelled
)

// TerminalCredential is the single persisted outcome of a closed cycle. Only one
// of completed / withdrawn / cancelled may ever be written for a given cycle.
type TerminalCredential struct {
	CycleID      string
	Kind         TerminalKind
	Version      int64
	EvidenceRoot string
	ReviewerOne  string
	ReviewerTwo  string
	CredentialID string
}
