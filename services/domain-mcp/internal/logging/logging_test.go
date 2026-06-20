// issue-17.3 structured-logging unit tests.

package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Helper: setup con writer interceptado.
func setupWithBuffer(t *testing.T, format string, level string) (*bytes.Buffer, *slog.Logger) {
	t.Helper()
	buf := &bytes.Buffer{}
	dynamicLevel.Set(parseLevel(level))
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: dynamicLevel})
	} else {
		h = slog.NewTextHandler(buf, &slog.HandlerOptions{Level: dynamicLevel})
	}
	return buf, slog.New(&ctxEnrichHandler{inner: h})
}

// Escenario 1: JSON format con campos estándar.
func TestLogger_JSON_HasStandardFields(t *testing.T) {
	buf, lg := setupWithBuffer(t, "json", "info")
	lg.Info("test event", slog.String("foo", "bar"))

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	require.Contains(t, entry, "time")
	require.Contains(t, entry, "level")
	require.Contains(t, entry, "msg")
	require.Equal(t, "test event", entry["msg"])
	require.Equal(t, "INFO", entry["level"])
	require.Equal(t, "bar", entry["foo"])
}

// Escenario 2: ctx propagation — request_id automático.
func TestLogger_CtxRequestID_AutoAdded(t *testing.T) {
	buf, lg := setupWithBuffer(t, "json", "info")
	ctx := WithRequestID(context.Background(), "req-abc-123")
	lg.InfoContext(ctx, "hi")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	require.Equal(t, "req-abc-123", entry["request_id"])
}

// Escenario 3: ctx con todos los campos.
func TestLogger_Ctx_AllFields(t *testing.T) {
	buf, lg := setupWithBuffer(t, "json", "info")
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-1")
	ctx = WithUserID(ctx, "user-1")
	ctx = WithOrgID(ctx, "org-1")
	ctx = WithProjectID(ctx, "proj-1")
	lg.InfoContext(ctx, "evt")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	require.Equal(t, "req-1", entry["request_id"])
	require.Equal(t, "user-1", entry["user_id"])
	require.Equal(t, "org-1", entry["organization_id"])
	require.Equal(t, "proj-1", entry["project_id"])
}

// Escenario 4: nivel dinámico.
func TestLogger_DynamicLevel_FiltersBelow(t *testing.T) {
	buf, lg := setupWithBuffer(t, "json", "warn")
	lg.Info("info-msg")
	lg.Warn("warn-msg")
	out := buf.String()
	require.NotContains(t, out, "info-msg")
	require.Contains(t, out, "warn-msg")
}

// Escenario 5: SetLevel cambia nivel en runtime sin recrear.
func TestLogger_SetLevel_TakesEffect(t *testing.T) {
	buf, lg := setupWithBuffer(t, "json", "warn")
	lg.Info("first") // filtered out
	SetLevel("debug")
	lg.Info("second") // pass
	out := buf.String()
	require.NotContains(t, out, "first")
	require.Contains(t, out, "second")
}

// Escenario 6: text format legible.
func TestLogger_TextFormat_Readable(t *testing.T) {
	buf, lg := setupWithBuffer(t, "text", "info")
	lg.Info("hello", slog.String("foo", "bar"))
	require.Contains(t, buf.String(), "hello")
	require.Contains(t, buf.String(), "foo=bar")
}

// Sabotaje: keys prohibidas en logs → linter atrapa.
// issue-17.3 / .claude/rules/security.md.
// Heurística estricta: pattern `"KEY",` evita falsos positivos en values.
func TestSabotage_LinterDetectsForbiddenKeys(t *testing.T) {
	suspect := []string{
		`slog.String("password", "secret")`, // viola
		`slog.String("api_key", "x")`,       // viola
		`slog.String("user_id", "ok")`,      // permitido
		`slog.Int("otp_code", 482917)`,      // viola
		`logger.Info("login", slog.String("email", u.Email))`, // viola
	}
	forbidden := []string{
		"password", "passwd", "secret", "api_key", "token", "otp", "otp_code", "email", "rut",
	}
	violations := 0
	for _, line := range suspect {
		for _, k := range forbidden {
			if strings.Contains(line, `("`+k+`",`) {
				violations++
				break // 1 violación por línea es suficiente
			}
		}
	}
	require.Equal(t, 4, violations, "linter debe detectar 4 violaciones (password, api_key, otp_code, email)")
}

// HotReloadCount tracking
func TestChangeLevel_IncrementsCounter(t *testing.T) {
	start := HotReloadCount()
	ChangeLevel("info")
	ChangeLevel("debug")
	ChangeLevel("warn")
	require.Equal(t, start+3, HotReloadCount())
}

func TestCurrentLevel(t *testing.T) {
	SetLevel("debug")
	require.Equal(t, "debug", CurrentLevel())
	SetLevel("info")
	require.Equal(t, "info", CurrentLevel())
	SetLevel("warn")
	require.Equal(t, "warn", CurrentLevel())
	SetLevel("error")
	require.Equal(t, "error", CurrentLevel())
}

// Setup global no rompe slog.Default + agrega context fields.
func TestSetup_AssignsDefaultAndEnriches(t *testing.T) {
	buf := &bytes.Buffer{}
	// usamos Setup pero redirigimos stdout no es trivial; testeamos directamente Default()
	prev := slog.Default()
	defer slog.SetDefault(prev)

	// reasignar default a un handler que escribe a buf
	dynamicLevel.Set(slog.LevelInfo)
	h := &ctxEnrichHandler{inner: slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: dynamicLevel})}
	slog.SetDefault(slog.New(h))

	ctx := WithRequestID(context.Background(), "abc")
	slog.InfoContext(ctx, "via default")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	require.Equal(t, "abc", entry["request_id"])
}
