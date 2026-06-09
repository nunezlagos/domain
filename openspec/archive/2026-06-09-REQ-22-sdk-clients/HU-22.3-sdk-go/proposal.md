# Proposal: HU-22.3-sdk-go

## Intención

SDK Go oficial en repo separado, idiomatic (context, errors, options pattern), publicado vía proxy.golang.org con vanity import opcional, generado en parte desde OpenAPI pero con surface manual delgado.

## Scope

**Incluye:**
- Repo separado `github.com/domain/sdk-go` (mirror opcional vanity `go.domain.sh/sdk`)
- Cliente con `Options` functional pattern (`WithHTTPClient`, `WithAPIKey`, `WithBaseURL`)
- Services por recurso (`client.Observations`, `client.Agents`, ...)
- Retries con `pkg/retry` propio + classifier 4xx vs 5xx
- Iter pagination idiomatic
- Tipos generados desde OpenAPI bajo `internal/` pero expuestos en API pública

**No incluye:**
- gRPC/MCP client (separados)
- CLI Go (la propia `domain` ya cubre)

## Enfoque técnico

1. Repo separado para evitar mezclar mod del binary con SDK
2. `oapi-codegen` para tipos solo
3. Cliente manual idiomatic encima

## Riesgos

- Semver Go: minor del SDK refleja minor del API; major si breaking
- Doble mantenimiento si types diverge → CI compara types generados

## Testing

- httptest server con responses fixture
- Tests integration contra dev compose
- govet + staticcheck strict
