// Package s3 provides S3-compatible storage client with presigned URL support.
package s3

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Client wraps S3 SDK for presigned URL operations.
type Client struct {
	S3     *s3.Client
	Bucket string
}

// Config for S3 client.
type Config struct {
	Endpoint string // S3-compatible endpoint (e.g., http://localhost:9000 for MinIO)
	Region   string
	Bucket   string
	Key      string
	Secret   string
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

	s3Opts := func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	}

	return &Client{
		S3:     s3.NewFromConfig(awsCfg, s3Opts),
		Bucket: cfg.Bucket,
	}, nil
}

// GenerateUploadURL creates a presigned PUT URL valid for 15 minutes.
func (c *Client) GenerateUploadURL(ctx context.Context, key string) (string, error) {
	ps := s3.NewPresignClient(c.S3)
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
	ps := s3.NewPresignClient(c.S3)
	req, err := ps.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(time.Hour))
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}
	return req.URL, nil
}

// ConfirmObject checks if an object exists in S3 (HEAD).
func (c *Client) ConfirmObject(ctx context.Context, key string) (bool, error) {
	_, err := c.S3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("head object %q: %w", key, err)
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
