

package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHealth_ReturnsOKWithVersion(t *testing.T) {
	h := &HealthHandler{
		Info:      VersionInfo{Version: "v1.2.3", Commit: "abc1234", BuildTime: "2026-06-07T00:00:00Z"},
		StartedAt: time.Now().Add(-5 * time.Second),
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, "ok", body["status"])
	require.Equal(t, "v1.2.3", body["version"])
	require.Equal(t, "abc1234", body["commit"])
	require.Equal(t, "2026-06-07T00:00:00Z", body["built"])
	require.Contains(t, body["uptime"], "s")
}

func TestReady_NoPool_ReturnsOK(t *testing.T) {
	h := &ReadyHandler{Pool: nil}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.True(t, body["ready"].(bool))
	require.Equal(t, "skipped", body["db"])
}

func TestReady_ShuttingDown_Returns503(t *testing.T) {
	ShuttingDown.Store(true)
	defer ShuttingDown.Store(false)
	h := &ReadyHandler{Pool: nil}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.False(t, body["ready"].(bool))
	require.Equal(t, "shutting_down", body["reason"])
}

// Sabotaje: response shape verifica que el Content-Type sea correcto y JSON parseable.
func TestSabotage_Health_AlwaysValidJSON(t *testing.T) {
	h := &HealthHandler{Info: VersionInfo{Version: "x"}, StartedAt: time.Now()}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	var out map[string]any
	err := json.NewDecoder(rec.Body).Decode(&out)
	require.NoError(t, err, "/health response must be parseable JSON")
}
