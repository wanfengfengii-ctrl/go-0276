package occupancy

// BudgetEntry is an append-only debit against a flight budget account.
type BudgetEntry struct {
	Seq         int64
	AccountID   string
	Amount      int64
	Generation  int64
	LogicalTime int64
}

// FlightBudgetAccount tracks an initial budget and its remaining balance. The
// conservation check requires initial == debited + remaining.
type FlightBudgetAccount struct {
	ID        string
	CycleID   string
	Initial   int64
	Remaining int64
	Debited   int64
	Entries   []BudgetEntry
}

// Debit reduces the remaining balance and records the debit. It returns false
// without mutation when the account would overdraw.
func (a *FlightBudgetAccount) Debit(amount, generation, logicalTime int64) bool {
	if amount < 0 || amount > a.Remaining {
		return false
	}
	a.Remaining -= amount
	a.Debited += amount
	a.Entries = append(a.Entries, BudgetEntry{
		Seq:         int64(len(a.Entries)) + 1,
		AccountID:   a.ID,
		Amount:      amount,
		Generation:  generation,
		LogicalTime: logicalTime,
	})
	return true
}

// Conserved reports whether the account satisfies the conservation invariant.
func (a FlightBudgetAccount) Conserved() bool {
	return a.Initial == a.Debited+a.Remaining
}
