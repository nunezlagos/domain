package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

// fakeServe monta un opencode serve falso: POST /session -> {id} y
// POST /session/:id/message -> {parts:[{type:text,text:reply}]}. Captura el
// último body de message para asserts.
func fakeServe(t *testing.T, reply string, capture *map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "ses_1"})
	})
	mux.HandleFunc("/session/ses_1/message", func(w http.ResponseWriter, r *http.Request) {
		if capture != nil {
			_ = json.NewDecoder(r.Body).Decode(capture)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"info":  map[string]any{"role": "assistant"},
			"parts": []any{map[string]any{"type": "text", "text": reply}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestProvider_Name_IsOpencode(t *testing.T) {
	require.Equal(t, "opencode", New("http://x", nil, "", "").Name())
}

func TestProvider_Complete_ReturnsAssistantText(t *testing.T) {
	var body map[string]any
	srv := fakeServe(t, "respuesta del cerebro", &body)
	p := New(srv.URL, srv.Client(), "", "")

	resp, err := p.Complete(context.Background(), llm.CompletionOptions{
		Model:        "grok-free",
		SystemPrompt: "sos un juez",
		Messages:     []llm.Message{{Role: "user", Content: "evaluá esto"}},
	})
	require.NoError(t, err)
	require.Equal(t, "respuesta del cerebro", resp.Content)
	require.Equal(t, "grok-free", resp.Model)
	require.Equal(t, "stop", resp.FinishReason)
	require.Equal(t, "sos un juez", body["system"])
	require.Equal(t, "grok-free", body["model"])
}

func TestProvider_Complete_ServerError_Propagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	_, err := New(srv.URL, srv.Client(), "", "").Complete(context.Background(), llm.CompletionOptions{})
	require.Error(t, err)
}

func TestProvider_CompleteStream_YieldsContentThenDone(t *testing.T) {
	srv := fakeServe(t, "hola", nil)
	ch, err := New(srv.URL, srv.Client(), "", "").CompleteStream(context.Background(), llm.CompletionOptions{})
	require.NoError(t, err)

	var got strings.Builder
	done := false
	for c := range ch {
		got.WriteString(c.Delta)
		done = done || c.Done
	}
	require.Equal(t, "hola", got.String())
	require.True(t, done)
}
