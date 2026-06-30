package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// -regen: regenera los snapshots reales (ver TestRegenSnapshots_Manual).
var regenFlag = flag.Bool("regen", false, "regenera snapshots del API real")

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

const fakeAPI = `package handler
func routes() {
	mux.HandleFunc("GET /api/v1/agent-runs/{id}", a.getRun)
	mux.HandleFunc("POST /api/v1/flows", a.createFlow)
	mux.HandleFunc("POST /api/v1/bad_snake/route", a.badSnake)
}
`

const fakeHandlers = `package handler

import "net/http"

func (a *API) createFlow(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusCreated, nil)
}

func (a *API) getRun(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "")
}

func (a *API) badSnake(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusOK, nil)
}
`

const fakeHandlersNo201 = `package handler

import "net/http"

func (a *API) createFlow(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusOK, nil)
}
`

func setupFake(t *testing.T, handlers string) (dir, routesFile, snapDir string) {
	t.Helper()
	root := t.TempDir()
	dir = filepath.Join(root, "handler")
	routesFile = filepath.Join(dir, "api.go")
	snapDir = filepath.Join(root, "snapshots")
	writeFile(t, routesFile, fakeAPI)
	writeFile(t, filepath.Join(dir, "handlers.go"), handlers)
	return dir, routesFile, snapDir
}

// test-003: URL snake_case → fail.
func TestRoutes_SnakeCaseURL_Fails(t *testing.T) {
	dir, routesFile, snapDir := setupFake(t, fakeHandlers)
	violations, err := runShapeChecks(dir, routesFile, snapDir, true)
	require.NoError(t, err)
	require.Len(t, violations, 1)
	require.Contains(t, violations[0].Reason, "kebab-case")
	require.Contains(t, violations[0].Reason, "bad_snake")
}

// test-002: POST create sin 201 → fail.
func TestRoutes_PostCreateWithout201_Fails(t *testing.T) {
	dir, routesFile, snapDir := setupFake(t, fakeHandlersNo201)
	violations, err := runShapeChecks(dir, routesFile, snapDir, true)
	require.NoError(t, err)
	var found bool
	for _, v := range violations {
		if v.Handler == "createFlow" {
			require.Contains(t, v.Reason, "StatusCreated")
			found = true
		}
	}
	require.True(t, found, "create handler sin 201 debe fallar")
}

// test-004: snapshot drift sin update → fail.
func TestSnapshot_DriftWithoutUpdate_Fails(t *testing.T) {
	dir, routesFile, snapDir := setupFake(t, fakeHandlers)


	_, err := runShapeChecks(dir, routesFile, snapDir, true)
	require.NoError(t, err)


	violations, err := runShapeChecks(dir, routesFile, snapDir, false)
	require.NoError(t, err)
	for _, v := range violations {
		require.NotContains(t, v.Reason, "snapshot", "sin drift no debe fallar snapshot")
	}


	writeFile(t, routesFile, fakeAPI+`
func more() { mux.HandleFunc("DELETE /api/v1/flows/{id}", a.deleteFlow) }
`)
	violations, err = runShapeChecks(dir, routesFile, snapDir, false)
	require.NoError(t, err)
	var drift bool
	for _, v := range violations {
		if v.File == filepath.Join(snapDir, "endpoint_shapes.json") {
			require.Contains(t, v.Reason, "drift")
			drift = true
		}
	}
	require.True(t, drift, "cambio de rutas sin -update debe reportar drift")
}

// test-005: update mode regenera.
func TestSnapshot_UpdateRegenerates(t *testing.T) {
	dir, routesFile, snapDir := setupFake(t, fakeHandlers)
	_, err := runShapeChecks(dir, routesFile, snapDir, true)
	require.NoError(t, err)

	writeFile(t, routesFile, fakeAPI+`
func more() { mux.HandleFunc("DELETE /api/v1/flows/{id}", a.deleteFlow) }
`)
	_, err = runShapeChecks(dir, routesFile, snapDir, true)
	require.NoError(t, err)


	violations, err := runShapeChecks(dir, routesFile, snapDir, false)
	require.NoError(t, err)
	for _, v := range violations {
		require.NotContains(t, v.Reason, "drift")
	}
}

// Snapshot missing → mensaje accionable.
func TestSnapshot_Missing_Reports(t *testing.T) {
	dir, routesFile, snapDir := setupFake(t, fakeHandlers)
	violations, err := runShapeChecks(dir, routesFile, snapDir, false)
	require.NoError(t, err)
	var missing int
	for _, v := range violations {
		if filepath.Dir(v.File) == snapDir {
			require.Contains(t, v.Reason, "-update")
			missing++
		}
	}
	require.Equal(t, 2, missing, "ambos snapshots faltantes deben reportarse")
}

// Bootstrap + verificación de los snapshots REALES del API.
// Primera corrida los genera; corridas siguientes detectan drift.
func TestRealAPI_SnapshotsUpToDate(t *testing.T) {
	root := repoRoot(t)
	handlerDir := filepath.Join(root, "internal", "api", "handler")
	routesFile := filepath.Join(handlerDir, "api.go")
	snapDir := filepath.Join(handlerDir, "testdata", "api")

	if _, err := os.Stat(filepath.Join(snapDir, "endpoint_shapes.json")); os.IsNotExist(err) {
		_, err := runShapeChecks(handlerDir, routesFile, snapDir, true)
		require.NoError(t, err)
		t.Log("snapshots generados por primera vez en", snapDir)
		return
	}

	violations, err := runShapeChecks(handlerDir, routesFile, snapDir, false)
	require.NoError(t, err)
	for _, v := range violations {
		t.Errorf("%s", v.String())
	}
}

// TestRegenSnapshots_Manual regenera los snapshots reales. Equivalente a
// `make api-snapshot-update`. Correr explícito:
//
//	go test -run TestRegenSnapshots_Manual ./cmd/response-shape-lint -args -regen
func TestRegenSnapshots_Manual(t *testing.T) {
	if !*regenFlag && os.Getenv("REGEN_SNAPSHOTS") != "1" {
		t.Skip("pasar -args -regen (o REGEN_SNAPSHOTS=1) para regenerar")
	}
	root := repoRoot(t)
	handlerDir := filepath.Join(root, "internal", "api", "handler")
	_, err := runShapeChecks(handlerDir, filepath.Join(handlerDir, "api.go"),
		filepath.Join(handlerDir, "testdata", "api"), true)
	require.NoError(t, err)
}

// El código real de handlers debe pasar el linter de shapes (CI lo bloquea).
func TestRealAPI_HandlersUseCanonicalWriters(t *testing.T) {
	root := repoRoot(t)
	violations, scanned, err := lintDir(filepath.Join(root, "internal", "api", "handler"))
	require.NoError(t, err)
	require.Greater(t, scanned, 0)
	for _, v := range violations {
		t.Errorf("%s", v.String())
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
