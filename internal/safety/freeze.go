package safety

import "sort"

// FreezeBarrier is the frozen window applied to affected zones and occupancies.
type FreezeBarrier struct {
	ID            string
	CycleID       string
	AffectedZones []string
	WindowStart   int64
	WindowEnd     int64
	RecoveryGen   int64
}

// Affects reports whether the barrier freezes the given zone at the given time.
func (b FreezeBarrier) Affects(zoneID string, at int64) bool {
	for _, z := range b.AffectedZones {
		if z == zoneID && at >= b.WindowStart && at < b.WindowEnd {
			return true
		}
	}
	return false
}

// PropagateFreeze computes the affected zones from an applied zone, a drift
// adjacency graph and the number of drift layers. It returns the applied zone
// plus every zone reachable within the given number of adjacency hops.
func PropagateFreeze(appliedZone string, adjacencies []Adjacency, layers int) []string {
	affected := map[string]bool{appliedZone: true}
	frontier := []string{appliedZone}
	for i := 0; i < layers; i++ {
		var next []string
		for _, from := range frontier {
			for _, a := range adjacencies {
				if a.From == from && !affected[a.To] {
					affected[a.To] = true
					next = append(next, a.To)
				}
			}
		}
		frontier = next
	}
	out := make([]string, 0, len(affected))
	for z := range affected {
		out = append(out, z)
	}
	sort.Strings(out)
	return out
}

// Adjacency is a directed drift edge between two zones.
type Adjacency struct {
	From string
	To   string
}
