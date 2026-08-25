package occupancy

// ConflictGraph adjudicates overlapping window occupancy for a cycle. Two
// occupancies conflict when the same colony overlaps in time, or when the same
// zone overlaps in time but the rule snapshot deems the colonies incompatible.
type ConflictGraph struct {
	occupancies []WindowOccupancy
}

// NewConflictGraph returns an empty conflict graph.
func NewConflictGraph() *ConflictGraph { return &ConflictGraph{} }

// Add records an occupancy for conflict detection.
func (g *ConflictGraph) Add(o WindowOccupancy) { g.occupancies = append(g.occupancies, o) }

// All returns the occupancies currently in the graph.
func (g *ConflictGraph) All() []WindowOccupancy {
	return append([]WindowOccupancy(nil), g.occupancies...)
}

// FindColonyOverlaps returns occupancies whose window overlaps the candidate
// and which belong to the same colony.
func (g *ConflictGraph) FindColonyOverlaps(candidate WindowOccupancy) []WindowOccupancy {
	var out []WindowOccupancy
	for _, o := range g.occupancies {
		if o.ColonyID == candidate.ColonyID && o.Window.Overlaps(candidate.Window) {
			out = append(out, o)
		}
	}
	return out
}
