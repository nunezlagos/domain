package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/saargo/domain/internal/llm"
)

func TestProvider_Complete_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(chatResponse{
			Model: "gpt-4o",
			Choices: []struct {
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{Message: chatMessage{Role: "assistant", Content: "Hola"}, FinishReason: "stop"},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		})
	}))
	defer srv.Close()

	p := New("test-key")
	p.BaseURL = srv.URL
	resp, err := p.Complete(context.Background(), llm.CompletionOptions{
		Model: "gpt-4o", SystemPrompt: "Sé conciso",
		Messages: []llm.Message{{Role: "user", Content: "hola"}},
	})
	require.NoError(t, err)
	require.Equal(t, "Hola", resp.Content)
	require.Equal(t, "stop", resp.FinishReason)
	require.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestProvider_Complete_ToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		msg := chatMessage{Role: "assistant"}
		tc := chatToolCall{ID: "call_1", Type: "function"}
		tc.Function.Name = "search"
		tc.Function.Arguments = `{"q":"hola"}`
		msg.ToolCalls = []chatToolCall{tc}
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{Message: msg, FinishReason: "tool_calls"},
			},
		})
	}))
	defer srv.Close()

	p := New("k")
	p.BaseURL = srv.URL
	resp, err := p.Complete(context.Background(), llm.CompletionOptions{
		Tools: []llm.ToolDef{{Name: "search", Schema: map[string]any{"type": "object"}}},
	})
	require.NoError(t, err)
	require.Len(t, resp.ToolCalls, 1)
	require.Equal(t, "search", resp.ToolCalls[0].Name)
	require.Equal(t, "hola", resp.ToolCalls[0].Arguments["q"])
}

func TestProvider_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "invalid key", "type": "auth"},
		})
	}))
	defer srv.Close()

	p := New("k")
	p.BaseURL = srv.URL
	_, err := p.Complete(context.Background(), llm.CompletionOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid key")
}

func TestEmbedder_Dimensions(t *testing.T) {
	e := NewEmbedder(EmbedderConfig{APIKey: "k"})
	require.Equal(t, 1536, e.Dimensions())
}

func TestEmbedder_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/embeddings", r.URL.Path)
		embedding := make([]float64, 1536)
		for i := range embedding {
			embedding[i] = 0.1
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": embedding}},
		})
	}))
	defer srv.Close()

	e := NewEmbedder(EmbedderConfig{APIKey: "k", BaseURL: srv.URL})
	vec, err := e.Embed(context.Background(), "hola")
	require.NoError(t, err)
	require.Equal(t, 1536, len(vec))
}

// Sabotaje: sin APIKey, embed falla
func TestSabotage_Embedder_NoAPIKey(t *testing.T) {
	e := NewEmbedder(EmbedderConfig{})
	_, err := e.Embed(context.Background(), "x")
	require.Error(t, err)
}
