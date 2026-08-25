package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tomato-bumblebee-pollination-rotation/internal/store"
)

func TestModel_RotateRejectsMismatchedSourceAndPreservesOccupancy(t *testing.T) {
	tests := []struct {
		name       string
		colonyID   string
		fromZoneID string
		toZoneID   string
		admitZone  string
	}{
		{
			name:       "c1 cannot claim its z2 occupancy as the source",
			colonyID:   "c1",
			fromZoneID: "z2",
			toZoneID:   "z1",
			admitZone:  "z2",
		},
		{
			name:       "source occupancy must belong to the rotating colony",
			colonyID:   "c2",
			fromZoneID: "z1",
			toZoneID:   "z2",
			admitZone:  "z1",
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := store.NewMemoryStore()
			handler := NewServer(st, defaultCatalog(), NewScriptDriver()).Handler()
			cycleID := "rotate-source-" + itoa(int64(i+1))

			doPost := func(path string, body any) *httptest.ResponseRecorder {
				t.Helper()
				payload, err := json.Marshal(body)
				if err != nil {
					t.Fatalf("marshal request: %v", err)
				}
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload)))
				return recorder
			}

			locked := doPost("/v1/cycles", validLockRequest(cycleID))
			if locked.Code != http.StatusCreated {
				t.Fatalf("lock status = %d, want %d; body=%s", locked.Code, http.StatusCreated, locked.Body.String())
			}

			rotated := doPost("/v1/cycles/"+cycleID+"/rotate", RotateRequest{
				ColonyID:    tc.colonyID,
				FromZoneID:  tc.fromZoneID,
				ToZoneID:    tc.toZoneID,
				WindowStart: 200,
				WindowEnd:   300,
				OperationID: "rotate-" + cycleID,
				LogicalTime: 1100,
			})
			if rotated.Code != http.StatusConflict {
				t.Errorf("mismatched-source rotate status = %d, want %d; body=%s", rotated.Code, http.StatusConflict, rotated.Body.String())
			} else {
				var apiErr APIError
				if err := json.Unmarshal(rotated.Body.Bytes(), &apiErr); err != nil {
					t.Fatalf("decode rotate error: %v", err)
				}
				if apiErr.Code != CodeInvalidTransition {
					t.Errorf("rotate error code = %q, want %q", apiErr.Code, CodeInvalidTransition)
				}
			}

			admitted := doPost("/v1/cycles/"+cycleID+"/admit", AdmitRequest{
				ColonyID:    tc.colonyID,
				ZoneID:      tc.admitZone,
				WindowStart: 50,
				WindowEnd:   60,
				OperationID: "admit-after-rejected-rotate-" + cycleID,
				LogicalTime: 1200,
			})
			if admitted.Code != http.StatusConflict {
				t.Errorf("overlapping admit status = %d, want %d because original occupancy must remain; body=%s", admitted.Code, http.StatusConflict, admitted.Body.String())
			}
		})
	}
}
