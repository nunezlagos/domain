package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

func chatOK(content string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{{Message: chatMessage{Role: "assistant", Content: content}, FinishReason: "stop"}},
		})
	}
}

func TestOpenAICompat_Complete_CustomBaseURL_HitsConfiguredEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		chatOK("pong")(w, r)
	}))
	defer srv.Close()

	p := NewWithBaseURL("k", srv.URL, "llama-3")
	require.Equal(t, srv.URL, p.BaseURL)
	require.Equal(t, "llama-3", p.Model)

	resp, err := p.Complete(context.Background(), llm.CompletionOptions{
		Messages: []llm.Message{{Role: "user", Content: "ping"}},
	})
	require.NoError(t, err)
	require.Equal(t, "pong", resp.Content)
	require.Equal(t, "/v1/chat/completions", gotPath)
}

func TestNewWithBaseURL_EmptyArgs_KeepsDefaults(t *testing.T) {
	p := NewWithBaseURL("k", "", "")
	require.Equal(t, defaultBaseURL, p.BaseURL)
	require.Equal(t, defaultModel, p.Model)
}

func TestRegisterOpenAICompat_ValidConfig_RegistersProviders(t *testing.T) {
	t.Setenv("GROQ_KEY", "gk")
	t.Setenv("TOGETHER_KEY", "tk")
	t.Setenv(CompatProvidersEnv, `[
		{"name":"groq","base_url":"https://api.groq.com/openai","api_key_env":"GROQ_KEY","model":"llama-3.1"},
		{"name":"together","base_url":"https://api.together.xyz","api_key_env":"TOGETHER_KEY","model":"mixtral"}
	]`)

	f := llm.NewFactory()
	n := RegisterOpenAICompat(f, nil, nil)
	require.Equal(t, 2, n)
	_, err := f.Get("groq")
	require.NoError(t, err)
	_, err = f.Get("together")
	require.NoError(t, err)
}

func TestRegisterOpenAICompat_ExistingProvidersUntouched(t *testing.T) {
	f := llm.NewFactory()
	existing := New("sk")
	f.Register("openai", existing)

	t.Setenv("X_KEY", "xk")
	t.Setenv(CompatProvidersEnv, `[{"name":"vllm","base_url":"http://localhost:8000","api_key_env":"X_KEY","model":"m"}]`)
	RegisterOpenAICompat(f, nil, nil)

	got, err := f.Get("openai")
	require.NoError(t, err)
	require.Same(t, existing, got)
	_, err = f.Get("vllm")
	require.NoError(t, err)
}

func TestRegisterOpenAICompat_MalformedJSON_SkipsWithoutPanic(t *testing.T) {
	t.Setenv(CompatProvidersEnv, `{not valid json`)
	f := llm.NewFactory()
	n := RegisterOpenAICompat(f, nil, nil)
	require.Equal(t, 0, n)
	require.Empty(t, f.List())
}

func TestRegisterOpenAICompat_ApiKeyNotLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	const secret = "super-secret-key-value"
	t.Setenv("SECRET_KEY", secret)
	t.Setenv(CompatProvidersEnv, `[{"name":"vllm","base_url":"http://localhost:8000","api_key_env":"SECRET_KEY","model":"m"}]`)

	f := llm.NewFactory()
	n := RegisterOpenAICompat(f, nil, logger)
	require.Equal(t, 1, n)
	require.NotContains(t, buf.String(), secret)
}
