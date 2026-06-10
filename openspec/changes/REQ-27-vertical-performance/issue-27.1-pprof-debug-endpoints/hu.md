# issue-27.1-pprof-debug-endpoints

**Origen:** `REQ-27-vertical-performance`
**Prioridad tentativa:** media
**Tipo:** tooling

## Historia de usuario

**Como** SRE/dev debuggeando issue prod
**Quiero** endpoints `/debug/pprof/*` accesibles bajo auth
**Para** profiling on-demand (heap, goroutines, CPU, allocs) sin restart con flag

## Criterios de aceptación

### Escenario 1: Endpoints presentes en puerto separado

```gherkin
Dado que `DOMAIN_DEBUG_ENABLED=true` y `DOMAIN_DEBUG_PORT=6060`
Cuando arranca el server
Entonces se sirven en `localhost:6060/debug/pprof/`:
  - `/debug/pprof/heap`
  - `/debug/pprof/goroutine`
  - `/debug/pprof/profile?seconds=30`
  - `/debug/pprof/allocs`
  - `/debug/pprof/mutex`
  - `/debug/pprof/block`
  - `/debug/pprof/cmdline`
  - `/debug/pprof/symbol`
Y bind solo 127.0.0.1 por default
```

### Escenario 2: Basic auth obligatoria

```gherkin
Dado que `DOMAIN_DEBUG_AUTH_USER` y `DOMAIN_DEBUG_AUTH_PASSWORD` configurados
Cuando GET /debug/pprof/heap sin auth
Entonces 401
Cuando con basic auth correcto
Entonces 200 + heap dump
```

### Escenario 3: Acceso vía port-forward K8s

```gherkin
Dado que SRE necesita profile
Cuando `kubectl port-forward pod/domain-X 6060:6060`
Y `go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30`
Entonces obtiene profile interactivo
```

### Escenario 4: Disabled en default prod

```gherkin
Dado que `DOMAIN_DEBUG_ENABLED=false` (default)
Cuando se intenta acceder a /debug/pprof/heap
Entonces 404 (endpoint no registrado)
```

### Escenario 5: Audit log de uso

```gherkin
Dado que SRE descarga profile
Cuando se procesa
Entonces audit_log "debug.pprof.accessed" con who, endpoint, ip, ua
```

## Análisis breve

- **Qué pide:** stdlib net/http/pprof + puerto separado + auth + audit + helm config
- **Esfuerzo:** S
- **Riesgos:** profile endpoint expone info sensible → auth + bind interno
