package acp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

func TestHandlerFS_ReadTextFile_InsideRoot_Ok(t *testing.T) {
	ws := newTestWorkspace(t)
	require.NoError(t, os.WriteFile(filepath.Join(ws.Root(), "ok.txt"), []byte("contenido"), 0o600))
	h := &handler{ws: ws, permissionMode: PermissionDenyAll}

	res, err := h.ReadTextFile(context.Background(), acpsdk.ReadTextFileRequest{Path: "ok.txt"})
	require.NoError(t, err)
	require.Equal(t, "contenido", res.Content)
}

func TestHandlerFS_ReadTextFile_OutsideRoot_Errors(t *testing.T) {
	ws := newTestWorkspace(t)
	h := &handler{ws: ws, permissionMode: PermissionDenyAll}

	_, err := h.ReadTextFile(context.Background(), acpsdk.ReadTextFileRequest{Path: "../../etc/passwd"})
	require.Error(t, err, "read fuera del root debe rechazarse")
}

func TestHandlerFS_WriteTextFile_DenyAll_Rejected(t *testing.T) {
	ws := newTestWorkspace(t)
	h := &handler{ws: ws, permissionMode: PermissionDenyAll}

	_, err := h.WriteTextFile(context.Background(), acpsdk.WriteTextFileRequest{Path: "x.txt", Content: "y"})
	require.ErrorIs(t, err, errUnsupported, "write debe estar deny-all por default")
}
