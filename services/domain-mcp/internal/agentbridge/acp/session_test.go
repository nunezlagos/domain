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
	return wireFakeMCP(t, fa, nil)
}

// wireFakeMCP conecta un Session con un McpServer opcional para ejercer el
// capability gating del path nativo.
func wireFakeMCP(t *testing.T, fa *fakeAgent, mcp *acpsdk.McpServer) *Session {
	t.Helper()
	clientConn, agentConn := net.Pipe()
	t.Cleanup(func() { _ = clientConn.Close(); _ = agentConn.Close() })
	fa.conn = acpsdk.NewAgentSideConnection(fa, agentConn, agentConn)
	return newSessionWithHandler(clientConn, clientConn, "/tmp", &handler{}, mcp)
}

func TestSession_Capability_HttpAbsent_Errors(t *testing.T) {
	mcp := buildMcpServer(Config{McpURL: "http://127.0.0.1:8000/mcp", McpToken: "domk_x"})
	sess := wireFakeMCP(t, &fakeAgent{reply: "x", httpCap: false}, mcp)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := sess.Prompt(ctx, "hola")
	require.Error(t, err, "McpURL set + agente sin capability http debe fallar explícito")
}

func TestSession_Capability_HttpPresent_Ok(t *testing.T) {
	mcp := buildMcpServer(Config{McpURL: "http://127.0.0.1:8000/mcp", McpToken: "domk_x"})
	sess := wireFakeMCP(t, &fakeAgent{reply: "respuesta", httpCap: true}, mcp)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := sess.Prompt(ctx, "hola")
	require.NoError(t, err)
	require.Equal(t, "respuesta", got)
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
