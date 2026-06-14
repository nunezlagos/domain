// HU-28.5 — tests del helper RecordOrLog: no propaga error, no panicea con
// recorder nil, y loggea cuando Record falla.
package audit

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type failingRecorder struct{ err error }

func (f *failingRecorder) Record(_ context.Context, _ Event) error { return f.err }

func TestRecordOrLog_NilRecorder_NoPanic(t *testing.T) {
	// No debe panicear ni loggear nada.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	RecordOrLog(context.Background(), nil, Event{Action: "x.y", EntityType: "x"})

	if buf.Len() != 0 {
		t.Errorf("expected no log output with nil recorder, got: %q", buf.String())
	}
}

func TestRecordOrLog_SuccessNoLog(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	rec := &NopRecorder{}
	RecordOrLog(context.Background(), rec, Event{Action: "user.created", EntityType: "user"})

	if buf.Len() != 0 {
		t.Errorf("did not expect logs on successful record, got: %q", buf.String())
	}
	if len(rec.Calls) != 1 {
		t.Errorf("expected 1 call captured, got %d", len(rec.Calls))
	}
}

func TestRecordOrLog_FailureLogs(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	id := uuid.New()
	rec := &failingRecorder{err: errors.New("db down")}
	RecordOrLog(context.Background(), rec, Event{
		Action:     "session.ended",
		EntityType: "session",
		EntityID:   &id,
	})

	out := buf.String()
	if !strings.Contains(out, "audit record failed") {
		t.Errorf("expected 'audit record failed' in log, got: %q", out)
	}
	if !strings.Contains(out, "action=session.ended") {
		t.Errorf("expected action attribute in log, got: %q", out)
	}
	if !strings.Contains(out, "level=WARN") {
		t.Errorf("expected WARN level, got: %q", out)
	}
}
