package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// issue-16.1 web dashboard.
// Tests unitarios de handlers HTTP sin DB: routing, auth, content-type,
// y shape del JSON. Los queries a DB (gatherStats, serveRecentRuns)
// requieren testcontainers — fuera de scope de este commit.

func TestCheckAuth_NilAuthCheck_Allows(t *testing.T) {
	// AuthCheck nil → checkAuth retorna true (modo dev / reverse proxy).
	// Documentado en godoc del campo.
	h := &Handler{}
	require.True(t, h.checkAuth(httptest.NewRequest("GET", "/admin/", nil)))
}

func TestCheckAuth_FuncTrue_Allows(t *testing.T) {
	h := &Handler{AuthCheck: func(r *http.Request) bool { return true }}
	require.True(t, h.checkAuth(httptest.NewRequest("GET", "/admin/", nil)))
}

func TestCheckAuth_FuncFalse_Rejects(t *testing.T) {
	h := &Handler{AuthCheck: func(r *http.Request) bool { return false }}
	require.False(t, h.checkAuth(httptest.NewRequest("GET", "/admin/", nil)))
}

func TestServeAdmin_Unauthorized(t *testing.T) {
	h := &Handler{AuthCheck: func(r *http.Request) bool { return false }}
	rec := httptest.NewRecorder()
	h.serveAdmin(rec, httptest.NewRequest("GET", "/admin/", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, "unauthorized\n", rec.Body.String())
}

func TestServeAdmin_DefaultPath_Index(t *testing.T) {
	// /admin o /admin/ → assets/index.html
	h := &Handler{AuthCheck: func(r *http.Request) bool { return true }}
	rec := httptest.NewRecorder()
	h.serveAdmin(rec, httptest.NewRequest("GET", "/admin/", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), "<!DOCTYPE html>",
		"index.html debe ser el dashboard HTML")
}

func TestServeAdmin_AssetNotFound_404(t *testing.T) {
	h := &Handler{AuthCheck: func(r *http.Request) bool { return true }}
	rec := httptest.NewRecorder()
	h.serveAdmin(rec, httptest.NewRequest("GET", "/admin/nope.html", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestServeAdmin_JSAsset_404SetsCorrectStatus(t *testing.T) {
	// Si piden un .js que no existe, devuelve 404 (no asume que existe).
	// El content-type mapping se aplica solo si el archivo se lee OK.
	h := &Handler{AuthCheck: func(r *http.Request) bool { return true }}
	rec := httptest.NewRecorder()
	h.serveAdmin(rec, httptest.NewRequest("GET", "/admin/nope.js", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestServeAdmin_XContentTypeOptions(t *testing.T) {
	// Todos los assets servidos deben llevar X-Content-Type-Options: nosniff
	// (defense contra MIME sniffing attacks).
	h := &Handler{AuthCheck: func(r *http.Request) bool { return true }}
	rec := httptest.NewRecorder()
	h.serveAdmin(rec, httptest.NewRequest("GET", "/admin/", nil))
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}

func TestServeStats_Unauthorized(t *testing.T) {
	h := &Handler{AuthCheck: func(r *http.Request) bool { return false }}
	rec := httptest.NewRecorder()
	h.serveStats(rec, httptest.NewRequest("GET", "/admin/api/stats", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestServeStats_PoolNil_Panics(t *testing.T) {
	// GAP CONOCIDO: gatherStats NO valida que h.Pool != nil antes de
	// ejecutar QueryRow. Si el handler se monta con Pool=nil, el primer
	// QueryRow hace panic con "nil pointer dereference".
	//
	// Por diseño deberia: validar Pool != nil al construir el Handler
	// (NewHandler retorna error), o servir un Stats vacio con todos
	// los counts en -1 y status 200 (consistente con el comentario del
	// codigo: "tabla puede no existir en ambiente parcial").
	//
	// Test documenta el bug: si en el futuro se corrige (e.g., guard
	// en gatherStats), este test debe actualizarse.
	h := &Handler{AuthCheck: func(r *http.Request) bool { return true }}
	require.Panics(t, func() {
		rec := httptest.NewRecorder()
		h.serveStats(rec, httptest.NewRequest("GET", "/admin/api/stats", nil))
		_ = rec
	}, "Pool=nil causa panic — gap conocido, fix futuro: guard en gatherStats")
}

func TestServeStats_ResponseShape(t *testing.T) {
	// Validamos el shape del struct Stats via JSON roundtrip.
	// No testeamos serveStats directo porque requiere Pool real o mock;
	// en su lugar validamos el shape Marshal/Unmarshal del struct.
	s := Stats{Orgs: 5, Projects: 10, Users: 20, Observations: 100, AgentRunsToday: 3}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	expectedFields := []string{
		"orgs", "projects", "users", "observations", "knowledge_docs",
		"agents", "flows", "skills", "agent_runs_today", "flow_runs_today",
	}
	for _, f := range expectedFields {
		require.Contains(t, got, f, "Stats JSON debe tener campo %q", f)
	}
}

func TestServeRecentRuns_Unauthorized(t *testing.T) {
	h := &Handler{AuthCheck: func(r *http.Request) bool { return false }}
	rec := httptest.NewRecorder()
	h.serveRecentRuns(rec, httptest.NewRequest("GET", "/admin/api/recent-runs", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWriteJSON_ContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, map[string]int{"a": 1})
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), `"a":1`)
}

func TestRegister_MountsRoutes_StaticAnd404(t *testing.T) {
	// Verifica que Register monta las rutas que NO requieren DB.
	// /admin/api/stats y /admin/api/recent-runs requieren Pool (panic si nil),
	// testeados por separado en TestServeStats_PoolNil_Panics.
	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	cases := []struct {
		path string
		want int
	}{
		{"/admin/", 200},                // index.html embed existe
		{"/admin/unknown", 404},         // asset no encontrado
		{"/unknown", 404},               // ruta no montada
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
			require.Equal(t, tc.want, rec.Code,
				"path %s debe responder %d, dio %d", tc.path, tc.want, rec.Code)
		})
	}
}

func TestRecentRun_StructShape(t *testing.T) {
	// RecentRun: validamos JSON tags.
	rr := RecentRun{
		Type:      "agent",
		ID:        "abc-123",
		Status:    "done",
		StartedAt: time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC),
	}
	data, err := json.Marshal(rr)
	require.NoError(t, err)
	s := string(data)
	require.Contains(t, s, `"type":"agent"`)
	require.Contains(t, s, `"id":"abc-123"`)
	require.Contains(t, s, `"status":"done"`)
	require.NotContains(t, s, `"duration"`, "Duration vacio se omite (omitempty)")
}

func TestStats_StructShape(t *testing.T) {
	// Stats: roundtrip JSON preserva campos.
	s := Stats{Orgs: 5, Projects: 10, Users: 20, Observations: 100, AgentRunsToday: 3}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	var decoded Stats
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, s.Orgs, decoded.Orgs)
	require.Equal(t, s.AgentRunsToday, decoded.AgentRunsToday)
}

func TestServeAdmin_PathTraversalBlocked(t *testing.T) {
	// Sabotaje: /admin/../../etc/passwd no debe funcionar.
	// El handler hace strings.TrimPrefix("/admin") → "../../etc/passwd"
	// → assets.ReadFile falla (path traversal bloqueado por embed.FS).
	h := &Handler{AuthCheck: func(r *http.Request) bool { return true }}
	rec := httptest.NewRecorder()
	// http.ServeMux normaliza el path antes de llegar al handler, pero
	// probamos con un path directo al handler.
	h.serveAdmin(rec, httptest.NewRequest("GET", "/admin/../../etc/passwd", nil))
	// No debe leer /etc/passwd. Si llega a leer algo, deberia ser index.html
	// (porque embed.FS normaliza). Acceptable: 200 con index o 404.
	// Lo importante: NO debe devolver contenido de /etc/passwd.
	body := rec.Body.String()
	require.NotContains(t, body, "root:", "path traversal no debe filtrar /etc/passwd")
}
