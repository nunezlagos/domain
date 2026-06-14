# issue-22.3-sdk-go

**Origen:** `REQ-22-sdk-clients`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer Go integrando Domain
**Quiero** un SDK Go idiomatic con context.Context, retries y publicado vía go modules
**Para** usar el API sin escribir HTTP a mano y con tipos compartidos cuando hay módulo común

## Criterios de aceptación

### Escenario 1: Módulo separado del repo principal

```gherkin
Dado que existe repo `go.domain.sh/sdk` (vanity import) o `github.com/domain/sdk-go`
Cuando hago `go get go.domain.sh/sdk@latest`
Entonces se descarga la última versión
Y compila sin warnings con Go 1.22+
```

### Escenario 2: API idiomatic

```gherkin
Dado que existe `sdk.NewClient(opts ...sdk.Option)`
Cuando hago `client.Observations.List(ctx, &sdk.ListObservationsParams{ProjectID: id})`
Entonces se devuelve `[]Observation, *Response, error`
Y `Response` incluye request_id, rate limit headers
```

### Escenario 3: Context cancel

```gherkin
Dado que pasamos ctx con deadline
Cuando se cancela durante request
Entonces el request se aborta y se devuelve `ctx.Err()`
```

### Escenario 4: Retry transparente

```gherkin
Dado que respuesta es 503
Cuando llamamos método del SDK
Entonces se reintenta 3x con backoff
Y errores 4xx no se reintentan
```

### Escenario 5: Iter pagination

```gherkin
Dado que list es paginado
Cuando hago `client.Observations.Iter(ctx, params)`
Entonces se devuelve un iterator con .Next() / .Value() / .Err()
```

## Análisis breve

- **Qué pide:** SDK Go independiente publicado vía proxy.golang.org
- **Esfuerzo:** M
- **Riesgos:** mantener compat semver; tagging
