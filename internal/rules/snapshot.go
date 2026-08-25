package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// RuleSnapshot is the immutable set of rules referenced by a locked cycle. It
// is built once at lock time and never mutated afterwards, so a cycle observes
// a stable world even if the catalog later advances.
type RuleSnapshot struct {
	Version            int64
	Digest             string
	GreenhouseID       string
	Zones              []Zone
	Adjacencies        []ZoneAdjacency
	Colonies           []Colony
	Certificates       []HealthCertificate
	Reviewers          []Reviewer
	Batches            []InflorescenceBatch
	CompatByZone       map[string][]string
	DriftLayersDefault int
}

// Compatible reports whether two zones may host each other's colonies given the
// snapshot compatibility rules: zones are compatible when they share at least
// one compatibility tag, or when either side declares no tags (open zone).
func (s RuleSnapshot) Compatible(a, b string) bool {
	ta := s.tagsFor(a)
	tb := s.tagsFor(b)
	if len(ta) == 0 || len(tb) == 0 {
		return true
	}
	set := make(map[string]bool, len(ta))
	for _, t := range ta {
		set[t] = true
	}
	for _, t := range tb {
		if set[t] {
			return true
		}
	}
	return false
}

func (s RuleSnapshot) tagsFor(zoneID string) []string {
	return s.CompatByZone[zoneID]
}

// ReviewerQualified reports whether a reviewer exists and is qualified.
func (s RuleSnapshot) ReviewerQualified(id string) bool {
	for _, r := range s.Reviewers {
		if r.ID == id {
			return r.Qualified
		}
	}
	return false
}

// ComputeDigest derives a stable content digest from the snapshot fields. It is
// used as the evidence root seed and for idempotent operation summaries.
func (s RuleSnapshot) ComputeDigest() string {
	h := sha256.New()
	h.Write([]byte(s.GreenhouseID))
	for _, z := range sortedZones(s.Zones) {
		h.Write([]byte(z.ID))
		h.Write([]byte{0})
		for _, t := range z.CompatTags {
			h.Write([]byte(t))
			h.Write([]byte{0})
		}
	}
	for _, a := range s.Adjacencies {
		h.Write([]byte(a.FromZoneID))
		h.Write([]byte(a.ToZoneID))
		h.Write([]byte{byte(a.Distance)})
	}
	for _, c := range s.Colonies {
		h.Write([]byte(c.ID))
		h.Write([]byte{0})
		h.Write([]byte{byte(c.QueenGeneration.Generation)})
	}
	for _, cert := range s.Certificates {
		h.Write([]byte(cert.ColonyID))
		h.Write([]byte(cert.Digest))
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

func sortedZones(zones []Zone) []Zone {
	out := append([]Zone(nil), zones...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// BuildSnapshot assembles an immutable snapshot from catalog entries and the
// locked rule version. It returns an error if any referenced entity is missing
// or a health certificate is stale relative to the lock logical time.
func BuildSnapshot(c Catalog, greenhouseID string, version int64, colonyIDs, batchIDs, reviewerIDs []string, at int64) (RuleSnapshot, error) {
	g, err := c.Greenhouse(greenhouseID)
	if err != nil {
		return RuleSnapshot{}, err
	}
	snap := RuleSnapshot{
		Version:      version,
		GreenhouseID: greenhouseID,
		CompatByZone: make(map[string][]string),
	}
	for _, zoneID := range g.Zones {
		z, err := c.Zone(zoneID)
		if err != nil {
			return RuleSnapshot{}, err
		}
		snap.Zones = append(snap.Zones, z)
		snap.CompatByZone[zoneID] = append([]string(nil), z.CompatTags...)
	}
	adj, err := c.Adjacencies(greenhouseID)
	if err != nil {
		return RuleSnapshot{}, err
	}
	snap.Adjacencies = append(snap.Adjacencies, adj...)

	for _, colonyID := range colonyIDs {
		col, err := c.Colony(colonyID)
		if err != nil {
			return RuleSnapshot{}, err
		}
		cert, err := c.Certificate(colonyID, at)
		if err != nil {
			return RuleSnapshot{}, err
		}
		if !cert.Valid(at) {
			return RuleSnapshot{}, ErrStaleCertificate{ColonyID: colonyID}
		}
		snap.Colonies = append(snap.Colonies, col)
		snap.Certificates = append(snap.Certificates, cert)
	}
	for _, batchID := range batchIDs {
		b, err := c.Batch(batchID)
		if err != nil {
			return RuleSnapshot{}, err
		}
		snap.Batches = append(snap.Batches, b)
	}
	for _, reviewerID := range reviewerIDs {
		r, err := c.Reviewer(reviewerID)
		if err != nil {
			return RuleSnapshot{}, err
		}
		snap.Reviewers = append(snap.Reviewers, r)
	}
	snap.Digest = snap.ComputeDigest()
	return snap, nil
}

// ErrStaleCertificate indicates a colony health certificate was not valid at the
// lock logical time, so the whole lock must fail before any occupancy exists.
type ErrStaleCertificate struct {
	ColonyID string
}

func (e ErrStaleCertificate) Error() string {
	return "rules: stale health certificate for colony " + e.ColonyID
}
