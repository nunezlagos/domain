package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
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

// stubS3ConTamano responde 200 declarando un Content-Length, que es de donde el HEAD saca
// el tamaño real del objeto (DOMAINSERV-224).
func stubS3ConTamano(t *testing.T, bytes int64) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.FormatInt(bytes, 10))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// El HEAD ya trae el ContentLength y ConfirmObject lo descartaba, así que size_bytes quedaba
// siendo el valor DECLARADO por el cliente, que nadie verificaba nunca. Medido en prod el
// 2026-08-03: la fila quedó con 96 bytes para un objeto de 79.
func TestClient_ConfirmObject_ObjetoPresente_DevuelveElTamanoReal(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: stubS3ConTamano(t, 79), Bucket: "test", Key: "k", Secret: "s"})

	exists, size, err := c.ConfirmObject(context.Background(), "test/key")

	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, int64(79), size, "el tamaño sale del HEAD, no de lo que declaró el cliente")
}

func TestClient_ConfirmObject_ObjetoAusente_ElTamanoEsCero(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: stubS3(t, http.StatusNotFound), Bucket: "test", Key: "k", Secret: "s"})

	exists, size, err := c.ConfirmObject(context.Background(), "test/key")

	require.NoError(t, err)
	require.False(t, exists)
	require.Zero(t, size, "sin objeto no hay tamaño que reportar")
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

// DOMAINSERV-213: WithPresignExpires recibe un time.Duration. Pasarle el entero
// de segundos daba 900ns, que se serializa como X-Amz-Expires=0 y nace expirada.
func TestClient_GenerateUploadURL_ExpiraEn15Minutos(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: "http://localhost:9000", Bucket: "test", Key: "k", Secret: "s"})

	raw, err := c.GenerateUploadURL(context.Background(), "test/key")
	require.NoError(t, err)

	require.Equal(t, "900", expiresParam(t, raw), "el PUT presignado debe durar 15 minutos en SEGUNDOS")
}

func TestClient_GenerateDownloadURL_ExpiraEn1Hora(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: "http://localhost:9000", Bucket: "test", Key: "k", Secret: "s"})

	raw, err := c.GenerateDownloadURL(context.Background(), "test/key")
	require.NoError(t, err)

	require.Equal(t, "3600", expiresParam(t, raw), "el GET presignado debe durar 1 hora en SEGUNDOS")
}

// DOMAINSERV-215: un 404 es la unica respuesta que significa "no esta".
func TestClient_ConfirmObject_ObjetoAusente_DevuelveFalseSinError(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: stubS3(t, http.StatusNotFound), Bucket: "test", Key: "k", Secret: "s"})

	exists, _, err := c.ConfirmObject(context.Background(), "test/key")

	require.NoError(t, err, "un objeto ausente es un caso normal, no un error")
	require.False(t, exists)
}

func TestClient_ConfirmObject_ObjetoPresente_DevuelveTrue(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: stubS3(t, http.StatusOK), Bucket: "test", Key: "k", Secret: "s"})

	exists, _, err := c.ConfirmObject(context.Background(), "test/key")

	require.NoError(t, err)
	require.True(t, exists)
}

func TestClient_ConfirmObject_AccesoDenegado_DevuelveError(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: stubS3(t, http.StatusForbidden), Bucket: "test", Key: "k", Secret: "s"})

	exists, _, err := c.ConfirmObject(context.Background(), "test/key")

	require.Error(t, err, "un 403 no es 'el objeto no existe': credenciales o politica mal")
	require.False(t, exists)
}

func TestClient_ConfirmObject_EndpointInalcanzable_DevuelveError(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: "http://127.0.0.1:1", Bucket: "test", Key: "k", Secret: "s"})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, _, err := c.ConfirmObject(ctx, "test/key")

	require.Error(t, err, "storage caido no puede reportarse como objeto ausente")
	require.False(t, exists)
}

func TestClient_ConfirmObject_ContextoCancelado_DevuelveError(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: stubS3(t, http.StatusOK), Bucket: "test", Key: "k", Secret: "s"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	exists, _, err := c.ConfirmObject(ctx, "test/key")

	require.Error(t, err)
	require.False(t, exists)
}

// DOMAINSERV-214: el endpoint que consume el cliente no es el que usa el server.
func TestClient_GenerateUploadURL_ConPublicEndpoint_FirmaContraElPublico(t *testing.T) {
	c := newTestClient(t, Config{
		Endpoint:       "http://minio:9000",
		PublicEndpoint: "https://storage.example.com",
		Bucket:         "test",
		Key:            "k",
		Secret:         "s",
	})

	raw, err := c.GenerateUploadURL(context.Background(), "test/key")
	require.NoError(t, err)

	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "storage.example.com", u.Host, "el cliente no puede resolver el host interno")
	require.Equal(t, "https", u.Scheme)
	require.Contains(t, u.Query().Get("X-Amz-SignedHeaders"), "host",
		"el host esta firmado: reescribirlo despues de firmar invalidaria SigV4")
}

func TestClient_GenerateDownloadURL_ConPublicEndpoint_FirmaContraElPublico(t *testing.T) {
	c := newTestClient(t, Config{
		Endpoint:       "http://minio:9000",
		PublicEndpoint: "https://storage.example.com",
		Bucket:         "test",
		Key:            "k",
		Secret:         "s",
	})

	raw, err := c.GenerateDownloadURL(context.Background(), "test/key")
	require.NoError(t, err)

	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "storage.example.com", u.Host)
}

func TestClient_GenerateUploadURL_SinPublicEndpoint_CaeAlInterno(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: "http://minio:9000", Bucket: "test", Key: "k", Secret: "s"})

	raw, err := c.GenerateUploadURL(context.Background(), "test/key")
	require.NoError(t, err)

	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "minio:9000", u.Host, "sin endpoint publico se preserva el comportamiento previo")
}

func TestClient_ConfirmObject_ConPublicEndpoint_UsaElInterno(t *testing.T) {
	interno := stubS3(t, http.StatusOK)
	c := newTestClient(t, Config{
		Endpoint:       interno,
		PublicEndpoint: "http://127.0.0.1:1",
		Bucket:         "test",
		Key:            "k",
		Secret:         "s",
	})

	exists, _, err := c.ConfirmObject(context.Background(), "test/key")

	require.NoError(t, err, "las operaciones del server van por el endpoint interno, no por el publico")
	require.True(t, exists)
}

// DOMAINSERV-216: path-style es requisito de MinIO y esta hardcodeado. Nada lo
// probaba, asi que el dia que alguien lo haga configurable nada va a avisar.
func TestClient_GenerateUploadURL_PoneElBucketEnElPath_NoEnElHost(t *testing.T) {
	c := newTestClient(t, Config{Endpoint: "http://minio:9000", Bucket: "domain-attachments", Key: "k", Secret: "s"})

	raw, err := c.GenerateUploadURL(context.Background(), "test/key")
	require.NoError(t, err)

	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "minio:9000", u.Host, "virtual-host style rompe MinIO: el bucket no puede ir en el host")
	require.Equal(t, "/domain-attachments/test/key", u.Path)
}

func TestClient_ConPublicEndpoint_MantienePathStyle(t *testing.T) {
	c := newTestClient(t, Config{
		Endpoint:       "http://minio:9000",
		PublicEndpoint: "https://storage.example.com",
		Bucket:         "domain-attachments",
		Key:            "k",
		Secret:         "s",
	})

	raw, err := c.GenerateDownloadURL(context.Background(), "test/key")
	require.NoError(t, err)

	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "storage.example.com", u.Host)
	require.Equal(t, "/domain-attachments/test/key", u.Path, "el cliente de presign tambien necesita path-style")
}

// Test estructura de tipos publicos para detectar breaking changes.
func TestClient_StructShape(t *testing.T) {
	c := &Client{S3: nil, Presign: nil, Bucket: "b"}
	require.Equal(t, "b", c.Bucket)
	var _ *s3.Client = c.S3
	var _ *s3.Client = c.Presign
}
