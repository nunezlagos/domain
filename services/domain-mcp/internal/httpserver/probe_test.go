package httpserver

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProbeHealth_OK(t *testing.T) {
	srv := httptest.NewServer(healthHandlerFunc(200))
	defer srv.Close()



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


	err := ProbeHealth(1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "health probe failed")
}
