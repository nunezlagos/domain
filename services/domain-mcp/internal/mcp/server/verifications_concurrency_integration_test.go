//go:build integration

package mcpserver_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/db"
	mcpserver "nunezlagos/domain/internal/mcp/server"
	dmigrate "nunezlagos/domain/internal/migrate"
	projsvc "nunezlagos/domain/internal/service/project"
)

type verifFixture struct {
	srv            *mcptest.Server
	verificationID string
	pool           *db.Pools
	cleanup        func()
}

func setupVerifConcurrencia(t *testing.T) *verifFixture {
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
	projS := &projsvc.Service{Pool: pools.App, Audit: rec}

	org, owner, err := seedOrgUser(ctx, pools.App, "Acme", "acme", "owner@acme.com", "Owner")
	require.NoError(t, err)
	proj, err := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "Demo", Slug: "demo", ActorID: owner.UserID,
	})
	require.NoError(t, err)

	// tres items para que cada goroutine mute uno distinto
	var verificationID string
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO tdd_verifications (project_id, user_id, kind, items, status, context)
		 VALUES ($1, $2, 'test',
		         '[{"label":"item-a","status":"pending"},
		           {"label":"item-b","status":"pending"},
		           {"label":"item-c","status":"pending"}]',
		         'running', 'checkpoint concurrente')
		 RETURNING id`,
		proj.ID, owner.UserID,
	).Scan(&verificationID))

	deps := mcpserver.Deps{
		Principal: &apikey.Principal{
			UserID:         owner.UserID.String(),
			OrganizationID: org.ID.String(),
			Role:           "owner",
		},
		Projects:   projS,
		Pool:       pools.App,
		ServerName: "domain-mcp-test",
		ServerVer:  "0.0.0",
	}
	testSrv, err := mcptest.NewServer(t, mcpserver.Tools(deps)...)
	require.NoError(t, err)

	return &verifFixture{
		srv:            testSrv,
		verificationID: verificationID,
		pool:           pools,
		cleanup: func() {
			testSrv.Close()
			pools.Close()
			_ = pgC.Terminate(ctx)
		},
	}
}

func (f *verifFixture) items(t *testing.T) map[string]string {
	t.Helper()
	var raw []byte
	require.NoError(t, f.pool.App.QueryRow(context.Background(),
		`SELECT items FROM tdd_verifications WHERE id = $1`, f.verificationID,
	).Scan(&raw))
	var items []map[string]any
	require.NoError(t, json.Unmarshal(raw, &items))
	out := map[string]string{}
	for _, it := range items {
		label, _ := it["label"].(string)
		status, _ := it["status"].(string)
		out[label] = status
	}
	return out
}

func callVerifTool(t *testing.T, srv *mcptest.Server, name string, args map[string]any) {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	result, err := srv.Client().CallTool(context.Background(), req)
	require.NoError(t, err)
	require.Falsef(t, result.IsError, "tool '%s' error: %+v", name, result.Content)
}

// DOMAINSERV-217: el read-modify-write del JSONB items no estaba en una
// transacción, así que dos update_item sobre labels distintos se pisaban el
// items completo — el último en escribir borraba el status del primero.
func TestMCP_VerifyUpdateItem_TresLabelsEnParalelo_NoSePisan(t *testing.T) {
	f := setupVerifConcurrencia(t)
	defer f.cleanup()

	labels := []string{"item-a", "item-b", "item-c"}
	var wg sync.WaitGroup
	for _, label := range labels {
		wg.Add(1)
		go func(l string) {
			defer wg.Done()
			callVerifTool(t, f.srv, "domain_verify_update_item", map[string]any{
				"verification_id": f.verificationID,
				"label":           l,
				"status":          "pass",
			})
		}(label)
	}
	wg.Wait()

	got := f.items(t)
	for _, l := range labels {
		require.Equal(t, "pass", got[l],
			"el label %s se perdió: un update concurrente lo sobreescribió", l)
	}
}

func TestMCP_VerifyUpdateItem_UnLabel_SoloEseCambia(t *testing.T) {
	f := setupVerifConcurrencia(t)
	defer f.cleanup()

	callVerifTool(t, f.srv, "domain_verify_update_item", map[string]any{
		"verification_id": f.verificationID,
		"label":           "item-b",
		"status":          "fail",
		"output":          "detalle del fallo",
	})

	got := f.items(t)
	require.Equal(t, "fail", got["item-b"])
	require.Equal(t, "pending", got["item-a"])
	require.Equal(t, "pending", got["item-c"])
}
