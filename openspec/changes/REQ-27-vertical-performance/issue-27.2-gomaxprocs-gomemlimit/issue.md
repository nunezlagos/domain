# issue-27.2-gomaxprocs-gomemlimit

**Origen:** `REQ-27-vertical-performance`
**Prioridad tentativa:** alta
**Tipo:** hardening

## Historia de usuario

**Como** runtime Go en container
**Quiero** GOMAXPROCS y GOMEMLIMIT auto-detectados desde cgroups
**Para** que no use 64 cores cuando el container tiene cap 2, ni OOM por no respetar memory limit

## Criterios de aceptación

### Escenario 1: GOMAXPROCS desde cgroup

```gherkin
Dado que `uber-go/automaxprocs` integrado al boot
Y container con cgroup cpu.limit=2.0
Cuando arranca
Entonces runtime.GOMAXPROCS() == 2
Y log info "GOMAXPROCS=2 (from cgroup)"
Y NO usa runtime.NumCPU() del host
```

### Escenario 2: GOMEMLIMIT desde cgroup memory

```gherkin
Dado que container memory limit 2Gi
Y código:
  ```go
  // cmd/domain-mcp/main.go
  if limit, ok := readCgroupMemLimit(); ok {
    debug.SetMemoryLimit(int64(float64(limit) * 0.9))  // 90% buffer
  }
  ```
Cuando arranca
Entonces GC más agresivo cerca del límite → evita OOM kill
```

### Escenario 3: Override manual

```gherkin
Dado que `GOMAXPROCS=4` env explícito
Y cgroup limit 2
Cuando arranca
Entonces respeta env explícito (4)
Y log warn "GOMAXPROCS overridden, cgroup limit was 2"
```

### Escenario 4: Métricas runtime

```gherkin
Dado que health endpoint
Cuando GET /health/runtime
Entonces 200 con
  ```json
  {
    "gomaxprocs": 2,
    "gomemlimit_bytes": 1932735283,
    "num_goroutine": 142,
    "alloc_bytes": 234567890,
    "gc_pause_p99_ms": 0.5
  }
  ```
```

### Escenario 5: GC tuning opcional

```gherkin
Dado que `DOMAIN_GOGC=50` env
Cuando arranca
Entonces runtime.SetGCPercent(50) (GC más frequente, menor pico memoria)
```

## Análisis breve

- **Qué pide:** automaxprocs + setMemoryLimit + métricas + env overrides + endpoint runtime
- **Esfuerzo:** S
- **Riesgos:** automaxprocs lib OK; GOMEMLIMIT con buffer 90% para evitar OOM
