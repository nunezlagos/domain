package acp

import (
	"context"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

func TestHandler_RequestPermission_DeniesByDefault(t *testing.T) {
	h := &handler{}
	res, err := h.RequestPermission(context.Background(), acpsdk.RequestPermissionRequest{})
	require.NoError(t, err)
	require.NotNil(t, res.Outcome.Cancelled, "el permiso debe rechazarse (cancelled) por default")
	require.Nil(t, res.Outcome.Selected)
}

func TestHandler_FileOps_ReturnUnsupported(t *testing.T) {
	h := &handler{}
	_, rErr := h.ReadTextFile(context.Background(), acpsdk.ReadTextFileRequest{})
	require.ErrorIs(t, rErr, errUnsupported)
	_, wErr := h.WriteTextFile(context.Background(), acpsdk.WriteTextFileRequest{})
	require.ErrorIs(t, wErr, errUnsupported)
}

func TestHandler_Terminal_ReturnsUnsupported(t *testing.T) {
	h := &handler{}
	_, err := h.CreateTerminal(context.Background(), acpsdk.CreateTerminalRequest{})
	require.ErrorIs(t, err, errUnsupported)
}
