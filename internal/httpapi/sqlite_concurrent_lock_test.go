package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"tomato-bumblebee-pollination-rotation/internal/store"
)

func TestModel_SQLiteConcurrentCycleLocksAreSerialized(t *testing.T) {
	cases := []struct {
		name       string
		requestURL func(string) string
	}{
		{
			name: "create cycle endpoint",
			requestURL: func(string) string {
				return "/v1/cycles"
			},
		},
		{
			name: "lock cycle endpoint",
			requestURL: func(cycleID string) string {
				return "/v1/cycles/" + cycleID + "/lock"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const requestCount = 24
			dbPath := filepath.Join(t.TempDir(), "cycles.db")
			st, err := store.OpenSQLStore(dbPath)
			if err != nil {
				t.Fatalf("open SQLite store: %v", err)
			}
			server := NewServer(st, defaultCatalog(), NewScriptDriver())

			type result struct {
				status int
				body   string
			}
			results := make([]result, requestCount)
			cycleIDs := make([]string, requestCount)
			bodies := make([][]byte, requestCount)
			for i := 0; i < requestCount; i++ {
				cycleIDs[i] = fmt.Sprintf("sqlite-cycle-%02d", i)
				body, marshalErr := json.Marshal(validLockRequest(cycleIDs[i]))
				if marshalErr != nil {
					t.Fatalf("marshal lock request %d: %v", i, marshalErr)
				}
				bodies[i] = body
			}

			start := make(chan struct{})
			var ready sync.WaitGroup
			var done sync.WaitGroup
			ready.Add(requestCount)
			done.Add(requestCount)
			for i := 0; i < requestCount; i++ {
				go func(index int) {
					defer done.Done()
					ready.Done()
					<-start
					req := httptest.NewRequest(http.MethodPost, tc.requestURL(cycleIDs[index]), bytes.NewReader(bodies[index]))
					recorder := httptest.NewRecorder()
					server.Handler().ServeHTTP(recorder, req)
					results[index] = result{status: recorder.Code, body: recorder.Body.String()}
				}(i)
			}
			ready.Wait()
			close(start)
			done.Wait()

			for i, got := range results {
				if got.status != http.StatusCreated {
					t.Errorf("lock %q returned status %d, want %d; body=%s", cycleIDs[i], got.status, http.StatusCreated, got.body)
				}
			}
			if t.Failed() {
				_ = st.Close()
				return
			}
			if err := st.Close(); err != nil {
				t.Fatalf("close SQLite store: %v", err)
			}

			reopened, err := store.OpenSQLStore(dbPath)
			if err != nil {
				t.Fatalf("reopen SQLite store: %v", err)
			}
			defer reopened.Close()
			restarted := NewServer(reopened, defaultCatalog(), NewScriptDriver())
			for _, cycleID := range cycleIDs {
				req := httptest.NewRequest(http.MethodGet, "/v1/cycles/"+cycleID, nil)
				recorder := httptest.NewRecorder()
				restarted.Handler().ServeHTTP(recorder, req)
				if recorder.Code != http.StatusOK {
					t.Errorf("persisted cycle %q returned status %d, want %d; body=%s", cycleID, recorder.Code, http.StatusOK, recorder.Body.String())
					continue
				}
				var view CycleView
				if err := json.Unmarshal(recorder.Body.Bytes(), &view); err != nil {
					t.Errorf("decode persisted cycle %q: %v", cycleID, err)
					continue
				}
				if view.CycleID != cycleID || view.State != "pending_admit" || view.CoverageCells != 4 {
					t.Errorf("persisted cycle %q is incomplete: %+v", cycleID, view)
				}
			}
		})
	}
}
