// Package batch — HU-13.5 endpoints batch genéricos.
//
// Provee handler wrapper que recibe array de items, valida cada uno
// independientemente, y devuelve resultado por item con 207 Multi-Status.
// Diseñado para envolver cualquier service de mutación item-at-a-time
// sin duplicar lógica de batching en cada handler.
//
// Pattern client → server:
//
//	POST /api/v1/observations/batch
//	{ "items": [ {...}, {...}, ... ] }
//
//	HTTP 207 Multi-Status
//	{
//	  "results": [
//	    {"index":0, "status":201, "data":{...}},
//	    {"index":1, "status":422, "error":{"code":"validation_failed", ...}}
//	  ],
//	  "summary": {"total":N, "success":S, "failed":F}
//	}
package batch

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// MaxItemsPerBatch es el cap duro para evitar payloads abusivos.
// Cada feature puede pasar un límite menor en Config.
const MaxItemsPerBatch = 5000

// ItemResult es el resultado de procesar un ítem del batch.
type ItemResult struct {
	Index  int             `json:"index"`
	Status int             `json:"status"`
	Data   json.RawMessage `json:"data,omitempty"`
	Error  *ErrorBody      `json:"error,omitempty"`
}

// ErrorBody refleja shape estándar de errores (REQ-13 response shape).
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Summary agrega counters por status group.
type Summary struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

// Response es el shape devuelto al cliente.
type Response struct {
	Results []ItemResult `json:"results"`
	Summary Summary      `json:"summary"`
}

// Request es el shape de entrada esperado.
type Request[T any] struct {
	Items []T `json:"items"`
}

// Handler[T,R] envuelve un processItem y devuelve un http.Handler.
// T = tipo del item de entrada. R = tipo del result data exitoso.
type Handler[T, R any] struct {
	MaxItems    int                                          // override sobre MaxItemsPerBatch (opcional)
	ProcessItem func(r *http.Request, item T) (R, int, error) // 201, 200, etc + error si falla
}

// ServeHTTP implementa http.Handler.
func (h *Handler[T, R]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	max := h.MaxItems
	if max <= 0 || max > MaxItemsPerBatch {
		max = MaxItemsPerBatch
	}

	var req Request[T]
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if len(req.Items) == 0 {
		writeErr(w, http.StatusBadRequest, "empty_batch", "items array required, min 1")
		return
	}
	if len(req.Items) > max {
		writeErr(w, http.StatusRequestEntityTooLarge, "batch_too_large",
			fmt.Sprintf("max %d items per batch, received %d", max, len(req.Items)))
		return
	}

	results := make([]ItemResult, len(req.Items))
	successCount := 0
	failCount := 0

	for i, item := range req.Items {
		data, status, err := h.ProcessItem(r, item)
		res := ItemResult{Index: i, Status: status}
		if err != nil {
			res.Error = &ErrorBody{Code: errCode(status), Message: err.Error()}
			failCount++
		} else {
			body, mErr := json.Marshal(data)
			if mErr != nil {
				res.Status = http.StatusInternalServerError
				res.Error = &ErrorBody{Code: "marshal_failed", Message: mErr.Error()}
				failCount++
			} else {
				res.Data = body
				successCount++
			}
		}
		results[i] = res
	}

	w.WriteHeader(http.StatusMultiStatus)
	_ = json.NewEncoder(w).Encode(Response{
		Results: results,
		Summary: Summary{Total: len(results), Success: successCount, Failed: failCount},
	})
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": ErrorBody{Code: code, Message: msg},
	})
}

func errCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnprocessableEntity:
		return "validation_failed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusTooManyRequests:
		return "rate_limited"
	default:
		if status >= 500 {
			return "internal_error"
		}
		return "error"
	}
}
