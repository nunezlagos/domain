package acp

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

// wireFake conecta un Session a un fakeAgent por net.Pipe (in-memory, sin
// subproceso ni red) y devuelve el Session listo para Prompt.
func wireFake(t *testing.T, fa *fakeAgent) *Session {
	t.Helper()
	clientConn, agentConn := net.Pipe()
	t.Cleanup(func() { _ = clientConn.Close(); _ = agentConn.Close() })
	fa.conn = acpsdk.NewAgentSideConnection(fa, agentConn, agentConn)
	return newSession(clientConn, clientConn, "/tmp")
}

func TestSession_Prompt_SingleChunk_ReturnsAgentText(t *testing.T) {
	sess := wireFake(t, &fakeAgent{reply: "respuesta del agente"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := sess.Prompt(ctx, "hola")
	require.NoError(t, err)
	require.Equal(t, "respuesta del agente", got)
}

func TestSession_Prompt_MultipleChunks_AccumulatesText(t *testing.T) {
	sess := wireFake(t, &fakeAgent{chunks: []string{"parte1 ", "parte2"}})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := sess.Prompt(ctx, "hola")
	require.NoError(t, err)
	require.Equal(t, "parte1 parte2", got)
}

func TestSession_Prompt_AgentError_Propagates(t *testing.T) {
	sess := wireFake(t, &fakeAgent{err: errors.New("boom")})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := sess.Prompt(ctx, "hola")
	require.Error(t, err)
}
