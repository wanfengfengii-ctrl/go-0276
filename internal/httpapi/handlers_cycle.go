package httpapi

import (
	"encoding/json"
	"net/http"

	"tomato-bumblebee-pollination-rotation/internal/store"
)

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	var view HealthView
	_ = s.store.View(func(st *store.State) error {
		report := store.Recover(st)
		view = HealthView{
			Status:           "ok",
			Cycles:           len(st.Cycles),
			PendingRetries:   len(report.PendingRetries),
			UnfinishedCycles: len(report.UnfinishedCycles),
			Now:              healthNow(),
		}
		return nil
	})
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleCreateCycle(w http.ResponseWriter, r *http.Request) {
	var req LockRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	if req.OperationID == "" {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "operation_id required"))
		return
	}
	view, err := s.service.CreateAndLock(req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) handleLockCycle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req LockRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	req.CycleID = id
	if req.OperationID == "" {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "operation_id required"))
		return
	}
	view, err := s.service.CreateAndLock(req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) handleGetCycle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	view, err := s.service.GetCycle(id)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleEvidence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	events, err := s.service.Evidence(id)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleAdmit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req AdmitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	view, err := s.service.Admit(id, req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleRotate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req RotateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	view, err := s.service.Rotate(id, req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req WithdrawRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	result, err := s.service.Withdraw(id, req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req CancelRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	result, err := s.service.Cancel(id, req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleComplete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req CompleteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, NewError(CodeInvalidRequest, "invalid request body"))
		return
	}
	result, err := s.service.Complete(id, req)
	if err != nil {
		respondErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}
