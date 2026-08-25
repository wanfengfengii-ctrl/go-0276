package safety

// Review is an independent review signing the current evidence root digest.
type Review struct {
	CycleID      string
	ReviewerID   string
	EvidenceRoot string
	SignedAt     int64
}

// ReviewsIndependent reports whether two reviews come from distinct reviewers.
func ReviewsIndependent(a, b Review) bool {
	return a.ReviewerID != "" && a.ReviewerID != b.ReviewerID
}

// ReviewsFor returns the reviews matching a cycle and evidence root.
func ReviewsFor(reviews []Review, cycleID, root string) []Review {
	var out []Review
	for _, r := range reviews {
		if r.CycleID == cycleID && r.EvidenceRoot == root {
			out = append(out, r)
		}
	}
	return out
}
