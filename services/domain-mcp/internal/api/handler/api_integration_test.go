//go:build integration

package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	"nunezlagos/domain/internal/service/observation"
	projsvc "nunezlagos/domain/internal/service/project"
	searchsvc "nunezlagos/domain/internal/service/search"

	"github.com/google/uuid"
)

func setupAPI(t *testing.T) (*httptest.Server, string, func()) {
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

	// Dos pools tipados (equivalente exacto al modelo de prod):
	//   pools.App  → SET ROLE app_user (NOBYPASSRLS)
	//   pools.Auth → SET ROLE app_admin (BYPASSRLS)
	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)
	pool := pools.App
	authPool := pools.Auth

	// Audit recorder usa AuthPool: audit_log INSERT policy es WITH CHECK true
	// (permite cross-org system events), pero los SELECT/lookups internos
	// requieren BYPASSRLS para reporting.
	rec := &audit.PGRecorder{Pool: authPool}

	// Services domain usan AppPool. Sus tablas (organizations, users, projects,
	// observations, auth_invitations) NO tienen RLS habilitada — sus services
	// validan org_id en la query. Tablas con RLS (auth_api_keys, audit_log,
	// auth_otp_codes, activity_log, auth_secrets) las accede AuthPool o un flujo
	// con txctx.WithOrgTx explicito.
	projS := &projsvc.Service{Pool: pool, Audit: rec}
	obsS := &observation.Service{Pool: pool, Audit: rec, Embedder: llm.FakeEmbedder{}}

	// apikey store usa AuthPool: Resolve hace lookup global de auth_api_keys por
	// prefix (no conoce org_id aun) y necesita atravesar RLS.
	keys := &apikey.PGStore{Pool: authPool, FieldEncKey: "test-field-enc-key"}

	searchS := &searchsvc.Service{Pool: pool}
	api := &handler.API{
		ProjectService: projS,
		ObsService:     obsS,
		SearchService:  searchS,
		APIKeys:        keys,
	}

	// Insertar org + user directamente (el org.Service fue removido). El schema
	// aun exige users.organization_id NOT NULL con FK a organizations, asi que
	// creamos ambas filas via SQL sobre el AuthPool (BYPASSRLS) y emitimos la key.
	var orgID, userID uuid.UUID
	require.NoError(t, authPool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID))
	require.NoError(t, authPool.QueryRow(ctx,
		`INSERT INTO users (organization_id, email, name, role) VALUES ($1, 'owner@acme.com', 'Owner', 'owner') RETURNING id`,
		orgID,
	).Scan(&userID))
	plaintext, _, err := keys.Issue(ctx, orgID, userID, "test-key", "test")
	require.NoError(t, err)

	// Middleware stack: auth + tx RLS → router. Pool es OBLIGATORIO desde
	// migration 000085 (observations/sessions con RLS FORCE): sin el no
	// se abre la tx con SET LOCAL y los writes devuelven 500.
	mw := &apikey.Middleware{Resolver: keys, Allowlist: handler.AuthAllowlist(), Pool: pools.App}
	handler := mw.Wrap(api.Router())

	srv := httptest.NewServer(handler)
	return srv, plaintext, func() {
		srv.Close()
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func doJSON(t *testing.T, method, url, key string, body any) (*http.Response, []byte) {
	t.Helper()
	var bodyReader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	require.NoError(t, err)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	buf := make([]byte, 0, 1024)
	chunk := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}
		if err != nil {
			break
		}
	}
	return resp, buf
}

func TestAPI_Unauth_Rejected(t *testing.T) {
	srv, _, cleanup := setupAPI(t)
	defer cleanup()
	resp, _ := doJSON(t, "GET", srv.URL+"/api/v1/projects", "", nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAPI_HappyPath_ProjectAndObservation(t *testing.T) {
	srv, key, cleanup := setupAPI(t)
	defer cleanup()

	// Crear project
	resp, body := doJSON(t, "POST", srv.URL+"/api/v1/projects", key, map[string]any{
		"name": "Demo", "slug": "demo", "description": "test",
	})
	require.Equalf(t, http.StatusCreated, resp.StatusCode, "body=%s", body)

	// Listar projects
	resp, body = doJSON(t, "GET", srv.URL+"/api/v1/projects", key, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var listResp struct {
		Data []struct {
			Slug string `json:"Slug"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &listResp))
	require.Len(t, listResp.Data, 1)

	// Save observation
	resp, body = doJSON(t, "POST", srv.URL+"/api/v1/observations", key, map[string]any{
		"project_slug": "demo",
		"content":      "decidimos usar pgvector con embeddings",
		"tags":         []string{"arch"},
	})
	require.Equalf(t, http.StatusCreated, resp.StatusCode, "body=%s", body)

	// Search
	resp, body = doJSON(t, "GET", srv.URL+"/api/v1/search?q=pgvector", key, nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", body)
	require.Contains(t, string(body), "pgvector")
}

func TestAPI_OTPRequest_AntiEnumeration(t *testing.T) {
	srv, _, cleanup := setupAPI(t)
	defer cleanup()
	// Sin OTPService configurado el handler igualmente devuelve 200 (anti-enum)
	resp, _ := doJSON(t, "POST", srv.URL+"/api/v1/auth/request-otp", "", map[string]any{
		"identifier": "nadie@x.com",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
