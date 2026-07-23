// Mock ACP agent: stand-in determinista de `opencode acp` para estresar el
// bridge (internal/agentbridge/acp) sin LLM ni red. Implementa el lado-agente
// del acp-go-sdk y responde initialize/session-new/prompt streameando chunks.
//
// Comportamiento inyectable por env (para buscarle los puntos débiles al bridge):
//   MOCK_REPLY           texto de cada chunk (default "pong")
//   MOCK_CHUNKS          cantidad de chunks a streamear (default 1)
//   MOCK_LATENCY_MS      delay antes del primer chunk
//   MOCK_CHUNK_DELAY_MS  delay entre chunks
//   MOCK_FAIL            none | crash-init | crash-prompt | hang | flood
//   MOCK_HANG_MS         duración del hang (default 600000)
package main

import (
	"context"
	"os"
	"strconv"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

type mockAgent struct {
	conn    *acp.AgentSideConnection
	reply   string
	chunks  int
	latency time.Duration
	between time.Duration
	fail    string
	hang    time.Duration
}

func (a *mockAgent) SetAgentConnection(c *acp.AgentSideConnection) { a.conn = c }

func (a *mockAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	if a.fail == "crash-init" {
		os.Exit(1)
	}
	return acp.InitializeResponse{
		ProtocolVersion:   acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{LoadSession: false},
	}, nil
}

func (a *mockAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: acp.SessionId("sess_mock")}, nil
}

func (a *mockAgent) Prompt(ctx context.Context, p acp.PromptRequest) (acp.PromptResponse, error) {
	switch a.fail {
	case "crash-prompt":
		os.Exit(1)
	case "hang":
		select {
		case <-time.After(a.hang):
		case <-ctx.Done():
			return acp.PromptResponse{}, ctx.Err()
		}
	}
	if a.latency > 0 {
		time.Sleep(a.latency)
	}
	n := a.chunks
	if a.fail == "flood" {
		n = 100000
	}
	for i := 0; i < n; i++ {
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: p.SessionId,
			Update:    acp.UpdateAgentMessageText(a.reply),
		}); err != nil {
			return acp.PromptResponse{}, err
		}
		if a.between > 0 {
			time.Sleep(a.between)
		}
	}
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (a *mockAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}
func (a *mockAgent) Cancel(context.Context, acp.CancelNotification) error { return nil }
func (a *mockAgent) Logout(context.Context, acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, nil
}
func (a *mockAgent) ListSessions(context.Context, acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, nil
}
func (a *mockAgent) ResumeSession(context.Context, acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, nil
}
func (a *mockAgent) CloseSession(context.Context, acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, nil
}
func (a *mockAgent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}
func (a *mockAgent) SetSessionConfigOption(context.Context, acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, nil
}

func main() {
	a := &mockAgent{
		reply:   or(os.Getenv("MOCK_REPLY"), "pong"),
		chunks:  envInt("MOCK_CHUNKS", 1),
		latency: time.Duration(envInt("MOCK_LATENCY_MS", 0)) * time.Millisecond,
		between: time.Duration(envInt("MOCK_CHUNK_DELAY_MS", 0)) * time.Millisecond,
		fail:    or(os.Getenv("MOCK_FAIL"), "none"),
		hang:    time.Duration(envInt("MOCK_HANG_MS", 600000)) * time.Millisecond,
	}
	conn := acp.NewAgentSideConnection(a, os.Stdout, os.Stdin)
	a.SetAgentConnection(conn)
	<-conn.Done()
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
