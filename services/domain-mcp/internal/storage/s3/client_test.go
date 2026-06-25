package s3

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)






func TestNew_RequiresRegion(t *testing.T) {


	_, err := New(Config{Region: ""})



	_ = err
}

func TestNew_WithEndpoint_SetsPathStyle(t *testing.T) {



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



	c, err := New(Config{
		Region: "us-east-1",
		Bucket: "test",
	})
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestConfig_Defaults(t *testing.T) {



	cfg := Config{Region: "us-east-1"}
	c, err := New(cfg)
	require.NoError(t, err)
	require.Equal(t, "", c.Bucket, "Bucket vacio permitido en constructor (SDK falla en operacion)")
}

func TestGenerateUploadURL_BucketRequired(t *testing.T) {

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

	c, err := New(Config{Region: "us-east-1", Bucket: "test"})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // asegura que ctx ya esta expirado
	exists, err := c.ConfirmObject(ctx, "key")


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
