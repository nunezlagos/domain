# REQ-27-vertical-performance: Performance vertical: pprof debug endpoints, GOMAXPROCS/GOMEMLIMIT cgroups-aware, hot-reload config, feature benchmarks.

**Estado:** activo
**Creado:** 2026-06-07

## Descripción

Tunear el binario Go para que escale verticalmente correctamente en containers/K8s: profiling on-demand, runtime tuning consciente de cgroups, hot-reload sin restart, benchmarks que detectan regresiones.

## Criterios de éxito

- Endpoints pprof en puerto separado autenticado: heap, goroutine, profile (CPU), allocs, mutex, block
- GOMAXPROCS auto-detecta cgroup CPU limit (no usa #CPUs del host); GOMEMLIMIT desde cgroup memory limit
- Hot-reload de config (log level, pool sizes, timeouts, feature flags) sin restart
- Benchmarks suite con baseline + regression check en CI (>10% slower fail)

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-27.1-pprof-debug-endpoints | proposed | /debug/pprof/* en puerto separado con basic auth |
| HU-27.2-gomaxprocs-gomemlimit | proposed | uber/automaxprocs + GOMEMLIMIT from cgroup |
| HU-27.3-hot-reload-config | proposed | Config reload via SIGHUP o admin API + LISTEN/NOTIFY |
| HU-27.4-feature-benchmarks | proposed | Benchmark suite + benchstat comparativo en CI |
