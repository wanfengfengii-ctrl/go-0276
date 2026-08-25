package safety

// ArbitrationOutcome is the result of a terminal competition. Only one request
// may write a terminal credential; all others lose with a stable code.
type ArbitrationOutcome struct {
	Won  bool
	Kind string
}

// ResolveArbitration deterministically picks the winning terminal request given
// the candidate kind and the kind already committed (nil when undecided). A
// loser receives the committed kind so it can surface a stable error.
func ResolveArbitration(existing *string, requested string) ArbitrationOutcome {
	if existing == nil || *existing == "" {
		return ArbitrationOutcome{Won: true, Kind: requested}
	}
	return ArbitrationOutcome{Won: false, Kind: *existing}
}
