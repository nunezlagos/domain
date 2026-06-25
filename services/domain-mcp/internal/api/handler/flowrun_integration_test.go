//go:build integration

package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/api/handler"
	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/llm"
	dmigrate "nunezlagos/domain/internal/migrate"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/observation"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

type flowAPIFixture struct {
	srv  *httptest.Server
	key  string
	pool *db.Pools
}

func setupFlowAPI(t *testing.T) (*httptest.Server, string, func()) {
	fx, cleanup := setupFlowAPIFull(t)
	return fx.srv, fx.key, cleanup
}

func setupFlowAPIFull(t *testing.T) (*flowAPIFixture, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))

	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)

	rec := &audit.PGRecorder{Pool: pools.Auth}
	flowS := &flow.Service{Pool: pools.App, Audit: rec}
	skillS := &skillsvc.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}
	obsS := &observation.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}
	keys := &apikey.PGStore{Pool: pools.Auth}

	runner := &flowrunner.Runner{
		Pool: pools.App, Audit: rec, Flows: flowS,
		Skills: skillS, Observations: obsS,
		SkillRunner: skillrunner.New(),
	}

	api := &handler.API{
		FlowService: flowS,
		FlowRunner:  runner,
		APIKeys:     keys,
		Audit:       rec,
	}

	org, owner, err := seedOrgUser(ctx, pools.App, "FlowOrg", "floworg", "f@x.com", "F")
	require.NoError(t, err)
	_, err = skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: org.ID, Slug: "fr-skill", Name: "FR",
		SkillType: skillsvc.TypePrompt, Content: "ok",
		InputSchema: map[string]any{"type": "object"},
		ActorID:     owner.UserID,
	})
	require.NoError(t, err)
	plaintext, _, err := keys.Issue(ctx, org.ID, owner.UserID, "test-key", "test")
	require.NoError(t, err)

	mw := &apikey.Middleware{Resolver: keys, Allowlist: handler.AuthAllowlist()}
	srv := httptest.NewServer(mw.Wrap(api.Router()))
	return &flowAPIFixture{srv: srv, key: plaintext, pool: pools}, func() {
		srv.Close()
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestFlowRunAPI_Lifecycle(t *testing.T) {
	srv, key, cleanup := setupFlowAPI(t)
	defer cleanup()


	resp, body := doJSON(t, "POST", srv.URL+"/api/v1/flows", key, map[string]any{
		"slug": "fr-flow", "name": "FR Flow",
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
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &created))


	resp, body = doJSON(t, "POST", srv.URL+"/api/v1/flows/"+created.Data.ID+"/run", key, map[string]any{})
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", body)
	var runResp struct {
		Data struct {
			RunID  string `json:"run_id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &runResp))
	require.Equal(t, "completed", runResp.Data.Status)


	resp, body = doJSON(t, "GET", srv.URL+"/api/v1/flow-runs/"+runResp.Data.RunID, key, nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", body)
	var got struct {
		Data struct {
			Status string `json:"status"`
			Steps  []struct {
				StepKey string `json:"step_key"`
				Status  string `json:"status"`
			} `json:"steps"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "completed", got.Data.Status)
	require.Len(t, got.Data.Steps, 1)
	require.Equal(t, "s1", got.Data.Steps[0].StepKey)
	require.Equal(t, "completed", got.Data.Steps[0].Status)


	resp, _ = doJSON(t, "POST", srv.URL+"/api/v1/flow-runs/"+runResp.Data.RunID+"/pause", key, nil)
	require.Equal(t, http.StatusConflict, resp.StatusCode)


	resp, _ = doJSON(t, "POST", srv.URL+"/api/v1/flow-runs/"+runResp.Data.RunID+"/cancel", key, nil)
	require.Equal(t, http.StatusConflict, resp.StatusCode)


	resp, _ = doJSON(t, "GET", srv.URL+"/api/v1/flow-runs/00000000-0000-0000-0000-000000000001", key, nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestFlowRunAPI_PauseResumeCancel(t *testing.T) {
	fx, cleanup := setupFlowAPIFull(t)
	defer cleanup()
	ctx := context.Background()
	srv, key := fx.srv, fx.key

	resp, body := doJSON(t, "POST", srv.URL+"/api/v1/flows", key, map[string]any{
		"slug": "pr-flow", "name": "PR Flow",
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
			ID             string `json:"id"`
			OrganizationID string `json:"organization_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &created))


	var runID string
	require.NoError(t, fx.pool.App.QueryRow(ctx, `
		INSERT INTO flow_runs (organization_id, flow_id, trigger_type, status, inputs)
		SELECT organization_id, id, 'manual', 'running', '{}' FROM flows WHERE id = $1
		RETURNING id::text`, created.Data.ID).Scan(&runID))


	resp, body = doJSON(t, "POST", srv.URL+"/api/v1/flow-runs/"+runID+"/pause", key, nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", body)

	resp, body = doJSON(t, "POST", srv.URL+"/api/v1/flow-runs/"+runID+"/resume", key, nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", body)

	resp, body = doJSON(t, "POST", srv.URL+"/api/v1/flow-runs/"+runID+"/cancel", key, nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", body)


	resp, body = doJSON(t, "GET", srv.URL+"/api/v1/flow-runs/"+runID, key, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), `"cancelled"`)

	resp, _ = doJSON(t, "POST", srv.URL+"/api/v1/flow-runs/"+runID+"/resume", key, nil)
	require.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestFlowRunAPI_SSEStream_TerminalRun(t *testing.T) {
	srv, key, cleanup := setupFlowAPI(t)
	defer cleanup()

	resp, body := doJSON(t, "POST", srv.URL+"/api/v1/flows", key, map[string]any{
		"slug": "sse-flow", "name": "SSE Flow",
		"spec": map[string]any{
			"version": 1,
			"steps": []map[string]any{
				{"id": "s1", "type": "skill_run",
					"config": map[string]any{"skill_slug": "fr-skill", "args": map[string]any{}}},
			},
		},
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &created))

	resp, body = doJSON(t, "POST", srv.URL+"/api/v1/flows/"+created.Data.ID+"/run", key, map[string]any{})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var runResp struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &runResp))


	req, err := http.NewRequest("GET", srv.URL+"/api/v1/flow-runs/"+runResp.Data.RunID+"/stream", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+key)
	streamResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer streamResp.Body.Close()
	require.Equal(t, http.StatusOK, streamResp.StatusCode)
	require.Equal(t, "text/event-stream", streamResp.Header.Get("Content-Type"))

	buf := make([]byte, 4096)
	n, _ := streamResp.Body.Read(buf)
	out := string(buf[:n])
	require.True(t, strings.HasPrefix(out, "event: status"), "primer evento debe ser status, got: %s", out)
	require.Contains(t, out, `"completed"`)
}
