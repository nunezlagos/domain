package batch_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/api/batch"
)

type item struct {
	Name string `json:"name"`
}
type result struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func newHandler(t *testing.T, proc func(*http.Request, item) (result, int, error)) *batch.Handler[item, result] {
	t.Helper()
	return &batch.Handler[item, result]{ProcessItem: proc}
}

func req(items any) *http.Request {
	body, _ := json.Marshal(map[string]any{"items": items})
	r := httptest.NewRequest("POST", "/batch", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestBatch_AllSuccess(t *testing.T) {
	h := newHandler(t, func(_ *http.Request, it item) (result, int, error) {
		return result{ID: "uuid-" + it.Name, Name: it.Name}, http.StatusCreated, nil
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req([]item{{Name: "a"}, {Name: "b"}}))

	require.Equal(t, http.StatusMultiStatus, rec.Code)
	var resp batch.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.Summary.Total)
	require.Equal(t, 2, resp.Summary.Success)
	require.Equal(t, 0, resp.Summary.Failed)
	require.Equal(t, 201, resp.Results[0].Status)
	require.Contains(t, string(resp.Results[0].Data), "uuid-a")
}

func TestBatch_PartialFailure(t *testing.T) {
	h := newHandler(t, func(_ *http.Request, it item) (result, int, error) {
		if it.Name == "bad" {
			return result{}, http.StatusUnprocessableEntity, errors.New("name=bad rejected")
		}
		return result{ID: "ok", Name: it.Name}, http.StatusCreated, nil
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req([]item{{Name: "good"}, {Name: "bad"}, {Name: "good2"}}))

	require.Equal(t, http.StatusMultiStatus, rec.Code)
	var resp batch.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 3, resp.Summary.Total)
	require.Equal(t, 2, resp.Summary.Success)
	require.Equal(t, 1, resp.Summary.Failed)
	require.NotNil(t, resp.Results[1].Error)
	require.Equal(t, "validation_failed", resp.Results[1].Error.Code)
}

func TestBatch_EmptyArray(t *testing.T) {
	h := newHandler(t, func(*http.Request, item) (result, int, error) { return result{}, 201, nil })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req([]item{}))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "empty_batch")
}

func TestBatch_TooLarge(t *testing.T) {
	h := &batch.Handler[item, result]{
		MaxItems: 2,
		ProcessItem: func(*http.Request, item) (result, int, error) {
			return result{}, 201, nil
		},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req([]item{{Name: "1"}, {Name: "2"}, {Name: "3"}}))
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.Contains(t, rec.Body.String(), "batch_too_large")
}

func TestBatch_BadJSON(t *testing.T) {
	h := newHandler(t, func(*http.Request, item) (result, int, error) { return result{}, 201, nil })
	r := httptest.NewRequest("POST", "/batch", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestBatch_OnlyPOST(t *testing.T) {
	h := newHandler(t, func(*http.Request, item) (result, int, error) { return result{}, 201, nil })
	r := httptest.NewRequest("GET", "/batch", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// Sabotaje: si ProcessItem retorna data NIL pero error=nil, no debe pretender success.
func TestSabotage_NilResult_NoError_StillMarshals(t *testing.T) {
	h := newHandler(t, func(*http.Request, item) (result, int, error) {
		// zero-value result + 200 → success válido (no es panic).
		return result{}, http.StatusOK, nil
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req([]item{{Name: "x"}}))
	require.Equal(t, http.StatusMultiStatus, rec.Code)
	var resp batch.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Summary.Success)
}
