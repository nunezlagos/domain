package httpserver

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProbeHealth_OK(t *testing.T) {
	srv := httptest.NewServer(healthHandlerFunc(200))
	defer srv.Close()

	// httptest.Server.URL tiene el formato "http://127.0.0.1:PORT".
	// Extraemos el port.
	port := portFromURL(t, srv.URL)

	err := ProbeHealth(port)
	require.NoError(t, err)
}

func TestProbeHealth_Non200(t *testing.T) {
	srv := httptest.NewServer(healthHandlerFunc(500))
	defer srv.Close()

	port := portFromURL(t, srv.URL)
	err := ProbeHealth(port)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
}

func TestProbeHealth_ConnectionRefused(t *testing.T) {
	// Pedir un port random que no está escuchando. Use 1 (privileged,
	// casi siempre cerrado).
	err := ProbeHealth(1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "health probe failed")
}
