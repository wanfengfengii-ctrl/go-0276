package httpapi

import (
	"testing"

	"tomato-bumblebee-pollination-rotation/internal/coverage"
	"tomato-bumblebee-pollination-rotation/internal/rules"
	"tomato-bumblebee-pollination-rotation/internal/store"
)

// defaultCatalog builds a deterministic three-zone greenhouse catalog used by
// all service tests. Zone adjacency: z1<->z2 and z2->z3.
func defaultCatalog() *rules.MemoryCatalog {
	c := rules.NewMemoryCatalog()
	c.AddGreenhouse(rules.Greenhouse{ID: "g1", Name: "demo", Version: 1, Zones: []string{"z1", "z2", "z3"}})
	c.AddZone(rules.Zone{ID: "z1", GreenhouseID: "g1", Capacity: 8, CompatTags: []string{"a"}})
	c.AddZone(rules.Zone{ID: "z2", GreenhouseID: "g1", Capacity: 8, CompatTags: []string{"a"}})
	c.AddZone(rules.Zone{ID: "z3", GreenhouseID: "g1", Capacity: 8, CompatTags: []string{"b"}})
	c.AddAdjacency(rules.ZoneAdjacency{FromZoneID: "z1", ToZoneID: "z2", Distance: 1})
	c.AddAdjacency(rules.ZoneAdjacency{FromZoneID: "z2", ToZoneID: "z1", Distance: 1})
	c.AddAdjacency(rules.ZoneAdjacency{FromZoneID: "z2", ToZoneID: "z3", Distance: 1})
	c.AddColony(rules.Colony{ID: "c1", GreenhouseID: "g1", QueenGeneration: rules.QueenGeneration{ColonyID: "c1", Generation: 1}, HealthStatus: "healthy"})
	c.AddColony(rules.Colony{ID: "c2", GreenhouseID: "g1", QueenGeneration: rules.QueenGeneration{ColonyID: "c2", Generation: 1}, HealthStatus: "healthy"})
	c.AddCertificate(rules.HealthCertificate{ColonyID: "c1", Digest: "cert-c1", IssuedAt: 0, ValidFrom: 0, ValidUntil: 1 << 40})
	c.AddCertificate(rules.HealthCertificate{ColonyID: "c2", Digest: "cert-c2", IssuedAt: 0, ValidFrom: 0, ValidUntil: 1 << 40})
	c.AddReviewer(rules.Reviewer{ID: "r1", Qualified: true})
	c.AddReviewer(rules.Reviewer{ID: "r2", Qualified: true})
	c.AddBatch(rules.InflorescenceBatch{ID: "b1", ZoneID: "z1", SampledInflorescences: 4})
	c.AddBatch(rules.InflorescenceBatch{ID: "b2", ZoneID: "z2", SampledInflorescences: 4})
	c.AddBatch(rules.InflorescenceBatch{ID: "b3", ZoneID: "z3", SampledInflorescences: 4})
	return c
}

func newTestService(t *testing.T) (*Service, *store.DB) {
	t.Helper()
	st := store.NewMemoryStore()
	svc := NewService(st, defaultCatalog(), NewScriptDriver())
	return svc, st
}

// validLockRequest returns a lock request for g1 with zones z1/z2, two
// observation points and a budget of 10.
func validLockRequest(cycleID string) LockRequest {
	return LockRequest{
		CycleID:           cycleID,
		GreenhouseID:      "g1",
		RuleVersion:       1,
		ColonyIDs:         []string{"c1", "c2"},
		BatchIDs:          []string{"b1", "b2"},
		ReviewerIDs:       []string{"r1", "r2"},
		ObservationPoints: []int64{1, 2},
		Budget:            10,
		Windows: []WindowSpec{
			{ColonyID: "c1", ZoneID: "z1", Start: 0, End: 100},
			{ColonyID: "c2", ZoneID: "z2", Start: 0, End: 100},
		},
		EnvThresholds: EnvThresholds{TempLow: 0, TempHigh: 1000, HumLow: 0, HumHigh: 1000, CO2Low: 0, CO2High: 5000},
		RetrySpec:     RetrySpec{MaxAttempts: 3, BaseDelay: 1, StepDelay: 2},
		DriftLayers:   1,
		ReentryHours:  24,
		OperationID:   "op-" + cycleID,
		LogicalTime:   1000,
	}
}

func validObservation(zone, batch string, point int64) ObservationRequest {
	return ObservationRequest{
		ZoneID:                     zone,
		InflorescenceBatchID:       batch,
		ObservationPoint:           point,
		VisitTotal:                 6,
		PerInflorescence:           []int64{2, 2, 2},
		BiteMarkInflorescences:     1,
		UnpollinatedInflorescences: 1,
		ReturningBees:              5,
		OutgoingBees:               5,
		DeadBees:                   0,
		SampledInflorescences:      4,
		Weight:                     1000,
		Temperature:                25,
		Humidity:                   60,
		CO2:                        800,
		LogicalTime:                1000,
	}
}

// prepareClosedCycle locks a cycle, fills all coverage cells and submits two
// independent qualified reviews, leaving the cycle in pending_close_review.
func prepareClosedCycle(t *testing.T, svc *Service, cycleID string) {
	t.Helper()
	if _, err := svc.CreateAndLock(validLockRequest(cycleID)); err != nil {
		t.Fatalf("lock: %v", err)
	}
	for _, zone := range []string{"z1", "z2"} {
		batch := "b1"
		if zone == "z2" {
			batch = "b2"
		}
		for _, p := range []int64{1, 2} {
			req := validObservation(zone, batch, p)
			req.OperationID = "obs-" + cycleID + "-" + zone + "-" + itoa(p)
			if _, err := svc.Observe(cycleID, req); err != nil {
				t.Fatalf("observe %s:%d: %v", zone, p, err)
			}
		}
	}
	if _, err := svc.SubmitReview(cycleID, ReviewRequest{ReviewerID: "r1", OperationID: "rev-" + cycleID + "-1", LogicalTime: 2000}); err != nil {
		t.Fatalf("review 1: %v", err)
	}
	if _, err := svc.SubmitReview(cycleID, ReviewRequest{ReviewerID: "r2", OperationID: "rev-" + cycleID + "-2", LogicalTime: 2001}); err != nil {
		t.Fatalf("review 2: %v", err)
	}
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

var _ = coverage.DeviceSuccess
