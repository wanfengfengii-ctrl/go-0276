// Package rules implements the greenhouse & colony rule catalog: greenhouses,
// zones, drift adjacency, colonies, queen generations, health certificates,
// reviewers and inflorescence batches. After a cycle is locked it references
// only immutable snapshots of these rules.
package rules

import "fmt"

// Greenhouse identifies a facility and its immutable rule version.
type Greenhouse struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Version int64    `json:"version"`
	Zones   []string `json:"zones"`
}

// Zone is a partition within a greenhouse with a capacity and compatibility tags.
type Zone struct {
	ID           string   `json:"id"`
	GreenhouseID string   `json:"greenhouse_id"`
	Capacity     int      `json:"capacity"`
	CompatTags   []string `json:"compat_tags"`
}

// ZoneAdjacency is a directed drift adjacency edge between two zones.
// Distance is the drift-adjacency level (1 = direct neighbour).
type ZoneAdjacency struct {
	FromZoneID string `json:"from_zone_id"`
	ToZoneID   string `json:"to_zone_id"`
	Distance   int    `json:"distance"`
}

// QueenGeneration records the generation of a colony's queen.
type QueenGeneration struct {
	ColonyID   string `json:"colony_id"`
	Generation int    `json:"generation"`
}

// Colony is a bumblebee colony and its current health status.
type Colony struct {
	ID              string          `json:"id"`
	GreenhouseID    string          `json:"greenhouse_id"`
	QueenGeneration QueenGeneration `json:"queen_generation"`
	HealthStatus    string          `json:"health_status"`
}

// HealthCertificate is an immutable summary of a colony's health proof.
// Historical versions must never be overwritten.
type HealthCertificate struct {
	ColonyID   string `json:"colony_id"`
	Digest     string `json:"digest"`
	IssuedAt   int64  `json:"issued_at"`
	ValidFrom  int64  `json:"valid_from"`
	ValidUntil int64  `json:"valid_until"`
}

// Valid reports whether the certificate is effective at the given logical time.
func (c HealthCertificate) Valid(at int64) bool {
	return at >= c.ValidFrom && at < c.ValidUntil
}

// Reviewer is a qualified person allowed to sign an independent review.
type Reviewer struct {
	ID        string `json:"id"`
	Qualified bool   `json:"qualified"`
}

// InflorescenceBatch is a set of sampled inflorescences in a zone.
type InflorescenceBatch struct {
	ID                    string `json:"id"`
	ZoneID                string `json:"zone_id"`
	SampledInflorescences int64  `json:"sampled_inflorescences"`
}

// Catalog is the read model for the rule catalog. Locked cycles consume
// immutable snapshots built from this interface.
type Catalog interface {
	Greenhouse(id string) (Greenhouse, error)
	Zone(id string) (Zone, error)
	Adjacencies(greenhouseID string) ([]ZoneAdjacency, error)
	Colony(id string) (Colony, error)
	Certificate(colonyID string, at int64) (HealthCertificate, error)
	Reviewer(id string) (Reviewer, error)
	Batch(id string) (InflorescenceBatch, error)
}

// ErrNotFound is returned when a catalog entry does not exist.
var ErrNotFound = fmt.Errorf("rules: not found")
