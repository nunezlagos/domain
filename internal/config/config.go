// Package config — HU-01.2 config-system.
//
// Carga config desde env vars con prefijo DOMAIN_*. Validación strict + defaults.
// Una sola Load() al boot; valores no se recargan salvo via HU-27.3 hot-reload.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config concentra toda la configuración runtime de Domain.
type Config struct {
	// Env / Server
	Env      string // dev | staging | prod
	HTTPBind string
	HTTPPort int

	HTTPReadTimeoutSeconds  int
	HTTPWriteTimeoutSeconds int

	// Database
	DatabaseURL     string // app_user pool — runtime queries (RLS enforced)
	DatabaseAuthURL     string // app_admin pool — auth/audit lookups (BYPASSRLS).
	DatabaseReadOnlyURL string // HU-25.9 read replica DSN (opcional, default vacío)
	// Si DatabaseAuthURL vacío, dev-fallback al pool de DatabaseURL con warning.

	// S3 (storage HU-04.6, GDPR export HU-23.3)
	S3Endpoint     string
	S3Region       string
	S3Bucket       string
	S3AccessKey    string
	S3SecretKey    string
	S3UsePathStyle bool

	// SMTP (HU-20.2, HU-02.7 OTP, HU-21.2 invitations)
	SMTPHost     string
	SMTPPort     int
	SMTPAuth     string // none | plain | login | cram-md5
	SMTPUser     string
	SMTPPassword string
	SMTPTLS      bool
	SMTPFrom     string

	// Logging (HU-17.3)
	LogLevel     string
	LogFormat    string // text | json
	LogOutput    string // stdout | stderr
	LogAddSource bool

	// Metrics (HU-17.1)
	MetricsEnabled bool
	MetricsBind    string
	MetricsPort    int

	// Tracing (HU-17.2)
	OTelEnabled         bool
	OTelExporterOTLPURL string
	OTelExporterProto   string // grpc | http/protobuf
	OTelSampleRatio     float64
	OTelServiceName     string

	// Seeders (HU-01.7)
	SeedOnBoot bool
}

// Load lee config desde env vars, aplica defaults y valida.
func Load() (*Config, error) {
	c := &Config{
		Env:                     getEnv("DOMAIN_ENV", "dev"),
		HTTPBind:                getEnv("DOMAIN_HTTP_BIND", "127.0.0.1"),
		HTTPPort:                getEnvInt("DOMAIN_HTTP_PORT", 8000),
		HTTPReadTimeoutSeconds:  getEnvInt("DOMAIN_HTTP_READ_TIMEOUT_SECONDS", 30),
		HTTPWriteTimeoutSeconds: getEnvInt("DOMAIN_HTTP_WRITE_TIMEOUT_SECONDS", 30),

		DatabaseURL:     getEnv("DOMAIN_DATABASE_URL", ""),
		DatabaseAuthURL:     getEnv("DOMAIN_DATABASE_AUTH_URL", ""),
		DatabaseReadOnlyURL: getEnv("DOMAIN_DATABASE_READONLY_URL", ""),

		S3Endpoint:     getEnv("DOMAIN_S3_ENDPOINT", ""),
		S3Region:       getEnv("DOMAIN_S3_REGION", "us-east-1"),
		S3Bucket:       getEnv("DOMAIN_S3_BUCKET", ""),
		S3AccessKey:    getEnv("DOMAIN_S3_ACCESS_KEY", ""),
		S3SecretKey:    getEnv("DOMAIN_S3_SECRET_KEY", ""),
		S3UsePathStyle: getEnvBool("DOMAIN_S3_USE_PATH_STYLE", true),

		SMTPHost:     getEnv("DOMAIN_SMTP_HOST", ""),
		SMTPPort:     getEnvInt("DOMAIN_SMTP_PORT", 1025),
		SMTPAuth:     getEnv("DOMAIN_SMTP_AUTH", "none"),
		SMTPUser:     getEnv("DOMAIN_SMTP_USER", ""),
		SMTPPassword: getEnv("DOMAIN_SMTP_PASSWORD", ""),
		SMTPTLS:      getEnvBool("DOMAIN_SMTP_TLS", false),
		SMTPFrom:     getEnv("DOMAIN_SMTP_FROM", "no-reply@domain.local"),

		LogLevel:     getEnv("DOMAIN_LOG_LEVEL", "info"),
		LogFormat:    getEnv("DOMAIN_LOG_FORMAT", "text"),
		LogOutput:    getEnv("DOMAIN_LOG_OUTPUT", "stdout"),
		LogAddSource: getEnvBool("DOMAIN_LOG_ADD_SOURCE", false),

		MetricsEnabled: getEnvBool("DOMAIN_METRICS_ENABLED", false),
		MetricsBind:    getEnv("DOMAIN_METRICS_BIND", "127.0.0.1"),
		MetricsPort:    getEnvInt("DOMAIN_METRICS_PORT", 9090),

		OTelEnabled:         getEnvBool("DOMAIN_OTEL_ENABLED", false),
		OTelExporterOTLPURL: getEnv("DOMAIN_OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317"),
		OTelExporterProto:   getEnv("DOMAIN_OTEL_EXPORTER_OTLP_PROTOCOL", "grpc"),
		OTelSampleRatio:     getEnvFloat("DOMAIN_OTEL_SAMPLE_RATIO", 1.0),
		OTelServiceName:     getEnv("DOMAIN_OTEL_SERVICE_NAME", "domain-mcp"),

		SeedOnBoot: getEnvBool("DOMAIN_SEED_ON_BOOT", true),
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate aplica reglas semánticas; fail-fast al boot.
func (c *Config) Validate() error {
	var errs []string
	if c.DatabaseURL == "" {
		errs = append(errs, "DOMAIN_DATABASE_URL is required")
	}
	if c.HTTPPort < 1 || c.HTTPPort > 65535 {
		errs = append(errs, fmt.Sprintf("DOMAIN_HTTP_PORT out of range: %d", c.HTTPPort))
	}
	switch c.Env {
	case "dev", "staging", "prod":
	default:
		errs = append(errs, fmt.Sprintf("DOMAIN_ENV invalid: %q (dev|staging|prod)", c.Env))
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Sprintf("DOMAIN_LOG_LEVEL invalid: %q", c.LogLevel))
	}
	switch c.LogFormat {
	case "text", "json":
	default:
		errs = append(errs, fmt.Sprintf("DOMAIN_LOG_FORMAT invalid: %q (text|json)", c.LogFormat))
	}
	switch c.SMTPAuth {
	case "none", "plain", "login", "cram-md5":
	default:
		errs = append(errs, fmt.Sprintf("DOMAIN_SMTP_AUTH invalid: %q", c.SMTPAuth))
	}
	if c.OTelSampleRatio < 0 || c.OTelSampleRatio > 1 {
		errs = append(errs, fmt.Sprintf("DOMAIN_OTEL_SAMPLE_RATIO out of range [0,1]: %f", c.OTelSampleRatio))
	}
	if len(errs) > 0 {
		return errors.New("config validation:\n  - " + strings.Join(errs, "\n  - "))
	}
	return nil
}

// IsProduction true si Env == prod.
func (c *Config) IsProduction() bool { return c.Env == "prod" }

// IsDev true si Env == dev.
func (c *Config) IsDev() bool { return c.Env == "dev" }

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getEnvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getEnvFloat(k string, def float64) float64 {
	if v := os.Getenv(k); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
