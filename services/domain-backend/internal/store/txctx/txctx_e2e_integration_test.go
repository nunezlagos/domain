//go:build integration

// issue-25.14 E2E test: apikey.Middleware con Pool inyecta tx con SET LOCAL
// en el ctx; handlers downstream la extraen con txctx.TxFromContext.
//
// Cubre: wireup HTTP post-auth + observacion.RLS via tx del ctx + sabotaje
// (handler que ignora tx usa pool directo → RLS bloquea).

package txctx_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"nunezlagos/domain/internal/auth/apikey"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/store/txctx"
)

// mockResolver simula apikey.Resolver devolviendo un Principal fijo.
type mockResolver struct {
	p *apikey.Principal
}

func (m *mockResolver) Resolve(ctx context.Context, plaintext string) (*apikey.Principal, error) {
	if plaintext == "domk_test_key" {
		return m.p, nil
	}
	return nil, apikey.ErrUnauthorized
}

// setupE2E levanta testcontainer, corre migrations + seed basico (2 orgs).
// Retorna pool y cleanup. Es la misma setup que txctx_integration_test.go
// pero renombrada para evitar colision.
func setupE2E(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))

	// GRANT app_user TO test (test es DB owner y bypassea RLS sin esto)
	bootstrap, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	_, err = bootstrap.Exec(ctx, `GRANT app_user TO test`)
	require.NoError(t, err)
	bootstrap.Close()

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE app_user`)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

// seedE2ECrossOrg crea 2 orgs + 1 user por org + 1 observation por org.
// Retorna (orgA, orgB, userA, userB, obsA, obsB, apiKeyA, apiKeyB, cleanup).
func seedE2ECrossOrg(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var orgA, orgB, userA, userB uuid.UUID
	var projectA, projectB uuid.UUID
	var obsA, obsB uuid.UUID

	// BYPASSRLS para seed: usar pool directo (test user es owner).
	// Necesitamos correr como superuser para bypassear RLS y sembrar.
	// Workaround: SET LOCAL app.current_org_id no funciona porque test es
	// owner; usamos una conexion fresca con SET ROLE app_admin.
	adminPool, err := pgxpool.New(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer adminPool.Close()
	_, err = adminPool.Exec(ctx, `SET ROLE app_admin`)
	require.NoError(t, err)

	// user needs organization_id FK; for now seed with explicit org + user.
	require.NoError(t, adminPool.QueryRow(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES (gen_random_uuid(), 'A', 'org-a') RETURNING id`).Scan(&orgA))
	require.NoError(t, adminPool.QueryRow(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES (gen_random_uuid(), 'B', 'org-b') RETURNING id`).Scan(&orgB))
	require.NoError(t, adminPool.QueryRow(ctx,
		`INSERT INTO users (id, organization_id, email) VALUES (gen_random_uuid(), $1, 'a@test') RETURNING id`, orgA).Scan(&userA))
	require.NoError(t, adminPool.QueryRow(ctx,
		`INSERT INTO users (id, organization_id, email) VALUES (gen_random_uuid(), $1, 'b@test') RETURNING id`, orgB).Scan(&userB))
	require.NoError(t, adminPool.QueryRow(ctx,
		`INSERT INTO projects (id, organization_id, name, slug) VALUES (gen_random_uuid(), $1, 'PA', 'pa') RETURNING id`, orgA).Scan(&projectA))
	require.NoError(t, adminPool.QueryRow(ctx,
		`INSERT INTO projects (id, organization_id, name, slug) VALUES (gen_random_uuid(), $1, 'PB', 'pb') RETURNING id`, orgB).Scan(&projectB))
	require.NoError(t, adminPool.QueryRow(ctx,
		`INSERT INTO observations (id, organization_id, project_id, content) VALUES (gen_random_uuid(), $1, $2, 'A obs') RETURNING id`, orgA, projectA).Scan(&obsA))
	require.NoError(t, adminPool.QueryRow(ctx,
		`INSERT INTO observations (id, organization_id, project_id, content) VALUES (gen_random_uuid(), $1, $2, 'B obs') RETURNING id`, orgB, projectB).Scan(&obsB))

	return orgA, orgB, userA, userB, obsA, obsB
}

// TestE2E_Wireup_HTTP_GET observations cross-org isolation
// Escenario 1: GET con API key de A devuelve solo obs de A.
// Escenario 2: GET id de B con API key de A → 404 (no leak).
// Escenario 3 (sabotaje): handler que ignora tx usa pool directo → RLS
// devuelve 0 rows aunque el WHERE intente id de B.
func TestE2E_Wireup_HTTP_GET_Observations_CrossOrgIsolation(t *testing.T) {
	pool, cleanup := setupE2E(t)
	defer cleanup()
	orgA, orgB, userA, userB, obsA, obsB := seedE2ECrossOrg(t, pool)
	_ = userA
	_ = userB

	// Handler dummy que delibera leer observations con la tx del ctx.
	// Si no hay tx, usa pool (que se cae por RLS).
	listHandler := func(w http.ResponseWriter, r *http.Request) {
		tx := txctx.TxFromContext(r.Context())
		if tx == nil {
			http.Error(w, `{"error":"no tx in ctx"}`, 500)
			return
		}
		rows, err := tx.Query(r.Context(), `SELECT id::text, content FROM observations ORDER BY created_at`)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"query: %v"}`, err), 500)
			return
		}
		defer rows.Close()
		var out []map[string]string
		for rows.Next() {
			var id, content string
			_ = rows.Scan(&id, &content)
			out = append(out, map[string]string{"id": id, "content": content})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": out})
	}

	getHandler := func(idWanted uuid.UUID) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			tx := txctx.TxFromContext(r.Context())
			if tx == nil {
				http.Error(w, `{"error":"no tx in ctx"}`, 500)
				return
			}
			var got uuid.UUID
			err := tx.QueryRow(r.Context(), `SELECT id FROM observations WHERE id = $1`, idWanted).Scan(&got)
			if err == pgx.ErrNoRows {
				http.Error(w, `{"error":"not found"}`, 404)
				return
			}
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": got})
		}
	}

	// Montar router
	resolverA := &mockResolver{p: &apikey.Principal{UserID: userA.String(), OrganizationID: orgA.String(), APIKeyID: uuid.New().String()}}
	resolverB := &mockResolver{p: &apikey.Principal{UserID: userB.String(), OrganizationID: orgB.String(), APIKeyID: uuid.New().String()}}

	muxA := http.NewServeMux()
	muxA.HandleFunc("GET /observations", listHandler)
	muxA.HandleFunc("GET /observations/{id}", getHandler(obsB)) // intenta leer obs de B con auth de A
	srvA := httptest.NewServer(&apikey.Middleware{Resolver: resolverA, Allowlist: []string{}, Pool: pool}.Wrap(muxA))
	defer srvA.Close()

	muxB := http.NewServeMux()
	muxB.HandleFunc("GET /observations", listHandler)
	muxB.HandleFunc("GET /observations/{id}", getHandler(obsA)) // intenta leer obs de A con auth de B
	srvB := httptest.NewServer(&apikey.Middleware{Resolver: resolverB, Allowlist: []string{}, Pool: pool}.Wrap(muxB))
	defer srvB.Close()

	// 1. GET con key de A → solo obs de A
	req, _ := http.NewRequest("GET", srvA.URL+"/observations", nil)
	req.Header.Set("Authorization", "Bearer domk_test_key")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var listA struct {
		Data []map[string]string `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listA))
	require.Len(t, listA.Data, 1, "org A debe ver solo 1 obs (la suya)")
	require.Equal(t, "A obs", listA.Data[0]["content"])
	require.NotEqual(t, obsB.String(), listA.Data[0]["id"], "NO debe aparecer obs de B")

	// 2. GET con key de B → solo obs de B
	req, _ = http.NewRequest("GET", srvB.URL+"/observations", nil)
	req.Header.Set("Authorization", "Bearer domk_test_key")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var listB struct {
		Data []map[string]string `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listB))
	require.Len(t, listB.Data, 1)
	require.Equal(t, "B obs", listB.Data[0]["content"])

	// 3. Sabotaje: GET con key de A el id de obs de B → 404 (RLS bloquea)
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/observations/%s", srvA.URL, obsB), nil)
	req.Header.Set("Authorization", "Bearer domk_test_key")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 404, resp.StatusCode,
		"cross-org GET de obs de B con auth de A debe ser 404 (RLS bloquea, no 200 ni 500)")
}

// TestE2E_Sabotage_HandlerIgnoresTx_USES_POOL
// Simula un handler con BUG que ignora la tx del ctx y consulta el pool
// directo. La RLS sigue bloqueando → 0 rows → ErrNotFound, no leak.
func TestE2E_Sabotage_HandlerIgnoresTx(t *testing.T) {
	pool, cleanup := setupE2E(t)
	defer cleanup()
	orgA, orgB, userA, userB, obsA, obsB := seedE2ECrossOrg(t, pool)
	_ = userA
	_ = userB

	// Handler ROTO: deliberadamente NO usa la tx del ctx, usa pool directo.
	// Es un bug RBAC simulado. La RLS debe defender.
	buggyHandler := func(w http.ResponseWriter, r *http.Request) {
		// BUG: ignora txctx.TxFromContext y va directo al pool.
		// ISSUE-21.6 single-org: el filtro cross-org ya no aplica (la tabla
		// organizations se dropea, no hay RLS por org). Mantenemos el handler
		// pero removemos el WHERE organization_id = $2 (siempre retorna
		// el row, lo cual es el comportamiento single-org esperado).
		var got int
		err := pool.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM observations WHERE id = $1`,
			obsB).Scan(&got)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"count": got})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /buggy", buggyHandler)
	resolverA := &mockResolver{p: &apikey.Principal{UserID: uuid.New().String(), OrganizationID: orgA.String()}}
	srv := httptest.NewServer(&apikey.Middleware{Resolver: resolverA, Allowlist: []string{}, Pool: pool}.Wrap(mux))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/buggy", nil)
	req.Header.Set("Authorization", "Bearer domk_test_key")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var out struct {
		Count int `json:"count"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Equal(t, 0, out.Count,
		"handler buggy + RLS: cross-org SELECT DEBE devolver 0 (defense in depth)")
	_ = obsA
}
