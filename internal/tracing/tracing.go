// Package tracing — HU-17.2 OpenTelemetry tracing.
//
// SDK setup con OTLP gRPC exporter + ParentBased(TraceIDRatioBased) sampler.
// Resource attributes: service.name, service.version, deployment.environment.
// SafeAttrs whitelist evita PII en span attributes.
package tracing

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config para Setup.
type Config struct {
	Enabled      bool
	OTLPEndpoint string  // host:port grpc, default localhost:4317
	ServiceName  string  // service.name resource attr
	Version      string  // service.version
	Environment  string  // deployment.environment
	SampleRatio  float64 // 0..1; 0=never, 1=always
	Insecure     bool    // use insecure gRPC (dev). prod: false con TLS
}

// Shutdown function para cleanup ordenado.
type Shutdown func(ctx context.Context) error

// Setup configura tracer provider global. Retorna shutdown function.
// Si !cfg.Enabled retorna noop provider + shutdown vacío.
func Setup(ctx context.Context, cfg Config) (Shutdown, error) {
	if !cfg.Enabled {
		otel.SetTracerProvider(noop.NewTracerProvider())
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{},
		))
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("resource merge: %w", err)
	}

	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	}
	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(clamp(cfg.SampleRatio)))
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return func(shutdownCtx context.Context) error {
		return tp.Shutdown(shutdownCtx)
	}, nil
}

func clamp(r float64) float64 {
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// Tracer convenience.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// ===== Safe Attrs (HU-17.2 + .claude/rules/security.md) =====

// SafeAttrKeys whitelist explícita. Otros keys son rechazados por SafeAttr.
var safeAttrKeys = map[string]bool{
	// HTTP
	"http.method": true, "http.status_code": true, "http.route": true,
	"http.host": true, "http.scheme": true, "http.target": true,
	// DB
	"db.system": true, "db.statement_short": true, "db.operation": true,
	"db.rows_affected": true,
	// LLM
	"llm.provider": true, "llm.model": true, "llm.input_tokens": true,
	"llm.output_tokens": true, "llm.cost_usd": true,
	// Domain
	"agent.slug": true, "skill.slug": true, "flow.slug": true,
	"org.id": true, "user.id": true, "project.id": true,
	"run.id": true,
}

// SafeAttr crea attribute.KeyValue solo si key está en whitelist.
// Si key prohibida, retorna empty KeyValue (será ignorada por OTel).
func SafeAttr(key string, value any) attribute.KeyValue {
	if !safeAttrKeys[key] {
		return attribute.KeyValue{} // empty, OTel skip
	}
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}

// IsSafeKey true si key está en whitelist (uso lint tests).
func IsSafeKey(key string) bool {
	return safeAttrKeys[key]
}

// ===== HTTP Middleware =====

// HTTPMiddleware envuelve handler con span automático.
// Span name: "HTTP {METHOD} {path}". Attrs: http.method, http.status_code, http.route.
// Propaga TraceContext desde headers entrantes y a contexto outgoing.
func HTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	tracer := Tracer(serviceName)
	prop := otel.GetTextMapPropagator()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := prop.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			path := r.URL.Path
			spanName := "HTTP " + r.Method + " " + normalizeRoute(path)

			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					SafeAttr("http.method", r.Method),
					SafeAttr("http.route", normalizeRoute(path)),
					SafeAttr("http.target", path),
				),
			)
			defer span.End()

			rec := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rec, r.WithContext(ctx))

			span.SetAttributes(SafeAttr("http.status_code", rec.status))
		})
	}
}

// normalizeRoute reemplaza UUIDs y números por placeholders.
// Similar a HU-17.1 metrics normalizePath para evitar cardinality blow-up en traces.
func normalizeRoute(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if len(part) == 36 && strings.Count(part, "-") == 4 {
			parts[i] = ":id"
			continue
		}
		if isNumeric(part) && part != "" {
			parts[i] = ":n"
		}
	}
	return strings.Join(parts, "/")
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
