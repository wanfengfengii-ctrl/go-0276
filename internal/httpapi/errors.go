// Package httpapi exposes the JSON HTTP API: stable error codes, operation
// idempotency semantics and deterministic error ordering, composing domain
// services and persistent transactions behind a single router.
package httpapi

import (
	"sort"
	"strings"
)

// Stable error codes referenced across the acceptance criteria.
const (
	CodeOperationContentConflict = "OPERATION_CONTENT_CONFLICT"
	CodeObservationDuplicate     = "OBSERVATION_DUPLICATE_CONFLICT"
	CodeTerminalAlreadyDecided   = "TERMINAL_ALREADY_DECIDED"
	CodeInvalidTransition        = "INVALID_STATE_TRANSITION"
	CodeNotFound                 = "NOT_FOUND"
	CodeInvalidRequest           = "INVALID_REQUEST"
	CodeInternalError            = "INTERNAL_ERROR"
)

// Reason is one sorted reason attached to an API error.
type Reason struct {
	Greenhouse       string `json:"greenhouse,omitempty"`
	Zone             string `json:"zone,omitempty"`
	WindowStart      int64  `json:"window_start,omitempty"`
	WindowEnd        int64  `json:"window_end,omitempty"`
	Inflorescence    string `json:"inflorescence,omitempty"`
	ObservationPoint int64  `json:"observation_point,omitempty"`
	Code             string `json:"code,omitempty"`
}

// APIError is the uniform error envelope {code, message, reasons[]}.
type APIError struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Reasons []Reason `json:"reasons"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return e.Code + ": " + e.Message
}

// SortReasons orders reasons deterministically by greenhouse, zone, window
// start, window end, inflorescence, observation point and code.
func SortReasons(reasons []Reason) {
	sort.SliceStable(reasons, func(i, j int) bool {
		return reasonLess(reasons[i], reasons[j])
	})
}

func reasonLess(a, b Reason) bool {
	if a.Greenhouse != b.Greenhouse {
		return a.Greenhouse < b.Greenhouse
	}
	if a.Zone != b.Zone {
		return a.Zone < b.Zone
	}
	if a.WindowStart != b.WindowStart {
		return a.WindowStart < b.WindowStart
	}
	if a.WindowEnd != b.WindowEnd {
		return a.WindowEnd < b.WindowEnd
	}
	if a.Inflorescence != b.Inflorescence {
		return a.Inflorescence < b.Inflorescence
	}
	if a.ObservationPoint != b.ObservationPoint {
		return a.ObservationPoint < b.ObservationPoint
	}
	return strings.Compare(a.Code, b.Code) < 0
}

// NewError builds an APIError and normalizes its reason ordering.
func NewError(code, message string, reasons ...Reason) *APIError {
	SortReasons(reasons)
	return &APIError{Code: code, Message: message, Reasons: reasons}
}
