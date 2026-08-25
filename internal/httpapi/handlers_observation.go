package httpapi

import (
	"net/http"
)

func (s *Server) handleObservation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req ObservationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	result, err := s.service.Observe(id, req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDeviceRetry(w http.ResponseWriter, r *http.Request) {
	retryKey := r.PathValue("retry_key")
	result, err := s.service.RetryDevice(retryKey)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
