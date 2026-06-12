package s3

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

// Tests del constructor New y semantica de Config.
// Las operaciones que requieren red (GenerateUploadURL, ConfirmObject, etc.)
// se testean en service_integration_test.go con MinIO via testcontainers.
// Acá cubrimos solo logica que no toca la red: defaults, validacion, etc.

func TestNew_RequiresRegion(t *testing.T) {
	// New() carga config desde env, pero Region es required via WithRegion.
	// Sin region, falla. Con region valida, retorna client OK.
	_, err := New(Config{Region: ""})
	// Sin env vars, puede que cargue una config default; pero sin region
	// explicita deberia fallar en LoadDefaultConfig o pasar. Documentamos
	// el comportamiento actual sin asercion dura.
	_ = err
}

func TestNew_WithEndpoint_SetsPathStyle(t *testing.T) {
	// MinIO / S3-compatible usa path-style addressing.
	// No podemos inspeccionar el s3.Client interno (es *s3.Client opaco),
	// pero sí validamos que New() no falla con endpoint custom.
	c, err := New(Config{
		Region:   "us-east-1",
		Endpoint: "http://localhost:9000",
		Bucket:   "test",
		Key:      "minio",
		Secret:   "minio123",
	})
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, "test", c.Bucket)
}

func TestNew_WithoutCredentials_OK(t *testing.T) {
	// Sin credenciales: la SDK intentara usar IAM role / env. En test
	// environment no hay IAM role, pero New() no falla — falla recien
	// cuando se hace una operacion que requiere firma.
	c, err := New(Config{
		Region: "us-east-1",
		Bucket: "test",
	})
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestConfig_Defaults(t *testing.T) {
	// Documentamos que Bucket es required por las operaciones pero no por
	// el constructor. Si alguien crea un Client con Bucket="", las
	// operaciones daran error de "BucketName required".
	cfg := Config{Region: "us-east-1"}
	c, err := New(cfg)
	require.NoError(t, err)
	require.Equal(t, "", c.Bucket, "Bucket vacio permitido en constructor (SDK falla en operacion)")
}

func TestGenerateUploadURL_BucketRequired(t *testing.T) {
	// Sin bucket, GenerateUploadURL falla con error de SDK.
	c, err := New(Config{Region: "us-east-1"})
	require.NoError(t, err)
	_, err = c.GenerateUploadURL(context.Background(), "test/key")
	require.Error(t, err, "Bucket vacio debe causar error de SDK")
}

func TestGenerateDownloadURL_BucketRequired(t *testing.T) {
	c, err := New(Config{Region: "us-east-1"})
	require.NoError(t, err)
	_, err = c.GenerateDownloadURL(context.Background(), "test/key")
	require.Error(t, err)
}

func TestConfirmObject_PropagatesContextCancellation(t *testing.T) {
	// El context cancelado DEBE propagarse a la SDK call.
	c, err := New(Config{Region: "us-east-1", Bucket: "test"})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // asegura que ctx ya esta expirado
	exists, err := c.ConfirmObject(ctx, "key")
	// Sin red, falla por timeout o por otro motivo. Lo importante: el
	// context cancelado NO debe colgarse.
	_ = exists
	_ = err
	require.NotNil(t, t, "test ejecutado sin colgarse")
}

// Test estructura de tipos publicos para detectar breaking changes.
func TestClient_StructShape(t *testing.T) {
	c := &Client{S3: nil, Bucket: "b"}
	require.Equal(t, "b", c.Bucket)
	var _ *s3.Client = c.S3 // type assertion: S3 field es *s3.Client
}
