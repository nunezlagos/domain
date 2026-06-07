# REQ-17-observability: Observabilidad: métricas Prometheus, tracing OpenTelemetry, structured logging con slog. Base operacional para producción.

**Estado:** activo
**Creado:** 2026-06-06

## Descripción

Sistema de observabilidad operacional: métricas Prometheus exportadas por defecto, tracing distribuido con OpenTelemetry, logs estructurados con slog en formato JSON. Habilita SLOs, debugging distribuido y correlación entre logs/metrics/traces.

## Criterios de éxito

- Endpoint `/metrics` en formato Prometheus con métricas de runtime Go, HTTP, DB pool, runs, costo
- Spans OpenTelemetry exportables via OTLP a Jaeger/Tempo/Honeycomb, con trace context propagado entre servicios
- Logs estructurados JSON con campos estándar (timestamp, level, msg, trace_id, span_id, request_id, user_id, project_id)
- Correlación entre los tres pilares vía `trace_id` y `request_id` consistentes en logs y spans

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-17.1-metrics-prometheus | proposed | Métricas Prometheus en `/metrics`: runtime Go, HTTP, DB pool, runs, costo, custom |
| HU-17.2-tracing-otel | proposed | Tracing OpenTelemetry con exporter OTLP, propagación W3C, sampling configurable |
| HU-17.3-structured-logging | proposed | Logs JSON con slog, correlación trace_id/request_id, niveles configurables |
