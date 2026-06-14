package httpserver

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// healthHandlerFunc retorna un http.Handler que responde con el
// status code dado a cualquier request.
func healthHandlerFunc(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte("OK"))
	})
}

// portFromURL extrae el port de una URL tipo "http://127.0.0.1:35443".
func portFromURL(t *testing.T, url string) int {
	t.Helper()
	// Formato esperado: http://127.0.0.1:PORT
	parts := strings.Split(url, ":")
	require.Len(t, parts, 3, "URL no tiene formato host:port: %s", url)
	port, err := strconv.Atoi(parts[2])
	require.NoError(t, err)
	return port
}
