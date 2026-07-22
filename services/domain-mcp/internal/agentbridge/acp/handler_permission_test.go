package acp

import (
	"context"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

func TestHandlerPermission_RequestPermission_Default_DenyAll(t *testing.T) {
	h := &handler{}
	res, err := h.RequestPermission(context.Background(), acpsdk.RequestPermissionRequest{})
	require.NoError(t, err)
	require.NotNil(t, res.Outcome.Cancelled, "el permiso debe rechazarse (deny-all) por default")
	require.Nil(t, res.Outcome.Selected)
}
