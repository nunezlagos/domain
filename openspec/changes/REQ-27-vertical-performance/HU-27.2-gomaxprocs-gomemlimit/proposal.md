# Proposal: HU-27.2-gomaxprocs-gomemlimit

## Intención

Runtime tuning cgroups-aware: GOMAXPROCS auto-detected con uber/automaxprocs, GOMEMLIMIT desde cgroup memory limit con buffer 90%, GOGC opcional override.

## Scope

- Dep `go.uber.org/automaxprocs`
- Read cgroup memory limit (v1 + v2)
- debug.SetMemoryLimit con buffer 10%
- GOGC env override
- Endpoint /health/runtime
- Logs informativos al boot

## Riesgos

- Cgroup v2 vs v1 paths diferentes: lib estándar maneja
- GOMEMLIMIT muy agresivo → GC thrashing: buffer 10%

## Testing

- Container con cap CPU 2 → GOMAXPROCS=2
- Container con memory 2Gi → GOMEMLIMIT ~1.8Gi
- Env override respetado + warn
- Endpoint runtime reporta valores
