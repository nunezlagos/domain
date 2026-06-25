

package audit

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNopRecorder_StoresCalls(t *testing.T) {
	r := &NopRecorder{}
	orgID := uuid.New()
	actorID := uuid.New()
	entID := uuid.New()
	err := r.Record(context.Background(), Event{
		OrganizationID: &orgID,
		ActorID:        &actorID,
		ActorType:      ActorUser,
		Action:         "user.created",
		EntityType:     "user",
		EntityID:       &entID,
		NewValues:      map[string]string{"name": "Bob"},
		IPAddress:      "1.2.3.4",
	})
	require.NoError(t, err)
	require.Len(t, r.Calls, 1)
	require.Equal(t, "user.created", r.Calls[0].Action)
	require.Equal(t, ActorUser, r.Calls[0].ActorType)
}

func TestActorType_Constants(t *testing.T) {
	require.Equal(t, ActorType("user"), ActorUser)
	require.Equal(t, ActorType("system"), ActorSystem)
	require.Equal(t, ActorType("api_key"), ActorAPIKey)
	require.Equal(t, ActorType("platform_admin"), ActorPlatformAdmin)
}

func TestNullIfEmpty(t *testing.T) {
	require.Nil(t, nullIfEmpty(""))
	require.Equal(t, "x", nullIfEmpty("x"))
}
