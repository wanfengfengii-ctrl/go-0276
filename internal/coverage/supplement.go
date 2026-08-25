package coverage

import "sort"

// SupplementTrigger describes one uncovered, low-return or weight-loss cell.
type SupplementTrigger struct {
	ZoneID               string
	InflorescenceBatchID string
	ObservationPoint     int64
	Reason               string
}

// SupplementScope is the deterministic merged supplement range. Adjacent
// observation points in the same zone and batch are merged into one range.
type SupplementScope struct {
	Ranges []SupplementRange
}

// SupplementRange is a merged contiguous observation-point span.
type SupplementRange struct {
	ZoneID               string
	InflorescenceBatchID string
	PointStart           int64
	PointEnd             int64
}

// MergeSupplementTriggers deduplicates and merges triggers that share a zone
// and batch and are adjacent in observation point, producing a stable sorted
// scope. The same trigger set always yields the same single scope.
func MergeSupplementTriggers(triggers []SupplementTrigger) SupplementScope {
	seen := map[string]bool{}
	var uniq []SupplementTrigger
	for _, t := range triggers {
		k := t.ZoneID + "|" + t.InflorescenceBatchID + "|" + itoa(t.ObservationPoint)
		if seen[k] {
			continue
		}
		seen[k] = true
		uniq = append(uniq, t)
	}
	sort.SliceStable(uniq, func(i, j int) bool {
		if uniq[i].ZoneID != uniq[j].ZoneID {
			return uniq[i].ZoneID < uniq[j].ZoneID
		}
		if uniq[i].InflorescenceBatchID != uniq[j].InflorescenceBatchID {
			return uniq[i].InflorescenceBatchID < uniq[j].InflorescenceBatchID
		}
		return uniq[i].ObservationPoint < uniq[j].ObservationPoint
	})

	var scope SupplementScope
	for _, t := range uniq {
		n := len(scope.Ranges)
		if n > 0 {
			last := &scope.Ranges[n-1]
			if last.ZoneID == t.ZoneID && last.InflorescenceBatchID == t.InflorescenceBatchID &&
				t.ObservationPoint == last.PointEnd {
				last.PointEnd = t.ObservationPoint + 1
				continue
			}
		}
		scope.Ranges = append(scope.Ranges, SupplementRange{
			ZoneID:               t.ZoneID,
			InflorescenceBatchID: t.InflorescenceBatchID,
			PointStart:           t.ObservationPoint,
			PointEnd:             t.ObservationPoint + 1,
		})
	}
	return scope
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
