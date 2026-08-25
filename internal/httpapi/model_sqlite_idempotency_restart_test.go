package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"tomato-bumblebee-pollination-rotation/internal/store"
)

func TestModel_SQLiteCreateCycleIdempotencySurvivesRestart(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*LockRequest)
		wantStatus int
		wantCode   string
		wantReplay bool
	}{
		{
			name:       "identical request replays the original response",
			mutate:     func(*LockRequest) {},
			wantStatus: http.StatusCreated,
			wantReplay: true,
		},
		{
			name: "changed content conflicts",
			mutate: func(req *LockRequest) {
				req.Budget++
			},
			wantStatus: http.StatusConflict,
			wantCode:   CodeOperationContentConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cycles.db")
			firstStore, err := store.OpenSQLStore(path)
			if err != nil {
				t.Fatalf("open initial SQLite store: %v", err)
			}

			original := validLockRequest("restart-cycle")
			body, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("marshal initial request: %v", err)
			}
			first := httptest.NewRecorder()
			NewServer(firstStore, defaultCatalog(), nil).Handler().ServeHTTP(
				first,
				httptest.NewRequest(http.MethodPost, "/v1/cycles", bytes.NewReader(body)),
			)
			if first.Code != http.StatusCreated {
				t.Fatalf("initial POST status = %d, want %d; body=%s", first.Code, http.StatusCreated, first.Body.String())
			}
			firstResponse := append([]byte(nil), first.Body.Bytes()...)
			if err := firstStore.Close(); err != nil {
				t.Fatalf("close initial SQLite store: %v", err)
			}

			reopened, err := store.OpenSQLStore(path)
			if err != nil {
				t.Fatalf("reopen SQLite store: %v", err)
			}
			defer reopened.Close()

			if err := reopened.View(func(state *store.State) error {
				if _, ok := state.Operations[original.OperationID]; !ok {
					t.Error("operation record was not persisted with the create transaction")
				}
				if _, ok := state.Cycles[original.CycleID]; !ok {
					t.Error("cycle was not persisted")
				}
				if len(state.Evidence[original.CycleID]) != 1 {
					t.Errorf("persisted evidence count = %d, want 1", len(state.Evidence[original.CycleID]))
				}
				if _, ok := state.Budgets[original.CycleID+":budget"]; !ok {
					t.Error("budget was not persisted")
				}
				if len(state.CoverageCells) != 4 {
					t.Errorf("persisted coverage cell count = %d, want 4", len(state.CoverageCells))
				}
				return nil
			}); err != nil {
				t.Fatalf("inspect reopened state: %v", err)
			}

			retry := original
			tt.mutate(&retry)
			retryBody, err := json.Marshal(retry)
			if err != nil {
				t.Fatalf("marshal retried request: %v", err)
			}
			got := httptest.NewRecorder()
			NewServer(reopened, defaultCatalog(), nil).Handler().ServeHTTP(
				got,
				httptest.NewRequest(http.MethodPost, "/v1/cycles", bytes.NewReader(retryBody)),
			)

			if got.Code != tt.wantStatus {
				t.Fatalf("retried POST status = %d, want %d; body=%s", got.Code, tt.wantStatus, got.Body.String())
			}
			if tt.wantReplay {
				if !bytes.Equal(got.Body.Bytes(), firstResponse) {
					t.Fatalf("replayed response = %s, want original response %s", got.Body.Bytes(), firstResponse)
				}
				return
			}
			var apiErr APIError
			if err := json.Unmarshal(got.Body.Bytes(), &apiErr); err != nil {
				t.Fatalf("decode conflict response: %v", err)
			}
			if apiErr.Code != tt.wantCode {
				t.Fatalf("retried POST error code = %q, want %q", apiErr.Code, tt.wantCode)
			}
		})
	}
}
