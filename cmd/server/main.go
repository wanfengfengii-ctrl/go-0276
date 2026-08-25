// Command server is the runnable entry point for the tomato bumblebee
// pollination rotation backend. It wires the rule catalog, the relational
// persistence store (with restart recovery) and the deterministic device driver
// behind the HTTP API.
package main

import (
	"log"
	"net/http"
	"os"

	"tomato-bumblebee-pollination-rotation/internal/httpapi"
	"tomato-bumblebee-pollination-rotation/internal/rules"
	"tomato-bumblebee-pollination-rotation/internal/store"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dataPath := os.Getenv("DATA_PATH")
	if dataPath == "" {
		dataPath = "benzhi-data.json"
	}

	catalog := rules.NewMemoryCatalog()
	registerSeedRules(catalog)

	var st store.Store
	var err error
	if dataPath == ":memory:" {
		st = store.NewMemoryStore()
	} else {
		st, err = store.OpenSQLStore(dataPath)
		if err != nil {
			log.Fatalf("open store: %v", err)
		}
	}
	// The store must stay open for the entire lifetime of the process: the
	// recovery scan and every request handler transact against it. Closing it
	// here would make View/Update fail with "database is closed", so HTTP 200
	// health checks would silently report an empty state while every write
	// (POST /v1/cycles) and read (GET /v1/cycles/{id}) returned 500. Defer the
	// close so the handle is released only when the server is shutting down.
	defer st.Close()

	report := recoverReport(st)
	if len(report) > 0 {
		log.Printf("recovery: %s", report)
	}

	driver := httpapi.NewScriptDriver()
	server := httpapi.NewServer(st, catalog, driver)

	log.Printf("listening on %s (data=%s)", addr, dataPath)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// registerSeedRules installs a small deterministic rule catalog so the server is
// usable out of the box. In production this catalog would be loaded from a
// persistent rules database or configuration.
func registerSeedRules(c *rules.MemoryCatalog) {
	g := rules.Greenhouse{ID: "g1", Name: "demo", Version: 1, Zones: []string{"z1", "z2", "z3"}}
	c.AddGreenhouse(g)
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
}

func recoverReport(st store.Store) string {
	var out string
	_ = st.View(func(s *store.State) error {
		rep := store.Recover(s)
		out = formatRecoveryReport(rep)
		return nil
	})
	return out
}

func formatRecoveryReport(r store.RecoveryReport) string {
	if len(r.UnfinishedCycles) == 0 && len(r.PendingRetries) == 0 && len(r.UndecidedCycles) == 0 {
		return ""
	}
	return "unfinished=" + itoaInt(len(r.UnfinishedCycles)) + " pending_retries=" + itoaInt(len(r.PendingRetries)) + " undecided=" + itoaInt(len(r.UndecidedCycles))
}

func itoaInt(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
