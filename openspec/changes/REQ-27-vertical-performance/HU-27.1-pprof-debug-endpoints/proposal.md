# Proposal: HU-27.1-pprof-debug-endpoints

## Intención

Exponer `/debug/pprof/*` en puerto separado autenticado con audit, opt-in vía env, bind interno.

## Scope

- `net/http/pprof` registered en mux separado
- Basic auth middleware
- Audit log de accesos
- Helm config
- Bind 127.0.0.1 default

## Riesgos

- Profile expone strings, code paths, allocs → auth obligatoria
- /debug/pprof/profile bloquea N segundos → solo SRE deliberado

## Testing

- Endpoints respondieron
- Auth required
- Bind interno (no 0.0.0.0)
- Audit log
