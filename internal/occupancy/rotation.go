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
