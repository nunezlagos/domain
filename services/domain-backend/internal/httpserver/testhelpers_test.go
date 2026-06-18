package httpserver

import (
	"io"
	"log/slog"
)

// newTestLogger retorna un slog.Logger que escribe en el buffer dado.
// Usado por tests que necesitan inspeccionar el output del logger.
func newTestLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
