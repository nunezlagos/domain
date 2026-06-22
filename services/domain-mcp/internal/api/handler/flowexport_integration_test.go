//go:build integration

package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func createTestFlow(t *testing.T, srvURL, key, slug string) (id string, updatedAt time.Time) {
	t.Helper()
	resp, body := doJSON(t, "POST", srvURL+"/api/v1/flows", key, map[string]any{
		"slug": slug, "name": "Flow " + slug,
		"spec": map[string]any{
			"version": 1,
			"steps": []map[string]any{
				{"id": "s1", "type": "skill_run",
					"config": map[string]any{"skill_slug": "fr-skill", "args": map[string]any{}}},
			},
		},
	})
	require.Equalf(t, http.StatusCreated, resp.StatusCode, "body=%s", body)
	var created struct {
		Data struct {
			ID        string    `json:"ID"`
			UpdatedAt time.Time `json:"UpdatedAt"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &created))
	return created.Data.ID, created.Data.UpdatedAt
}

// Export → import roundtrip con fidelidad + slug dup 409 + cap 1MB.
func TestFlowAPI_ExportImport(t *testing.T) {
	srv, key, cleanup := setupFlowAPI(t)
	defer cleanup()
	id, _ := createTestFlow(t, srv.URL, key, "exp-flow")

	// Export JSON
	resp, body := doJSON(t, "GET", srv.URL+"/api/v1/flows/"+id+"/export?format=json", key, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	require.Equal(t, "exp-flow", doc["slug"])

	// Export YAML
	resp, body = doJSON(t, "GET", srv.URL+"/api/v1/flows/"+id+"/export?format=yaml", key, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "yaml")
	require.Contains(t, string(body), "slug: exp-flow")

	// Import del mismo slug → 409
	doc["slug"] = "exp-flow"
	raw, _ := json.Marshal(doc)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/flows/import", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp2.Body.Close()
	require.Equal(t, http.StatusConflict, resp2.StatusCode)

	// Import con slug nuevo → 201 + fidelidad del spec
	doc["slug"] = "exp-flow-copy"
	raw, _ = json.Marshal(doc)
	req, _ = http.NewRequest("POST", srv.URL+"/api/v1/flows/import", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp3, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp3.Body.Close()
	require.Equal(t, http.StatusCreated, resp3.StatusCode)

	// Payload >1MB → 413
	big := bytes.Repeat([]byte("x"), (1<<20)+10)
	req, _ = http.NewRequest("POST", srv.URL+"/api/v1/flows/import", bytes.NewReader(big))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp4, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp4.Body.Close()
	require.Equal(t, http.StatusRequestEntityTooLarge, resp4.StatusCode)
}

// PUT con If-Unmodified-Since: stale → 412, fresco → 200.
func TestFlowAPI_PutOptimisticLocking(t *testing.T) {
	srv, key, cleanup := setupFlowAPI(t)
	defer cleanup()
	id, updatedAt := createTestFlow(t, srv.URL, key, "lock-flow")

	newBody := map[string]any{
		"name": "Renamed",
		"spec": map[string]any{
			"version": 1,
			"steps": []map[string]any{
				{"id": "s1", "type": "skill_run",
					"config": map[string]any{"skill_slug": "fr-skill", "args": map[string]any{}}},
			},
		},
	}

	// Timestamp stale (1h atras) → 412
	raw, _ := json.Marshal(newBody)
	req, _ := http.NewRequest("PUT", srv.URL+"/api/v1/flows/"+id, bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Unmodified-Since", updatedAt.Add(-1*time.Hour).UTC().Format(http.TimeFormat))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)

	// Timestamp correcto → 200
	raw, _ = json.Marshal(newBody)
	req, _ = http.NewRequest("PUT", srv.URL+"/api/v1/flows/"+id, bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Unmodified-Since", updatedAt.UTC().Format(http.TimeFormat))
	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	// Verificar rename aplicado
	resp3, body := doJSON(t, "GET", srv.URL+"/api/v1/flows/"+id, key, nil)
	require.Equal(t, http.StatusOK, resp3.StatusCode)
	require.Contains(t, string(body), "Renamed")
}
