# HU-26.1-stateless-invariant

**Origen:** `REQ-26-horizontal-scalability`
**Persona:** security-officer
**Prioridad tentativa:** alta
**Tipo:** hardening + tooling

## Historia de usuario

**Como** plataforma multi-pod
**Quiero** un invariante "no state crítico in-memory" validado por linter
**Para** garantizar que cualquier pod puede recibir cualquier request y que reiniciar un pod no pierde data

## Criterios de aceptación

### Escenario 1: Linter detecta state crítico

```gherkin
Dado que existe linter `cmd/domain-lint-stateless/`
Cuando scaneo paquetes (excepto whitelist)
Entonces detecta y reporta:
  - `var counter int` global mutable
  - `var cache = map[K]V{}` sin TTL ni sync
  - `sync.Map` global sin comment de justificación
  - Channels sin owner claro
Y CI fail si encuentra issues no-whitelisted
```

### Escenario 2: Whitelist explícita

```gherkin
Dado que cierto estado in-memory es legítimo (cache LRU de policies, metrics counters)
Cuando declaramos en `.stateless-allowed.yaml`:
  ```yaml
  allowed:
    - path: internal/mcp/resilience/cache.go
      var: lruCache
      reason: "Cache LRU de policies con TTL; aceptable, NO contiene state crítico"
  ```
Entonces el linter ignora con audit reason
```

### Escenario 3: Tests verifican stateless

```gherkin
Dado que arranco 2 pods con misma config
Cuando hago 100 requests
Entonces ambos pueden servir cualquiera
Y kill un pod NO pierde data persisted
```

## Análisis breve

- **Qué pide:** AST linter Go + whitelist YAML + tests integration multi-pod
- **Esfuerzo:** S-M
- **Riesgos:** falsos positivos en patterns legítimos (cache, pools) → whitelist
