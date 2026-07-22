package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Taier05/InfraView/internal/service"
)

type ResponseMeta struct {
	RequestID   string    `json:"request_id"`
	Stale       bool      `json:"stale"`
	CollectedAt time.Time `json:"collected_at,omitempty"`
}

func (meta ResponseMeta) MarshalJSON() ([]byte, error) {
	type responseMeta struct {
		RequestID   string     `json:"request_id"`
		Stale       bool       `json:"stale"`
		CollectedAt *time.Time `json:"collected_at,omitempty"`
	}
	encoded := responseMeta{RequestID: meta.RequestID, Stale: meta.Stale}
	if !meta.CollectedAt.IsZero() {
		collectedAt := meta.CollectedAt
		encoded.CollectedAt = &collectedAt
	}
	return json.Marshal(encoded)
}

type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Retryable bool   `json:"retryable"`
	Stale     bool   `json:"stale"`
}

type successEnvelope struct {
	Data any          `json:"data"`
	Meta ResponseMeta `json:"meta"`
}

func writeSuccess(w http.ResponseWriter, r *http.Request, data any, meta service.Meta) {
	if state := requestStateFrom(r.Context()); state != nil {
		state.stale = meta.Stale
	}
	writeJSON(w, http.StatusOK, successEnvelope{
		Data: data,
		Meta: ResponseMeta{
			RequestID:   requestIDFrom(r.Context()),
			Stale:       meta.Stale,
			CollectedAt: meta.CollectedAt,
		},
	})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, retryable bool) {
	state := requestStateFrom(r.Context())
	writeJSON(w, status, ErrorBody{
		Code:      code,
		Message:   message,
		RequestID: requestIDFrom(r.Context()),
		Retryable: retryable,
		Stale:     state != nil && state.stale,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
