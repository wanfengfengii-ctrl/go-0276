package occupancy

// CarrierRisk lists colonies currently occupying zones other than the target
// that are incompatible with the target zone under the supplied compatibility
// predicate. Rotation must reject a move when a carried colony would be placed
// in an incompatible target zone.
func CarrierRisk(occupancies []WindowOccupancy, toZone string, compatible func(a, b string) bool) []string {
	var risk []string
	for _, o := range occupancies {
		if o.ZoneID != toZone && !compatible(o.ZoneID, toZone) {
			risk = append(risk, o.ColonyID)
		}
	}
	return risk
}

// SourceOccupancies returns the occupancies a colony currently holds in the
// declared source zone. Rotation releases only these occupancies: a colony can
// only be moved out of a zone it actually occupies. An empty result means the
// declared from-zone does not match the colony's current occupancy, so the old
// occupancy must not be released and the rotation must be rejected.
func SourceOccupancies(occupancies []WindowOccupancy, colonyID, fromZoneID string) []WindowOccupancy {
	var src []WindowOccupancy
	for _, o := range occupancies {
		if o.ColonyID == colonyID && o.ZoneID == fromZoneID {
			src = append(src, o)
		}
	}
	return src
}
