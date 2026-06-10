# Design: issue-22.3-sdk-go

## Decisión arquitectónica

**Layout:** repo separado `github.com/domain/sdk-go` (no monorepo)
**Generator types:** `oapi-codegen` con strict-server=false
**Cliente:** manual encima de tipos generados
**Pattern:** functional options + services

## API ejemplo

```go
client, err := sdk.NewClient(
  sdk.WithAPIKey(os.Getenv("DOMAIN_API_KEY")),
  sdk.WithBaseURL("https://api.domain.sh"),
  sdk.WithTimeout(30*time.Second),
)
list, resp, err := client.Observations.List(ctx, &sdk.ListObservationsParams{
  ProjectID: projectID,
  Limit:     20,
})
```

## Estructura repo

```
sdk-go/
  go.mod
  client.go
  options.go
  errors.go
  pagination.go
  services/
    observations.go
    agents.go
    flows.go
    ...
  internal/
    generated/
      types.go    # oapi-codegen output
```

## TDD plan

1. httptest server fixture → cliente devuelve typed
2. Context cancel durante request → ctx.Err()
3. Retry 503 → ok después de 3
4. Iter pagination consume todas las páginas
5. govet + staticcheck strict
