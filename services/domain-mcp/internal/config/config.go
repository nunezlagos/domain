// Package config — issue-01.2 config-system.
//
// Carga config desde env vars con prefijo DOMAIN_*. Validación strict + defaults.
// Una sola Load() al boot; valores no se recargan salvo via issue-27.3 hot-reload.
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
	Env      string // dev | staging | prod
	HTTPBind string
	HTTPPort int

	HTTPReadTimeoutSeconds  int
	HTTPWriteTimeoutSeconds int

	DatabaseURL         string // app_user pool — runtime queries (RLS enforced)
	DatabaseAuthURL     string // app_admin pool — auth/audit lookups (BYPASSRLS).
	DatabaseReadOnlyURL string // issue-25.9 read replica DSN (opcional, default vacío)

	S3Endpoint     string
	S3Region       string
	S3Bucket       string
	S3AccessKey    string
	S3SecretKey    string
	S3UsePathStyle bool

	SMTPHost     string
	SMTPPort     int
	SMTPAuth     string // none | plain | login | cram-md5
	SMTPUser     string
	SMTPPassword string
	SMTPTLS      bool
	SMTPFrom     string

	LogLevel     string
	LogFormat    string // text | json
	LogOutput    string // stdout | stderr
	LogAddSource bool

	MetricsEnabled bool
	MetricsBind    string
	MetricsPort    int
	MetricsUser    string
	MetricsPass    string

	OTelEnabled         bool
	OTelExporterOTLPURL string
	OTelExporterProto   string // grpc | http/protobuf
	OTelSampleRatio     float64
	OTelServiceName     string

	FieldEncKey string

	SeedOnBoot bool

	RateLimitRequests int
	RateLimitWindow   string // e.g. "60s"

	CORSOrigins []string

	HeartbeatWatcherEnabled        bool
	HeartbeatWatcherTimeoutMinutes int // default 5
	HeartbeatWatcherTickSeconds    int // default 60
	OrphanAuditEnabled             bool
	OrphanAuditSchedule            string // formato cron; default "0 4 * * *"

	// HealthPoller — system cron de auto-monitoreo del MCP. Escribe heartbeats
	// en mcp_health_checks cada 60s. Default enabled (bajo consumo).
	HealthPollerEnabled bool

	// AuthAnomalyAudit — system cron SOC (DOMAINSERV-82 H3). Cada 15m detecta
	// brute-force sobre auth_events y emite slog.Warn. Default enabled (barato:
	// una query indexada por tick).
	AuthAnomalyAuditEnabled bool

	// EdgeInference — system cron de inferencia de aristas de memoria con MiniMax.
	// Default disabled: requiere LLM_API_KEY (alias: MINIMAX_API_KEY) y consume tokens; opt-in explícito.
	EdgeInferenceEnabled      bool
	EdgeInferenceTickHours    int // default 6
	EdgeInferenceMaxPairs     int // pares candidatos por proyecto por pasada; default 30
	EdgeInferenceProjectBatch int // proyectos por pasada; default 50

	// FeedbackAggregator — system cron (HU-52.1) que consolida skill_feedback
	// en skill_feedback_daily cada N horas. Default disabled: opt-in explicito.
	FeedbackAggregatorEnabled   bool
	FeedbackAggregatorTickHours int // default 6
	FeedbackAggregatorDays      int // ventana a consolidar por pasada; default 7

	// SkillMetrics — system crons (HU-52.2): agregan skill_executions en
	// skill_metrics_daily/weekly. Default disabled: opt-in explicito.
	SkillMetricsEnabled         bool
	SkillMetricsTickHours       int // aggregator hourly; default 1
	SkillMetricsRollupTickHours int // rollup+cleanup; default 24
	SkillMetricsDailyRetention  int // dias; default 90
	SkillMetricsWeeklyRetention int // dias; default 365

	// SkillJudge — system cron (HU-52.3): LLM-as-judge semanal que genera
	// sugerencias 'pending' (split/merge/refine/archive). Human-in-the-loop:
	// NADA se auto-aplica. Default disabled: opt-in explicito. Degrada sin LLM.
	SkillJudgeEnabled   bool
	SkillJudgeWeekday   int // 0=domingo .. 1=lunes (default); ventana semanal
	SkillJudgeHour      int // hora local de la corrida; default 3 (03:00)
	SkillJudgeMaxSkills int // skills escaneados por corrida; default 200

	// ABTest — system cron (HU-52.4): Analyzer (z-test de proporciones) sobre los
	// skill_ab_tests 'running' cada N horas. Default disabled: opt-in explicito.
	// AutoApply (global) por default FALSE: solo declara el ganador, no pinea.
	ABTestEnabled   bool
	ABTestTickHours int     // analyzer cada N horas; default 6
	ABTestAlpha     float64 // nivel de significancia del z-test; default 0.05
	ABTestAutoApply bool    // pin global del ganador; default false
}

// Load lee config desde env vars, aplica defaults y valida.
func Load() (*Config, error) {
	c := &Config{
		Env:                     getEnv("DOMAIN_ENV", "dev"),
		HTTPBind:                getEnv("DOMAIN_HTTP_BIND", "127.0.0.1"),
		HTTPPort:                getEnvInt("DOMAIN_HTTP_PORT", 8000),
		HTTPReadTimeoutSeconds:  getEnvInt("DOMAIN_HTTP_READ_TIMEOUT_SECONDS", 30),
		HTTPWriteTimeoutSeconds: getEnvInt("DOMAIN_HTTP_WRITE_TIMEOUT_SECONDS", 30),

		DatabaseURL:         getEnv("DOMAIN_DATABASE_URL", ""),
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
		MetricsUser:    getEnv("DOMAIN_METRICS_USER", ""),
		MetricsPass:    getEnv("DOMAIN_METRICS_PASS", ""),

		OTelEnabled:         getEnvBool("DOMAIN_OTEL_ENABLED", false),
		OTelExporterOTLPURL: getEnv("DOMAIN_OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317"),
		OTelExporterProto:   getEnv("DOMAIN_OTEL_EXPORTER_OTLP_PROTOCOL", "grpc"),
		OTelSampleRatio:     getEnvFloat("DOMAIN_OTEL_SAMPLE_RATIO", 1.0),
		OTelServiceName:     getEnv("DOMAIN_OTEL_SERVICE_NAME", "domain-mcp"),

		FieldEncKey: getEnv("DOMAIN_FIELD_ENC_KEY", ""),

		SeedOnBoot: getEnvBool("DOMAIN_SEED_ON_BOOT", true),

		RateLimitRequests: getEnvInt("DOMAIN_RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:   getEnv("DOMAIN_RATE_LIMIT_WINDOW", "60s"),

		CORSOrigins: parseCSV(getEnv("DOMAIN_CORS_ORIGINS", "")),

		HeartbeatWatcherEnabled:        getEnvBool("DOMAIN_HEARTBEAT_WATCHER_ENABLED", true),
		HeartbeatWatcherTimeoutMinutes: getEnvInt("DOMAIN_HEARTBEAT_WATCHER_TIMEOUT_MINUTES", 5),
		HeartbeatWatcherTickSeconds:    getEnvInt("DOMAIN_HEARTBEAT_WATCHER_TICK_SECONDS", 60),
		OrphanAuditEnabled:             getEnvBool("DOMAIN_ORPHAN_AUDIT_ENABLED", true),
		OrphanAuditSchedule:            getEnv("DOMAIN_ORPHAN_AUDIT_SCHEDULE", "0 4 * * *"),

		HealthPollerEnabled: getEnvBool("DOMAIN_HEALTH_POLLER_ENABLED", true),

		AuthAnomalyAuditEnabled: getEnvBool("DOMAIN_AUTH_ANOMALY_AUDIT_ENABLED", true),

		EdgeInferenceEnabled:      getEnvBool("DOMAIN_EDGE_INFERENCE_ENABLED", false),
		EdgeInferenceTickHours:    getEnvInt("DOMAIN_EDGE_INFERENCE_TICK_HOURS", 6),
		EdgeInferenceMaxPairs:     getEnvInt("DOMAIN_EDGE_INFERENCE_MAX_PAIRS", 30),
		EdgeInferenceProjectBatch: getEnvInt("DOMAIN_EDGE_INFERENCE_PROJECT_BATCH", 50),

		FeedbackAggregatorEnabled:   getEnvBool("DOMAIN_FEEDBACK_AGGREGATOR_ENABLED", false),
		FeedbackAggregatorTickHours: getEnvInt("DOMAIN_FEEDBACK_AGGREGATOR_TICK_HOURS", 6),
		FeedbackAggregatorDays:      getEnvInt("DOMAIN_FEEDBACK_AGGREGATOR_DAYS", 7),

		SkillMetricsEnabled:         getEnvBool("DOMAIN_SKILL_METRICS_ENABLED", false),
		SkillMetricsTickHours:       getEnvInt("DOMAIN_SKILL_METRICS_TICK_HOURS", 1),
		SkillMetricsRollupTickHours: getEnvInt("DOMAIN_SKILL_METRICS_ROLLUP_TICK_HOURS", 24),
		SkillMetricsDailyRetention:  getEnvInt("DOMAIN_SKILL_METRICS_DAILY_RETENTION_DAYS", 90),
		SkillMetricsWeeklyRetention: getEnvInt("DOMAIN_SKILL_METRICS_WEEKLY_RETENTION_DAYS", 365),

		SkillJudgeEnabled:   getEnvBool("DOMAIN_SKILL_JUDGE_ENABLED", false),
		SkillJudgeWeekday:   getEnvInt("DOMAIN_SKILL_JUDGE_WEEKDAY", 1),
		SkillJudgeHour:      getEnvInt("DOMAIN_SKILL_JUDGE_HOUR", 3),
		SkillJudgeMaxSkills: getEnvInt("DOMAIN_SKILL_JUDGE_MAX_SKILLS", 200),

		ABTestEnabled:   getEnvBool("DOMAIN_AB_TEST_ENABLED", false),
		ABTestTickHours: getEnvInt("DOMAIN_AB_TEST_TICK_HOURS", 6),
		ABTestAlpha:     getEnvFloat("DOMAIN_AB_TEST_ALPHA", 0.05),
		ABTestAutoApply: getEnvBool("DOMAIN_AB_TEST_AUTO_APPLY", false),
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

// parseCSV separa por comas, trimea espacios, descarta vacíos.
func parseCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
