package httpapi

import (
	"encoding/json"
	"net/http"

	"tomato-bumblebee-pollination-rotation/internal/rules"
	"tomato-bumblebee-pollination-rotation/internal/store"
)

// Server wires domain services and the persistence store behind the HTTP API.
type Server struct {
	store   store.Store
	service *Service
	mux     *http.ServeMux
}

// NewServer constructs the router with all versioned endpoints registered. The
// catalog supplies the rule source of truth and the driver supplies the
// deterministic device adapters.
func NewServer(st store.Store, catalog rules.Catalog, driver DeviceDriver) *Server {
	s := &Server{
		store:   st,
		service: NewService(st, catalog, driver),
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

// Handler returns the underlying http.Handler for use by an HTTP server.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("POST /v1/cycles", s.handleCreateCycle)
	s.mux.HandleFunc("POST /v1/cycles/{id}/lock", s.handleLockCycle)
	s.mux.HandleFunc("GET /v1/cycles/{id}", s.handleGetCycle)
	s.mux.HandleFunc("GET /v1/cycles/{id}/evidence", s.handleEvidence)
	s.mux.HandleFunc("POST /v1/cycles/{id}/admit", s.handleAdmit)
	s.mux.HandleFunc("POST /v1/cycles/{id}/rotate", s.handleRotate)
	s.mux.HandleFunc("POST /v1/cycles/{id}/withdraw", s.handleWithdraw)
	s.mux.HandleFunc("POST /v1/cycles/{id}/cancel", s.handleCancel)
	s.mux.HandleFunc("POST /v1/cycles/{id}/complete", s.handleComplete)
	s.mux.HandleFunc("POST /v1/cycles/{id}/observations", s.handleObservation)
	s.mux.HandleFunc("POST /v1/cycles/{id}/device-retries/{retry_key}", s.handleDeviceRetry)
	s.mux.HandleFunc("POST /v1/pesticide-events", s.handlePesticideEvent)
	s.mux.HandleFunc("POST /v1/cycles/{id}/recovery-checks", s.handleRecoveryCheck)
	s.mux.HandleFunc("POST /v1/cycles/{id}/supplements", s.handleSupplement)
	s.mux.HandleFunc("POST /v1/cycles/{id}/reviews", s.handleReview)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, e *APIError) {
	writeJSON(w, status, e)
}

// respondErr maps a stable API error to an HTTP status and writes it.
func respondErr(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*APIError); ok {
		writeError(w, statusForCode(apiErr.Code), apiErr)
		return
	}
	writeError(w, http.StatusInternalServerError, NewError(CodeInternalError, err.Error()))
}

func statusForCode(code string) int {
	switch code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeInvalidRequest:
		return http.StatusBadRequest
	case CodeInvalidTransition, CodeOperationContentConflict, CodeObservationDuplicate, CodeTerminalAlreadyDecided:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
