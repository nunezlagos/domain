// Package s3 provides S3-compatible storage client with presigned URL support.
package s3

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Client wraps S3 SDK for presigned URL operations.
type Client struct {
	S3      *s3.Client
	Presign *s3.Client
	Bucket  string
	// endpointFirmado es la base con la que Presign firma: el publico si esta seteado, el
	// interno si no. Se guarda porque el SDK no lo expone y el guard de DOMAINSERV-214 lo
	// necesita para decidir si la URL es alcanzable por un cliente externo.
	endpointFirmado string
}

// Config for S3 client.
type Config struct {
	Endpoint string // S3-compatible endpoint (e.g., http://localhost:9000 for MinIO)
	// PublicEndpoint es el endpoint que alcanza el CLIENTE. Vacio ⇒ se usa Endpoint.
	PublicEndpoint string
	Region         string
	Bucket         string
	Key            string
	Secret         string
}

// New creates an S3 client. If Endpoint is set, uses path-style addressing (MinIO compatible).
func New(cfg Config) (*Client, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}
	if cfg.Key != "" && cfg.Secret != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.Key, cfg.Secret, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	internal := s3.NewFromConfig(awsCfg, endpointOpts(cfg.Endpoint))
	presign := internal
	firmado := cfg.Endpoint
	if cfg.PublicEndpoint != "" {
		presign = s3.NewFromConfig(awsCfg, endpointOpts(cfg.PublicEndpoint))
		firmado = cfg.PublicEndpoint
	}

	return &Client{
		S3:              internal,
		Presign:         presign,
		Bucket:          cfg.Bucket,
		endpointFirmado: firmado,
	}, nil
}

// esHostDeRedInterna reconoce un host que solo resuelve dentro de la red de Docker, o sea
// un nombre de servicio del compose.
//
// El criterio NO puede ser "PublicEndpoint esta vacio": en desarrollo el vacio es legitimo,
// porque ahi el endpoint interno ya es alcanzable por el cliente (localhost).
func esHostDeRedInterna(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "" || host == "localhost" || net.ParseIP(host) != nil {
		return false
	}
	// un FQDN publico lleva al menos un punto; un nombre de servicio del compose, no
	return !strings.Contains(host, ".")
}

// validarEndpointFirmado corta la emision de URLs que el cliente no puede resolver.
//
// El defecto medido en prod (DOMAINSERV-214) no era una URL mal formada: salia perfecta,
// firmada y con expiracion valida, apuntando a minio:9000. El cliente descubria el problema
// recien al hacer el PUT, lejos de la causa. La policy default-de-env-var-va-en-el-compose
// exige que la ausencia de un valor propio del ambiente falle ruidosamente y no degrade.
//
// Solo aplica a las URLs que se entregan al CLIENTE: el server sigue operando por el
// endpoint interno.
func (c *Client) validarEndpointFirmado() error {
	if !esHostDeRedInterna(c.endpointFirmado) {
		return nil
	}
	return fmt.Errorf(
		"el endpoint que se firma para el cliente (%q) solo resuelve dentro de la red de Docker: "+
			"setear DOMAIN_S3_PUBLIC_ENDPOINT con una URL alcanzable desde afuera, y exponer el "+
			"storage para que lo sea (DOMAINSERV-214)", c.endpointFirmado)
}

func endpointOpts(endpoint string) func(*s3.Options) {
	return func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			// path-style es requisito de MinIO, no una preferencia: no resuelve
			// bucket.host. No hacerlo configurable (DOMAINSERV-216)
			o.UsePathStyle = true
		}
	}
}

// GenerateUploadURL creates a presigned PUT URL valid for 15 minutes.
func (c *Client) GenerateUploadURL(ctx context.Context, key string) (string, error) {
	if err := c.validarEndpointFirmado(); err != nil {
		return "", err
	}
	ps := s3.NewPresignClient(c.Presign)
	req, err := ps.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}
	return req.URL, nil
}

// GenerateDownloadURL creates a presigned GET URL valid for 1 hour.
func (c *Client) GenerateDownloadURL(ctx context.Context, key string) (string, error) {
	if err := c.validarEndpointFirmado(); err != nil {
		return "", err
	}
	ps := s3.NewPresignClient(c.Presign)
	req, err := ps.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(time.Hour))
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}
	return req.URL, nil
}

// ConfirmObject checks if an object exists in S3 (HEAD) y devuelve su tamaño REAL.
//
// El tamaño viaja porque el HEAD ya lo trae: descartarlo dejaba a size_bytes siendo el valor
// que declaró el cliente en init_upload, que nadie verificaba nunca (DOMAINSERV-224).
func (c *Client) ConfirmObject(ctx context.Context, key string) (bool, int64, error) {
	out, err := c.S3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		var size int64
		if out.ContentLength != nil {
			size = *out.ContentLength
		}
		return true, size, nil
	}
	if isNotFound(err) {
		return false, 0, nil
	}
	return false, 0, fmt.Errorf("head object %q: %w", key, err)
}

// isNotFound distingue "el objeto no esta" de cualquier otra falla. El SDK
// mapea el 404 de HEAD por status, no por el body, asi que MinIO cae aca igual.
func isNotFound(err error) bool {
	var nf *types.NotFound
	return errors.As(err, &nf)
}

// DeleteObject removes an object from S3.
func (c *Client) DeleteObject(ctx context.Context, key string) error {
	_, err := c.S3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}
