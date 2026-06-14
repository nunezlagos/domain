package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

func testProvider(url string) *Provider {
	return &Provider{
		APIKey: "test-key", BaseURL: url,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		Model:      "gemini-2.0-flash",
	}
}

func opts(prompt string) llm.CompletionOptions {
	return llm.CompletionOptions{
		SystemPrompt: "sos un asistente",
		Messages:     []llm.Message{{Role: "user", Content: prompt}},
		Temperature:  0.5, MaxTokens: 100,
	}
}

func TestComplete_Basic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, ":generateContent")
		require.Equal(t, "test-key", r.URL.Query().Get("key"))
		var req genReq
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.NotNil(t, req.SystemInstruction)
		require.Len(t, req.Contents, 1)
		require.Equal(t, "user", req.Contents[0].Role)

		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{{
				"content":      map[string]any{"role": "model", "parts": []map[string]any{{"text": "hola "}, {"text": "mundo"}}},
				"finishReason": "STOP",
			}},
			"usageMetadata": map[string]any{"promptTokenCount": 5, "candidatesTokenCount": 2, "totalTokenCount": 7},
		})
	}))
	defer srv.Close()

	res, err := testProvider(srv.URL).Complete(context.Background(), opts("hi"))
	require.NoError(t, err)
	require.Equal(t, "hola mundo", res.Content)
	require.Equal(t, "stop", res.FinishReason)
	require.Equal(t, 7, res.Usage.TotalTokens)
}

func TestComplete_RoleMapping_AssistantToModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req genReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		require.Equal(t, "model", req.Contents[1].Role)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{{"content": map[string]any{"parts": []map[string]any{{"text": "ok"}}}}},
		})
	}))
	defer srv.Close()

	o := opts("hi")
	o.Messages = append(o.Messages, llm.Message{Role: "assistant", Content: "previa"})
	_, err := testProvider(srv.URL).Complete(context.Background(), o)
	require.NoError(t, err)
}

func TestComplete_InvalidAPIKey_ClearError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": 400, "status": "INVALID_ARGUMENT", "message": "API key not valid"},
		})
	}))
	defer srv.Close()

	_, err := testProvider(srv.URL).Complete(context.Background(), opts("hi"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "API key not valid")
	require.Contains(t, err.Error(), "400")
}

func TestCompleteStream_Chunks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, ":streamGenerateContent")
		fl := w.(http.Flusher)
		for _, txt := range []string{"ho", "la"} {
			fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]any{
				"candidates": []map[string]any{{"content": map[string]any{"parts": []map[string]any{{"text": txt}}}}},
			}))
			fl.Flush()
		}
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]any{
			"candidates":    []map[string]any{},
			"usageMetadata": map[string]any{"promptTokenCount": 3, "candidatesTokenCount": 2, "totalTokenCount": 5},
		}))
		fl.Flush()
	}))
	defer srv.Close()

	ch, err := testProvider(srv.URL).CompleteStream(context.Background(), opts("hi"))
	require.NoError(t, err)
	var full string
	var done bool
	for c := range ch {
		full += c.Delta
		if c.Done {
			done = true
			require.NotNil(t, c.Usage)
			require.Equal(t, 5, c.Usage.TotalTokens)
		}
	}
	require.Equal(t, "hola", full)
	require.True(t, done)
}

// Sabotaje: respuesta no-JSON → error graceful, no panic.
func TestSabotage_MalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "{{{not json")
	}))
	defer srv.Close()

	_, err := testProvider(srv.URL).Complete(context.Background(), opts("hi"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid response")
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
