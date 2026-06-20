package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

func TestProvider_Complete_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/messages", r.URL.Path)
		require.Equal(t, "test-key", r.Header.Get("x-api-key"))
		require.NotEmpty(t, r.Header.Get("anthropic-version"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responseBody{
			ID: "msg_xxx", Model: "claude-sonnet-4-5", StopReason: "end_turn",
			Content: []responseContentBlock{
				{Type: "text", Text: "Hola"},
			},
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{InputTokens: 10, OutputTokens: 5},
		})
	}))
	defer srv.Close()

	p := New("test-key")
	p.BaseURL = srv.URL

	resp, err := p.Complete(context.Background(), llm.CompletionOptions{
		Model:    "claude-sonnet-4-5",
		Messages: []llm.Message{{Role: "user", Content: "hola"}},
	})
	require.NoError(t, err)
	require.Equal(t, "Hola", resp.Content)
	require.Equal(t, "stop", resp.FinishReason)
	require.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestProvider_Complete_ToolUse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(responseBody{
			ID: "msg_x", Model: "claude", StopReason: "tool_use",
			Content: []responseContentBlock{
				{Type: "text", Text: "voy a usar la tool"},
				{Type: "tool_use", ID: "tu_1", Name: "search", Input: map[string]any{"q": "hola"}},
			},
		})
	}))
	defer srv.Close()

	p := New("k")
	p.BaseURL = srv.URL
	resp, err := p.Complete(context.Background(), llm.CompletionOptions{
		Model:    "x",
		Messages: []llm.Message{{Role: "user", Content: "buscá"}},
		Tools: []llm.ToolDef{{Name: "search", Schema: map[string]any{"type": "object"}}},
	})
	require.NoError(t, err)
	require.Equal(t, "tool_use", resp.FinishReason)
	require.Len(t, resp.ToolCalls, 1)
	require.Equal(t, "search", resp.ToolCalls[0].Name)
	require.Equal(t, "hola", resp.ToolCalls[0].Arguments["q"])
}

func TestProvider_Complete_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(errorResponse{
			Type: "error",
			Error: struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}{Type: "invalid_request", Message: "bad model"},
		})
	}))
	defer srv.Close()

	p := New("k")
	p.BaseURL = srv.URL
	_, err := p.Complete(context.Background(), llm.CompletionOptions{Model: "x"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad model")
}

func TestProvider_BuildRequest_MessageWithToolResult(t *testing.T) {
	p := New("k")
	body := p.buildRequest(llm.CompletionOptions{
		Messages: []llm.Message{
			{Role: "user", Content: "buscá"},
			{Role: "assistant", Content: "ok", ToolCalls: []llm.ToolCall{
				{ID: "tu_1", Name: "search", Arguments: map[string]any{"q": "x"}},
			}},
			{Role: "tool", ToolCallID: "tu_1", Content: "result"},
		},
	}, false)

	require.Len(t, body.Messages, 3)
	// El third message (tool) se convierte a role=user con tool_result block
	require.Equal(t, "user", body.Messages[2].Role)
	require.Equal(t, "tool_result", body.Messages[2].Content[0].Type)
	require.Equal(t, "tu_1", body.Messages[2].Content[0].ToolUseID)
}

func TestNormalizeStopReason(t *testing.T) {
	require.Equal(t, "stop", normalizeStopReason("end_turn"))
	require.Equal(t, "length", normalizeStopReason("max_tokens"))
	require.Equal(t, "tool_use", normalizeStopReason("tool_use"))
}

// Sabotaje: sin APIKey configurada en VoyageEmbedder, retorna error.
func TestSabotage_VoyageEmbedder_NoAPIKey(t *testing.T) {
	e := NewEmbedder(EmbedderConfig{})
	_, err := e.Embed(context.Background(), "hola")
	require.Error(t, err)
}
