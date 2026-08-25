package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"tomato-bumblebee-pollination-rotation/internal/cycle"
	"tomato-bumblebee-pollination-rotation/internal/store"
)

func TestModel_ServerKeepsStoreAvailableWhileListening(t *testing.T) {
	testDir := t.TempDir()
	serverBinary := filepath.Join(testDir, "pollination-server")
	build := exec.Command("go", "build", "-o", serverBinary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build server: %v\n%s", err, output)
	}

	cases := []struct {
		name             string
		dataPath         func(string) string
		persistedCycleID string
	}{
		{
			name:     "memory_store_remains_usable",
			dataPath: func(string) string { return ":memory:" },
		},
		{
			name:             "file_store_remains_usable_and_serves_existing_cycle",
			dataPath:         func(dir string) string { return filepath.Join(dir, "cycles.db") },
			persistedCycleID: "cycle-from-disk",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caseDir := filepath.Join(testDir, tc.name)
			if err := os.Mkdir(caseDir, 0o755); err != nil {
				t.Fatal(err)
			}
			dataPath := tc.dataPath(caseDir)
			if tc.persistedCycleID != "" {
				db, err := store.OpenSQLStore(dataPath)
				if err != nil {
					t.Fatalf("prepare file store: %v", err)
				}
				err = db.Update(func(state *store.State) error {
					state.Cycles[tc.persistedCycleID] = cycle.PollinationCycle{
						ID:    tc.persistedCycleID,
						State: cycle.StateInFlight,
					}
					return nil
				})
				if err != nil {
					_ = db.Close()
					t.Fatalf("seed file store: %v", err)
				}
				if err := db.Close(); err != nil {
					t.Fatalf("close seeded file store: %v", err)
				}
			}

			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("reserve address: %v", err)
			}
			addr := listener.Addr().String()
			if err := listener.Close(); err != nil {
				t.Fatalf("release address: %v", err)
			}

			var logs bytes.Buffer
			cmd := exec.Command(serverBinary)
			cmd.Env = append(os.Environ(), "ADDR="+addr, "DATA_PATH="+dataPath)
			cmd.Stdout = &logs
			cmd.Stderr = &logs
			if err := cmd.Start(); err != nil {
				t.Fatalf("start server: %v", err)
			}
			defer func() {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}()

			baseURL := "http://" + addr
			client := &http.Client{Timeout: 500 * time.Millisecond}
			deadline := time.Now().Add(10 * time.Second)
			for {
				resp, requestErr := client.Get(baseURL + "/healthz")
				if requestErr == nil {
					_ = resp.Body.Close()
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("server did not start: %v\n%s", requestErr, logs.String())
				}
				time.Sleep(20 * time.Millisecond)
			}

			getJSON := func(path string, wantStatus int, target any) {
				t.Helper()
				resp, err := client.Get(baseURL + path)
				if err != nil {
					t.Fatalf("GET %s: %v", path, err)
				}
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("read GET %s: %v", path, err)
				}
				if resp.StatusCode != wantStatus {
					t.Fatalf("GET %s status = %d, want %d; body=%s", path, resp.StatusCode, wantStatus, body)
				}
				if err := json.Unmarshal(body, target); err != nil {
					t.Fatalf("decode GET %s: %v; body=%s", path, err, body)
				}
			}

			var health struct {
				Status string `json:"status"`
				Cycles int    `json:"cycles"`
			}
			getJSON("/healthz", http.StatusOK, &health)
			if health.Status != "ok" {
				t.Fatalf("health status = %q, want ok (store was not usable)", health.Status)
			}
			wantCycles := 0
			if tc.persistedCycleID != "" {
				wantCycles = 1
				var persisted struct {
					CycleID string `json:"cycle_id"`
					State   string `json:"state"`
				}
				getJSON("/v1/cycles/"+tc.persistedCycleID, http.StatusOK, &persisted)
				if persisted.CycleID != tc.persistedCycleID || persisted.State != cycle.StateInFlight.String() {
					t.Fatalf("persisted cycle = %+v", persisted)
				}
			}
			if health.Cycles != wantCycles {
				t.Fatalf("health cycles = %d, want %d", health.Cycles, wantCycles)
			}

			newCycleID := "created-" + tc.name
			payload := fmt.Sprintf(`{
				"cycle_id":%q,"greenhouse_id":"g1","rule_version":1,
				"colony_ids":["c1","c2"],"batch_ids":["b1","b2"],"reviewer_ids":["r1","r2"],
				"observation_points":[1,2],"budget":10,
				"windows":[{"colony_id":"c1","zone_id":"z1","start":0,"end":100},{"colony_id":"c2","zone_id":"z2","start":0,"end":100}],
				"env_thresholds":{"temp_low":0,"temp_high":1000,"hum_low":0,"hum_high":1000,"co2_low":0,"co2_high":5000},
				"retry_spec":{"max_attempts":3,"base_delay":1,"step_delay":2},
				"drift_layers":1,"reentry_hours":24,"operation_id":%q,"logical_time":1000
			}`, newCycleID, "op-"+newCycleID)
			resp, err := client.Post(baseURL+"/v1/cycles", "application/json", bytes.NewBufferString(payload))
			if err != nil {
				t.Fatalf("POST /v1/cycles: %v", err)
			}
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				t.Fatalf("read POST /v1/cycles: %v", readErr)
			}
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("POST /v1/cycles status = %d, want %d; body=%s", resp.StatusCode, http.StatusCreated, body)
			}

			var created struct {
				CycleID string `json:"cycle_id"`
			}
			getJSON("/v1/cycles/"+newCycleID, http.StatusOK, &created)
			if created.CycleID != newCycleID {
				t.Fatalf("created cycle id = %q, want %q", created.CycleID, newCycleID)
			}
		})
	}
}
