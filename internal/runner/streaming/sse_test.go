package streaming_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/runner/streaming"
)

func TestStream_BasicChunks(t *testing.T) {
	rec := httptest.NewRecorder()
	s, err := streaming.New(rec)
	require.NoError(t, err)
	defer s.Close()

	require.NoError(t, s.Send(streaming.Chunk{Type: streaming.EventStarted, RunID: "r1"}))
	require.NoError(t, s.Send(streaming.Chunk{Type: streaming.EventChunk, Data: "hola"}))
	require.NoError(t, s.Send(streaming.Chunk{Type: streaming.EventCompleted}))

	body := rec.Body.String()
	require.Contains(t, body, "event: started")
	require.Contains(t, body, "event: chunk")
	require.Contains(t, body, "event: completed")
	require.Contains(t, body, "\"hola\"")
}

func TestStream_HeadersSet(t *testing.T) {
	rec := httptest.NewRecorder()
	_, err := streaming.New(rec)
	require.NoError(t, err)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	require.Equal(t, "no-cache, no-store, must-revalidate", rec.Header().Get("Cache-Control"))
}

func TestStream_NoFlusher(t *testing.T) {
	// Wrap recorder en algo que no expone Flusher
	w := struct{ http.ResponseWriter }{httptest.NewRecorder()}
	_, err := streaming.New(w)
	require.Error(t, err)
}

func TestStream_Pump(t *testing.T) {
	rec := httptest.NewRecorder()
	s, err := streaming.New(rec)
	require.NoError(t, err)
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	in := make(chan streaming.Chunk, 3)
	in <- streaming.Chunk{Type: streaming.EventChunk, Data: "a"}
	in <- streaming.Chunk{Type: streaming.EventChunk, Data: "b"}
	close(in)
	_ = s.Pump(ctx, in)

	body := rec.Body.String()
	require.True(t, strings.Count(body, "event: chunk") == 2)
}

// Sabotaje: post-Close, Send debe rechazar (no crashear / no escribir basura).
func TestSabotage_SendAfterClose(t *testing.T) {
	rec := httptest.NewRecorder()
	s, err := streaming.New(rec)
	require.NoError(t, err)
	s.Close()
	err = s.Send(streaming.Chunk{Type: streaming.EventChunk, Data: "post-close"})
	require.Error(t, err)
	require.NotContains(t, rec.Body.String(), "post-close")
}
