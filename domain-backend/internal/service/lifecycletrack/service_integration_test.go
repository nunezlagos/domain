//go:build integration

package lifecycletrack_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	lt "nunezlagos/domain/internal/service/lifecycletrack"
)

func setupLT(t *testing.T) (*lt.Service, func()) {
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

	return &lt.Service{Pool: pools.App}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestRecord_CreationTransition(t *testing.T) {
	svc, cleanup := setupLT(t)
	defer cleanup()

	issueID := uuid.New()
	tr, err := svc.Record(context.Background(), lt.EntityHU, issueID, "", "proposed",
		lt.Actor{Kind: lt.ActorAgent, Name: "claude-code"},
		"initial HU spec generated", nil, nil)
	require.NoError(t, err)
	require.Equal(t, "proposed", tr.ToState)
	require.Nil(t, tr.FromState)
}

func TestRecord_ValidTransition(t *testing.T) {
	svc, cleanup := setupLT(t)
	defer cleanup()
	ctx := context.Background()

	issueID := uuid.New()
	_, _ = svc.Record(ctx, lt.EntityHU, issueID, "", "proposed",
		lt.Actor{Kind: lt.ActorSystem, Name: "init"}, "", nil, nil)

	tr, err := svc.Record(ctx, lt.EntityHU, issueID, "proposed", "approved",
		lt.Actor{Kind: lt.ActorUser, Name: "alice@acme"}, "looks good", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, tr.FromState)
	require.Equal(t, "proposed", *tr.FromState)
	require.Equal(t, "approved", tr.ToState)
}

func TestRecord_InvalidTransition(t *testing.T) {
	svc, cleanup := setupLT(t)
	defer cleanup()
	ctx := context.Background()

	_, err := svc.Record(ctx, lt.EntityHU, uuid.New(), "done", "in_progress",
		lt.Actor{Kind: lt.ActorUser, Name: "bob"}, "", nil, nil)
	require.ErrorIs(t, err, lt.ErrInvalidTransition)
}

func TestRecord_InvalidEntity(t *testing.T) {
	svc, cleanup := setupLT(t)
	defer cleanup()

	_, err := svc.Record(context.Background(), "alien_kind", uuid.New(), "", "x",
		lt.Actor{Kind: lt.ActorSystem, Name: "x"}, "", nil, nil)
	require.ErrorIs(t, err, lt.ErrInvalidEntity)
}

func TestRecord_MissingActor(t *testing.T) {
	svc, cleanup := setupLT(t)
	defer cleanup()
	_, err := svc.Record(context.Background(), lt.EntityHU, uuid.New(), "", "proposed",
		lt.Actor{}, "", nil, nil)
	require.ErrorIs(t, err, lt.ErrMissingActor)
}

func TestListByEntity_FullTimeline(t *testing.T) {
	svc, cleanup := setupLT(t)
	defer cleanup()
	ctx := context.Background()

	issueID := uuid.New()
	actor := lt.Actor{Kind: lt.ActorSystem, Name: "system"}

	_, err := svc.Record(ctx, lt.EntityHU, issueID, "", "proposed", actor, "", nil, nil)
	require.NoError(t, err)
	_, err = svc.Record(ctx, lt.EntityHU, issueID, "proposed", "approved", actor, "", nil, nil)
	require.NoError(t, err)
	_, err = svc.Record(ctx, lt.EntityHU, issueID, "approved", "in_progress", actor, "", nil, nil)
	require.NoError(t, err)

	timeline, err := svc.ListByEntity(ctx, lt.EntityHU, issueID, 0)
	require.NoError(t, err)
	require.Len(t, timeline, 3)
	require.Equal(t, "proposed", timeline[0].ToState)
	require.Equal(t, "in_progress", timeline[2].ToState)
}

func TestCurrentState(t *testing.T) {
	svc, cleanup := setupLT(t)
	defer cleanup()
	ctx := context.Background()

	issueID := uuid.New()
	actor := lt.Actor{Kind: lt.ActorSystem, Name: "x"}
	_, _ = svc.Record(ctx, lt.EntityHU, issueID, "", "proposed", actor, "", nil, nil)
	_, _ = svc.Record(ctx, lt.EntityHU, issueID, "proposed", "approved", actor, "", nil, nil)

	st, err := svc.CurrentState(ctx, lt.EntityHU, issueID)
	require.NoError(t, err)
	require.Equal(t, "approved", st)
}

func TestCanTransition_Helpers(t *testing.T) {
	require.True(t, lt.CanTransition(lt.EntityHU, "proposed", "approved"))
	require.True(t, lt.CanTransition(lt.EntityHU, "in_progress", "done"))
	require.False(t, lt.CanTransition(lt.EntityHU, "done", "in_progress"))
	require.False(t, lt.CanTransition(lt.EntityHU, "", "done"))
	require.Empty(t, lt.AllowedTransitions("bogus", ""))
}

// Sabotaje: la tabla es append-only. Defense-in-depth:
//  - app_user solo tiene GRANT SELECT, INSERT → UPDATE/DELETE → permission denied
//  - Aunque tuviera GRANT, el trigger entity_state_transitions_no_update aborta
// Cualquiera de los dos errores satisface el invariante.
func TestSabotage_ImmutableTrigger_BlocksUpdate(t *testing.T) {
	svc, cleanup := setupLT(t)
	defer cleanup()
	ctx := context.Background()

	tr, _ := svc.Record(ctx, lt.EntityHU, uuid.New(), "", "proposed",
		lt.Actor{Kind: lt.ActorSystem, Name: "x"}, "", nil, nil)

	_, err := svc.Pool.Exec(ctx,
		`UPDATE entity_state_transitions SET reason = 'tampered' WHERE id = $1`, tr.ID)
	require.Error(t, err)
	require.True(t,
		strings.Contains(err.Error(), "append-only") ||
			strings.Contains(err.Error(), "permission denied"),
		"expected immutability enforcement, got: %v", err,
	)

	_, err = svc.Pool.Exec(ctx,
		`DELETE FROM entity_state_transitions WHERE id = $1`, tr.ID)
	require.Error(t, err)
	require.True(t,
		strings.Contains(err.Error(), "append-only") ||
			strings.Contains(err.Error(), "permission denied"),
		"expected immutability enforcement, got: %v", err,
	)
}
