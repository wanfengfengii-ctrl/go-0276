package occupancy

// OccupancyPurpose describes why a colony occupies a window.
type OccupancyPurpose uint8

const (
	PurposeAdmit OccupancyPurpose = iota
	PurposeRotate
	PurposeSupplement
)

// String returns a stable name for the purpose.
func (p OccupancyPurpose) String() string {
	switch p {
	case PurposeAdmit:
		return "admit"
	case PurposeRotate:
		return "rotate"
	case PurposeSupplement:
		return "supplement"
	default:
		return "unknown"
	}
}

// WindowOccupancy binds a colony to a zone across a half-open window.
type WindowOccupancy struct {
	ID         string
	CycleID    string
	ColonyID   string
	ZoneID     string
	Window     Interval
	Purpose    OccupancyPurpose
	Generation int64
	Version    int64
}

// ConflictsWith reports whether two occupancies of the same colony overlap in time.
func (o WindowOccupancy) ConflictsWith(other WindowOccupancy) bool {
	if o.ColonyID != other.ColonyID {
		return false
	}
	return o.Window.Overlaps(other.Window)
}
