//go:build integration

package mcpserver_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedOrgResult / seedMemberResult reemplazan los tipos org.Organization /
// org.Member tras remover el service org. Exponen los mismos campos (.ID /
// .UserID) que usaban los tests, evitando reescribir cada call site.
type seedOrgResult struct{ ID uuid.UUID }
type seedMemberResult struct{ UserID uuid.UUID }

// seedOrgUser inserta org + owner user directamente (el org.Service fue
// removido). Replica el núcleo del antiguo org.Service.Create: las dos filas
// que el resto del setup necesita (organizations + users con role owner). No
// ejecuta los post-create hooks (seeds de skills/agents/flows) porque ningún
// test acá depende de ese seeding.
func seedOrgUser(ctx context.Context, pool *pgxpool.Pool, name, slug, ownerEmail, ownerName string) (*seedOrgResult, *seedMemberResult, error) {
	var org seedOrgResult
	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, settings)
		 VALUES ($1, $2, '{}'::jsonb) RETURNING id`,
		name, slug,
	).Scan(&org.ID); err != nil {
		return nil, nil, err
	}
	var member seedMemberResult
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (organization_id, email, name, role)
		 VALUES ($1, $2, $3, 'owner') RETURNING id`,
		org.ID, ownerEmail, ownerName,
	).Scan(&member.UserID); err != nil {
		return nil, nil, err
	}
	return &org, &member, nil
}

// mustSeedOrgUser es la variante que falla el test ante error.
func mustSeedOrgUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name, slug, ownerEmail, ownerName string) (*seedOrgResult, *seedMemberResult) {
	t.Helper()
	org, member, err := seedOrgUser(ctx, pool, name, slug, ownerEmail, ownerName)
	if err != nil {
		t.Fatalf("seedOrgUser: %v", err)
	}
	return org, member
}
