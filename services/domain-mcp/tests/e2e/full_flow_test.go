//go:build integration

// Package e2e — tests end-to-end que validan flujos cliente completos.
//
// NO duplica unit tests granulares; cubre el HAPPY PATH del cliente:
//  1. Crear org + user owner
//  2. Emitir API key (simula post-verify-OTP)
//  3. Crear project
//  4. Save observations (con privacy + dedup verificado en service)
//  5. Search global cross-entity
//  6. Crear skill + agent
//  7. Ejecutar agent → verificar agent_run + logs persistidos
//  8. Crear flow → ejecutar
//  9. Soft-delete + restore
//  10. GDPR export incluye todo el data del user
//
// Si este test pasa, las APIs públicas funcionan end-to-end.
package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
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
	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	agentsvc "nunezlagos/domain/internal/service/agent"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/knowledge"
	"nunezlagos/domain/internal/service/lifecycle"
	"nunezlagos/domain/internal/service/observation"
	projsvc "nunezlagos/domain/internal/service/project"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	searchsvc "nunezlagos/domain/internal/service/search"
	skillsvc "nunezlagos/domain/internal/service/skill"
	timelinesvc "nunezlagos/domain/internal/service/timeline"
)

// stubProvider para tests E2E (no toca red real)
type stubProvider struct{}

func (stubProvider) Name() string { return "stub" }
func (stubProvider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	return &llm.Response{
		Content:      "Respuesta simulada del agente",
		Model:        opts.Model,
		Usage:        llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		FinishReason: "stop",
	}, nil
}
func (stubProvider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	return nil, nil
}

type e2eFixture struct {
	srv    *httptest.Server
	apiKey string
	orgID  uuid.UUID
	userID uuid.UUID
}

func setupE2E(t *testing.T) (*e2eFixture, func()) {
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

	// Services
	rec := &audit.PGRecorder{Pool: pools.Auth}
	projS := &projsvc.Service{Pool: pools.App, Audit: rec}
	obsS := &observation.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}
	skillS := &skillsvc.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}
	agentS := &agentsvc.Service{Pool: pools.App, Audit: rec}
	flowS := &flow.Service{Pool: pools.App, Audit: rec}
	lifeS := &lifecycle.Service{Pool: pools.App, Audit: rec}
	keys := &apikey.PGStore{Pool: pools.Auth, FieldEncKey: "test-field-enc-key"}

	// LLM factory con stub registrado como "ollama" (provider en whitelist del agent service)
	factory := llm.NewFactory()
	factory.Register("ollama", stubProvider{})

	skillR := skillrunner.New()
	agentR := &agentrunner.Runner{
		Pool: pools.App, Audit: rec, Factory: factory,
		Agents: agentS, Skills: skillS,
		SkillRunner: skillR,
	}
	flowR := &flowrunner.Runner{
		Pool: pools.App, Audit: rec, Flows: flowS,
		Agents: agentS, Skills: skillS, Observations: obsS,
		AgentRunner: agentR, SkillRunner: skillR,
	}

	api := &handler.API{
		ProjectService:   projS,
		ObsService:       obsS,
		PromptService:    &promptsvc.Service{Pool: pools.App, Audit: rec},
		TimelineService:  &timelinesvc.Service{Pool: pools.App},
		SearchService:    &searchsvc.Service{Pool: pools.App},
		KnowledgeService: &knowledge.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}},
		LifecycleService: lifeS,
		SkillService:     skillS,
		AgentService:     agentS,
		AgentRunner:      agentR,
		FlowService:      flowS,
		FlowRunner:       flowR,
		APIKeys:          keys,
	}

	// Setup inicial: org + user + API key (simula post-verify-OTP)
	org, owner, err := seedOrgUser(ctx, pools.App, "Acme E2E", "acme-e2e", "owner@e2e.test", "Owner E2E")
	require.NoError(t, err)

	plaintext, _, err := keys.Issue(ctx, org.ID, owner.UserID, "e2e-key", "test")
	require.NoError(t, err)

	// Middleware stack
	// REQ-42.3: idempotency middleware removido (idempotency_keys dropeada).
	authMW := &apikey.Middleware{Resolver: keys, Allowlist: handler.AuthAllowlist()}
	srv := httptest.NewServer(authMW.Wrap(api.Router()))

	return &e2eFixture{
			srv: srv, apiKey: plaintext, orgID: org.ID, userID: owner.UserID,
		}, func() {
			srv.Close()
			pools.Close()
			_ = pgC.Terminate(ctx)
		}
}

// pickID extrae el campo id del response. Tolerante a "ID" (Go default
// JSON marshal de field exported) y "id" (custom tag).
func pickID(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	for _, k := range []string{"id", "ID"} {
		if s, ok := m[k].(string); ok {
			return s
		}
	}
	return ""
}

// req helper: POST/GET con auth, devuelve status + body parsed.
func (f *e2eFixture) req(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(raw)
	}
	r, err := http.NewRequest(method, f.srv.URL+path, bodyReader)
	require.NoError(t, err)
	r.Header.Set("Authorization", "Bearer "+f.apiKey)
	r.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(r)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &parsed)
	}
	return resp.StatusCode, parsed
}

// Test E2E completo: simula lo que hace un cliente real desde cero.
func TestE2E_FullClientFlow(t *testing.T) {
	f, cleanup := setupE2E(t)
	defer cleanup()

	// === FASE 1: Project + Observations + Search ===

	// Crear project
	st, body := f.req(t, "POST", "/api/v1/projects",
		map[string]any{"name": "Demo", "slug": "demo", "description": "test"})
	require.Equalf(t, 201, st, "create project failed: %+v", body)

	// Listar projects
	st, body = f.req(t, "GET", "/api/v1/projects", nil)
	require.Equal(t, 200, st)
	projects := body["data"].([]any)
	require.Len(t, projects, 1)

	// Save observation con privacy strip
	st, body = f.req(t, "POST", "/api/v1/observations", map[string]any{
		"project_slug":     "demo",
		"content":          "Decidimos usar pgvector. <private>token_secreto_123</private> Es rápido.",
		"observation_type": "decision",
		"tags":             []string{"arch", "db"},
	})
	require.Equalf(t, 201, st, "save observation: %+v", body)
	// Content verification: serializa nuevamente como JSON y busca por substring
	raw, _ := json.Marshal(body)
	require.NotContains(t, string(raw), "token_secreto_123", "<private> stripped end-to-end")

	// Save segunda observation (test dedup permite distinto content)
	st, _ = f.req(t, "POST", "/api/v1/observations", map[string]any{
		"project_slug": "demo",
		"content":      "El clima en Santiago hoy es soleado",
		"tags":         []string{"misc"},
	})
	require.Equal(t, 201, st)

	// Tercera observation con MISMO content que la primera → dedup rechaza
	st, body = f.req(t, "POST", "/api/v1/observations", map[string]any{
		"project_slug":     "demo",
		"content":          "Decidimos usar pgvector. Es rápido.",
		"observation_type": "decision",
	})
	require.NotEqual(t, 201, st, "dedup hash debe rechazar duplicate (mismo content+type+project)")

	// Search global
	st, body = f.req(t, "GET", "/api/v1/search?q=pgvector", nil)
	require.Equal(t, 200, st)
	results := body["data"].([]any)
	require.NotEmpty(t, results, "search debe encontrar la observation de pgvector")

	// === FASE 2: (REQ-42.3) sessions dropeada — fase de lifecycle de sesión removida ===

	// === FASE 3: Skill + Agent + Run ===

	// Crear skill tipo prompt
	st, _ = f.req(t, "POST", "/api/v1/skills", map[string]any{
		"slug":        "summarize",
		"name":        "Summarize",
		"description": "resume un texto en 3 líneas",
		"type":        "prompt",
		"content":     "Resume en 3 líneas:\n{{text}}",
		"input_schema": map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}},
			"required":   []any{"text"},
		},
	})
	require.Equal(t, 201, st)

	// Crear agent que usa el skill
	st, body = f.req(t, "POST", "/api/v1/agents", map[string]any{
		"slug":          "summarizer",
		"name":          "Summarizer Agent",
		"provider":      "ollama",
		"model":         "llama3.1",
		"system_prompt": "Eres un asistente que resume textos.",
		"skills_slugs":  []string{"summarize"},
	})
	require.Equalf(t, 201, st, "create agent: %+v", body)
	agentID := pickID(body["data"])
	require.NotEmpty(t, agentID, "agent id presente")

	// Ejecutar agent
	st, body = f.req(t, "POST", "/api/v1/agents/"+agentID+"/run",
		map[string]any{"input": "Resumime el clima"})
	require.Equalf(t, 200, st, "run agent: %+v", body)
	runData, _ := body["data"].(map[string]any)
	require.Equal(t, "completed", runData["status"])
	runID, _ := runData["run_id"].(string)
	require.NotEmpty(t, runID)

	// Logs del agent_run persistieron
	st, body = f.req(t, "GET", "/api/v1/agent-runs/"+runID+"/logs", nil)
	require.Equal(t, 200, st)
	logs := body["data"].([]any)
	require.NotEmpty(t, logs, "agent_run_logs deben tener al menos 1 entry (llm_call + final)")

	// === FASE 4: Flow ===

	// Crear flow simple
	st, body = f.req(t, "POST", "/api/v1/flows", map[string]any{
		"slug": "greet-flow", "name": "Greet Flow",
		"spec": map[string]any{
			"version": 1,
			"steps": []map[string]any{
				{
					"id":   "greet",
					"type": "skill_run",
					"config": map[string]any{
						"skill_slug": "summarize",
						"args":       map[string]any{"text": "hello world"},
					},
				},
			},
		},
	})
	require.Equalf(t, 201, st, "create flow: %+v", body)
	flowID := pickID(body["data"])
	require.NotEmpty(t, flowID)

	st, body = f.req(t, "POST", "/api/v1/flows/"+flowID+"/run",
		map[string]any{"inputs": map[string]any{}})
	require.Equalf(t, 200, st, "run flow: %+v", body)
	require.Equal(t, "completed", body["data"].(map[string]any)["status"])

	// === FASE 5: Restore + GDPR export ===

	// Soft delete project
	st, _ = f.req(t, "DELETE", "/api/v1/projects/demo", nil)
	require.Equal(t, 204, st)

	// Restore
	projID := pickID(projects[0])
	require.NotEmpty(t, projID)
	st, _ = f.req(t, "POST", "/api/v1/restore", map[string]any{
		"entity_type": "project",
		"entity_id":   projID,
	})
	require.Equal(t, 204, st)

	// Project visible de nuevo
	st, _ = f.req(t, "GET", "/api/v1/projects/demo", nil)
	require.Equal(t, 200, st)

	// GDPR export
	st, body = f.req(t, "GET", "/api/v1/me/export", nil)
	require.Equal(t, 200, st)
	exp := body["data"].(map[string]any)
	require.Equal(t, "1.0", exp["export_version"])
	require.NotEmpty(t, exp["organizations"])
	require.NotEmpty(t, exp["projects"])
	require.NotEmpty(t, exp["observations"])
	// Sin secrets en auth_api_keys
	if apiKeys, ok := exp["api_keys_metadata"].([]any); ok {
		for _, k := range apiKeys {
			km := k.(map[string]any)
			_, hasHash := km["key_hash"]
			require.False(t, hasHash, "key_hash NO debe estar en export")
		}
	}
}

// Test E2E del idempotency middleware: misma Idempotency-Key con body
// idéntico devuelve la response cached con Idempotent-Replayed: true.
func TestE2E_Idempotency(t *testing.T) {
	f, cleanup := setupE2E(t)
	defer cleanup()

	body := map[string]any{"name": "Demo Idemp", "slug": "demo-idemp"}
	raw, _ := json.Marshal(body)
	key := uuid.New().String()

	// Primera request
	r1, _ := http.NewRequest("POST", f.srv.URL+"/api/v1/projects", bytes.NewReader(raw))
	r1.Header.Set("Authorization", "Bearer "+f.apiKey)
	r1.Header.Set("Content-Type", "application/json")
	r1.Header.Set("Idempotency-Key", key)
	resp1, err := http.DefaultClient.Do(r1)
	require.NoError(t, err)
	resp1.Body.Close()
	require.Equal(t, 201, resp1.StatusCode)
	require.Empty(t, resp1.Header.Get("Idempotent-Replayed"))

	// Segunda request con MISMA key + mismo body → replayed
	r2, _ := http.NewRequest("POST", f.srv.URL+"/api/v1/projects", bytes.NewReader(raw))
	r2.Header.Set("Authorization", "Bearer "+f.apiKey)
	r2.Header.Set("Content-Type", "application/json")
	r2.Header.Set("Idempotency-Key", key)
	resp2, err := http.DefaultClient.Do(r2)
	require.NoError(t, err)
	resp2.Body.Close()
	require.Equal(t, 201, resp2.StatusCode, "replay devuelve mismo status")
	require.Equal(t, "true", resp2.Header.Get("Idempotent-Replayed"))

	// Tercera request misma key con body DISTINTO → 409
	differentBody, _ := json.Marshal(map[string]any{"name": "Otro", "slug": "demo-idemp"})
	r3, _ := http.NewRequest("POST", f.srv.URL+"/api/v1/projects", bytes.NewReader(differentBody))
	r3.Header.Set("Authorization", "Bearer "+f.apiKey)
	r3.Header.Set("Content-Type", "application/json")
	r3.Header.Set("Idempotency-Key", key)
	resp3, err := http.DefaultClient.Do(r3)
	require.NoError(t, err)
	resp3.Body.Close()
	require.Equal(t, 409, resp3.StatusCode, "misma key con body distinto → 409 mismatch")
}

// Test E2E sin auth: API responde 401.
func TestE2E_UnauthorizedRejected(t *testing.T) {
	f, cleanup := setupE2E(t)
	defer cleanup()

	r, _ := http.NewRequest("GET", f.srv.URL+"/api/v1/projects", nil)
	resp, err := http.DefaultClient.Do(r)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, 401, resp.StatusCode)
}
