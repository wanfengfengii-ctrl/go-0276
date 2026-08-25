package rules

// MemoryCatalog is an in-memory implementation of Catalog used by the service
// and by deterministic tests. It stores greenhouses, zones, adjacencies,
// colonies, certificates, reviewers and batches in plain maps.
type MemoryCatalog struct {
	greenhouses  map[string]Greenhouse
	zones        map[string]Zone
	adjacencies  map[string][]ZoneAdjacency
	colonies     map[string]Colony
	certificates map[string]HealthCertificate
	reviewers    map[string]Reviewer
	batches      map[string]InflorescenceBatch
}

// NewMemoryCatalog returns an empty catalog.
func NewMemoryCatalog() *MemoryCatalog {
	return &MemoryCatalog{
		greenhouses:  make(map[string]Greenhouse),
		zones:        make(map[string]Zone),
		adjacencies:  make(map[string][]ZoneAdjacency),
		colonies:     make(map[string]Colony),
		certificates: make(map[string]HealthCertificate),
		reviewers:    make(map[string]Reviewer),
		batches:      make(map[string]InflorescenceBatch),
	}
}

// AddGreenhouse registers a greenhouse.
func (c *MemoryCatalog) AddGreenhouse(g Greenhouse) { c.greenhouses[g.ID] = g }

// AddZone registers a zone.
func (c *MemoryCatalog) AddZone(z Zone) { c.zones[z.ID] = z }

// AddAdjacency registers a directed drift adjacency edge.
func (c *MemoryCatalog) AddAdjacency(a ZoneAdjacency) {
	c.adjacencies[a.FromZoneID] = append(c.adjacencies[a.FromZoneID], a)
}

// AddColony registers a colony.
func (c *MemoryCatalog) AddColony(col Colony) { c.colonies[col.ID] = col }

// AddCertificate registers the latest certificate for a colony.
func (c *MemoryCatalog) AddCertificate(cert HealthCertificate) {
	c.certificates[cert.ColonyID] = cert
}

// AddReviewer registers a reviewer.
func (c *MemoryCatalog) AddReviewer(r Reviewer) { c.reviewers[r.ID] = r }

// AddBatch registers an inflorescence batch.
func (c *MemoryCatalog) AddBatch(b InflorescenceBatch) { c.batches[b.ID] = b }

// Greenhouse implements Catalog.
func (c *MemoryCatalog) Greenhouse(id string) (Greenhouse, error) {
	g, ok := c.greenhouses[id]
	if !ok {
		return Greenhouse{}, ErrNotFound
	}
	return g, nil
}

// Zone implements Catalog.
func (c *MemoryCatalog) Zone(id string) (Zone, error) {
	z, ok := c.zones[id]
	if !ok {
		return Zone{}, ErrNotFound
	}
	return z, nil
}

// Adjacencies implements Catalog.
func (c *MemoryCatalog) Adjacencies(greenhouseID string) ([]ZoneAdjacency, error) {
	// Return all adjacencies whose "from" zone belongs to the greenhouse.
	var out []ZoneAdjacency
	for _, from := range c.zonesOf(greenhouseID) {
		out = append(out, c.adjacencies[from]...)
	}
	return out, nil
}

func (c *MemoryCatalog) zonesOf(greenhouseID string) []string {
	var ids []string
	for _, z := range c.zones {
		if z.GreenhouseID == greenhouseID {
			ids = append(ids, z.ID)
		}
	}
	return ids
}

// Colony implements Catalog.
func (c *MemoryCatalog) Colony(id string) (Colony, error) {
	col, ok := c.colonies[id]
	if !ok {
		return Colony{}, ErrNotFound
	}
	return col, nil
}

// Certificate implements Catalog.
func (c *MemoryCatalog) Certificate(colonyID string, at int64) (HealthCertificate, error) {
	cert, ok := c.certificates[colonyID]
	if !ok {
		return HealthCertificate{}, ErrNotFound
	}
	return cert, nil
}

// Reviewer implements Catalog.
func (c *MemoryCatalog) Reviewer(id string) (Reviewer, error) {
	r, ok := c.reviewers[id]
	if !ok {
		return Reviewer{}, ErrNotFound
	}
	return r, nil
}

// Batch implements Catalog.
func (c *MemoryCatalog) Batch(id string) (InflorescenceBatch, error) {
	b, ok := c.batches[id]
	if !ok {
		return InflorescenceBatch{}, ErrNotFound
	}
	return b, nil
}
