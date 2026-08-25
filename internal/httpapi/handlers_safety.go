package httpapi

import (
	"net/http"
)

func (s *Server) handlePesticideEvent(w http.ResponseWriter, r *http.Request) {
	var req PesticideEventRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	result, err := s.service.RegisterPesticideEvent(req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleRecoveryCheck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req RecoveryCheckRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	result, err := s.service.RecoveryCheck(id, req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSupplement(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req CompleteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	result, err := s.service.ComputeSupplement(id, req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleReview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req ReviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	result, err := s.service.SubmitReview(id, req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
