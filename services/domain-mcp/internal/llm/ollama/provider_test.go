package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

func testProvider(url string) *Provider {
	return &Provider{
		BaseURL:    url,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		Model:      "test-model",
	}
}

func opts(prompt string) llm.CompletionOptions {
	return llm.CompletionOptions{
		Messages: []llm.Message{{Role: "user", Content: prompt}},
	}
}

func TestComplete_Basic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/chat", r.URL.Path)
		var req chatReq
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "test-model", req.Model)
		require.False(t, req.Stream)
		_ = json.NewEncoder(w).Encode(chatResp{
			Model: "test-model", Done: true,
			Message:         chatMsg{Role: "assistant", Content: "hola"},
			PromptEvalCount: 7, EvalCount: 3,
		})
	}))
	defer srv.Close()

	res, err := testProvider(srv.URL).Complete(context.Background(), opts("hi"))
	require.NoError(t, err)
	require.Equal(t, "hola", res.Content)
	require.Equal(t, 10, res.Usage.TotalTokens)
}

func TestCompleteStream_Chunks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fl := w.(http.Flusher)
		for _, c := range []string{"ho", "la"} {
			_ = json.NewEncoder(w).Encode(chatResp{Message: chatMsg{Content: c}})
			fl.Flush()
		}
		_ = json.NewEncoder(w).Encode(chatResp{Done: true, PromptEvalCount: 2, EvalCount: 2})
		fl.Flush()
	}))
	defer srv.Close()

	ch, err := testProvider(srv.URL).CompleteStream(context.Background(), opts("hi"))
	require.NoError(t, err)

	var full string
	var done bool
	for chunk := range ch {
		full += chunk.Delta
		if chunk.Done {
			done = true
			require.NotNil(t, chunk.Usage)
			require.Equal(t, 4, chunk.Usage.TotalTokens)
		}
	}
	require.Equal(t, "hola", full)
	require.True(t, done)
}

func TestComplete_ModelNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"model 'nope' not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := testProvider(srv.URL).Complete(context.Background(), opts("hi"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "404")
}

func TestComplete_AutoPull_RetriesOnce(t *testing.T) {
	var pulls, chats atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/pull":
			pulls.Add(1)
			w.WriteHeader(http.StatusOK)
		case "/api/chat":
			if chats.Add(1) == 1 {
				http.Error(w, `{"error":"model 'test-model' not found"}`, http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(chatResp{
				Done: true, Message: chatMsg{Content: "ok tras pull"},
			})
		}
	}))
	defer srv.Close()

	p := testProvider(srv.URL)
	p.AutoPull = true
	res, err := p.Complete(context.Background(), opts("hi"))
	require.NoError(t, err)
	require.Equal(t, "ok tras pull", res.Content)
	require.Equal(t, int32(1), pulls.Load())
	require.Equal(t, int32(2), chats.Load())
}

func TestComplete_AutoPullDisabled_NoRetry(t *testing.T) {
	var pulls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/pull" {
			pulls.Add(1)
			return
		}
		http.Error(w, `{"error":"model not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := testProvider(srv.URL).Complete(context.Background(), opts("hi"))
	require.Error(t, err)
	require.Equal(t, int32(0), pulls.Load(), "sin AutoPull no debe pullear")
}

func TestComplete_ConnectionRefused(t *testing.T) {
	p := testProvider("http://127.0.0.1:1") // puerto cerrado
	_, err := p.Complete(context.Background(), opts("hi"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "ollama")
}

func TestComplete_CustomURL(t *testing.T) {
	hits := atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_ = json.NewEncoder(w).Encode(chatResp{Done: true, Message: chatMsg{Content: "x"}})
	}))
	defer srv.Close()

	_, err := testProvider(srv.URL).Complete(context.Background(), opts("hi"))
	require.NoError(t, err)
	require.Equal(t, int32(1), hits.Load(), "debe pegar a la URL custom")
}

func TestNew_EnvOverrides(t *testing.T) {
	t.Setenv("DOMAIN_OLLAMA_URL", "http://ollama.interno:11434")
	t.Setenv("DOMAIN_OLLAMA_AUTO_PULL", "true")
	p := New()
	require.Equal(t, "http://ollama.interno:11434", p.BaseURL)
	require.True(t, p.AutoPull)
	require.Equal(t, defaultTimeout, p.HTTPClient.Timeout)

	t.Setenv("DOMAIN_OLLAMA_URL", "")
	t.Setenv("DOMAIN_OLLAMA_HOST", "http://legacy:11434")
	p2 := New()
	require.Equal(t, "http://legacy:11434", p2.BaseURL, "fallback legacy DOMAIN_OLLAMA_HOST")
}

func TestComplete_ContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(chatResp{Done: true})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := testProvider(srv.URL).Complete(ctx, opts("hi"))
	require.Error(t, err)
}

// Sabotaje: respuesta no-JSON debe fallar graceful, no panic.
func TestSabotage_InvalidResponse_GracefulError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<<<not json>>>")
	}))
	defer srv.Close()

	_, err := testProvider(srv.URL).Complete(context.Background(), opts("hi"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid response")
}
