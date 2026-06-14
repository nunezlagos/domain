// issue-02.6 activity-log unit tests.

package activity

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestVisibility_Constants(t *testing.T) {
	require.Equal(t, Visibility("public"), VisPublic)
	require.Equal(t, Visibility("org"), VisOrg)
	require.Equal(t, Visibility("project"), VisProject)
	require.Equal(t, Visibility("private"), VisPrivate)
}

func TestNopRecorder_StoresEvents(t *testing.T) {
	r := &NopRecorder{}
	orgID := uuid.New()
	id, err := r.Record(context.Background(), Event{
		OrganizationID: orgID,
		Action:         "observation.created",
		EntityType:     "observation",
		Summary:        "Alice creó observation",
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)
	require.Len(t, r.Calls, 1)
	require.Equal(t, "observation.created", r.Calls[0].Action)
}

func TestNopRecorder_MultipleEvents(t *testing.T) {
	r := &NopRecorder{}
	for i := 0; i < 3; i++ {
		_, err := r.Record(context.Background(), Event{
			OrganizationID: uuid.New(),
			Action:         "test",
			EntityType:     "x",
			Summary:        "s",
		})
		require.NoError(t, err)
	}
	require.Len(t, r.Calls, 3)
}
