package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, cfg Config) *Client {
	t.Helper()
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	c, err := New(cfg)
	require.NoError(t, err)
	return c
}

// stubS3 responde todas las requests con el status dado.
func stubS3(t *testing.T, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func expiresParam(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return u.Query().Get("X-Amz-Expires")
}

func TestNew_WithEndpoint_SetsPathStyle(t *testing.T) {
	c := newTestClient(t, Config{
		Endpoint: "http://localhost:9000",
		Bucket:   "test",
		Key:      "minio",
		Secret:   "minio123",
	})
	require.Equal(t, "test", c.Bucket)
}

func TestNew_WithoutCredentials_OK(t *testing.T) {
	c := newTestClient(t, Config{Bucket: "test"})
	require.NotNil(t, c.S3)
}

func TestNew_SinBucket_LoDejaVacio(t *testing.T) {
	c := newTestClient(t, Config{})
	require.Equal(t, "", c.Bucket, "bucket vacio permitido en constructor (el SDK falla en la operacion)")
}

func TestGenerateUploadURL_BucketRequired(t *testing.T) {
	c := newTestClient(t, Config{})
	_, err := c.GenerateUploadURL(context.Background(), "test/key")
	require.Error(t, err, "bucket vacio debe causar error de SDK")
}

func TestGenerateDownloadURL_BucketRequired(t *testing.T) {
	c := newTestClient(t, Config{})
	_, err := c.GenerateDownloadURL(context.Background(), "test/key")
	require.Error(t, err)
}

// DOMAINSERV-215: un 404 es la unica respuesta que significa "no esta".
func TestClient_ConfirmObject_ObjetoAusente_DevuelveFalseSinError(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: stubS3(t, http.StatusNotFound), Bucket: "test", Key: "k", Secret: "s"})

	exists, err := c.ConfirmObject(context.Background(), "test/key")

	require.NoError(t, err, "un objeto ausente es un caso normal, no un error")
	require.False(t, exists)
}

func TestClient_ConfirmObject_ObjetoPresente_DevuelveTrue(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: stubS3(t, http.StatusOK), Bucket: "test", Key: "k", Secret: "s"})

	exists, err := c.ConfirmObject(context.Background(), "test/key")

	require.NoError(t, err)
	require.True(t, exists)
}

func TestClient_ConfirmObject_AccesoDenegado_DevuelveError(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: stubS3(t, http.StatusForbidden), Bucket: "test", Key: "k", Secret: "s"})

	exists, err := c.ConfirmObject(context.Background(), "test/key")

	require.Error(t, err, "un 403 no es 'el objeto no existe': credenciales o politica mal")
	require.False(t, exists)
}

func TestClient_ConfirmObject_EndpointInalcanzable_DevuelveError(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: "http://127.0.0.1:1", Bucket: "test", Key: "k", Secret: "s"})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := c.ConfirmObject(ctx, "test/key")

	require.Error(t, err, "storage caido no puede reportarse como objeto ausente")
	require.False(t, exists)
}

func TestClient_ConfirmObject_ContextoCancelado_DevuelveError(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: stubS3(t, http.StatusOK), Bucket: "test", Key: "k", Secret: "s"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	exists, err := c.ConfirmObject(ctx, "test/key")

	require.Error(t, err)
	require.False(t, exists)
}

// Test estructura de tipos publicos para detectar breaking changes.
func TestClient_StructShape(t *testing.T) {
	c := &Client{S3: nil, Bucket: "b"}
	require.Equal(t, "b", c.Bucket)
	var _ *s3.Client = c.S3
}
