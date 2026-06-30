// HU-28.5 — verifica que writeJSON loggea el error de encode en vez de
// tragarlo silenciosamente. Inyectamos un body no-marshalable (channel) y
// confirmamos que el logger global recibe un mensaje.
package handler

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSON_LogsEncodeError(t *testing.T) {

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	rec := httptest.NewRecorder()

	writeJSON(rec, 200, map[string]any{"bad": make(chan int)})

	logs := buf.String()
	if !strings.Contains(logs, "failed to encode response") {
		t.Errorf("expected log message 'failed to encode response', got: %q", logs)
	}
	if !strings.Contains(logs, "level=ERROR") {
		t.Errorf("expected ERROR level, got: %q", logs)
	}
}

func TestWriteJSON_NoLogOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	rec := httptest.NewRecorder()
	writeJSON(rec, 200, map[string]any{"ok": true})

	if strings.Contains(buf.String(), "failed to encode response") {
		t.Errorf("did not expect error log on successful encode, got: %q", buf.String())
	}


	_ = context.Background()
}
