# Tasks: issue-32.3-openapi-spec-generation

## Backend

- [ ] **T1**: Agregar `github.com/swaggo/swag` a `go.mod`. Instalar
  CLI: `go install github.com/swaggo/swag/cmd/swag@latest`.

- [ ] **T2**: Agregar annotations `// @Summary`, `// @Tags`, etc.
  a los handlers existentes en `internal/api/handler/*.go`.
  Empezar por los más usados: `auth_*.go`, `observations.go`,
  `projects.go`, `agents.go`, `flows.go`, `skills.go`, `policies.go`.

- [ ] **T3**: Definir security schemes globales. En
  `cmd/domain/main.go` o `docs/docs.go`:
  ```go
  // @securityDefinitions.apikey bearerAuth
  // @in header
  // @name Authorization
  // @description Type "Bearer <api_key>" or session cookie
  ```

- [ ] **T4**: Agregar `// @title Domain API`, `// @version 0.x.y`,
  `// @description Memory + orchestration API`, `// @host ...`,
  `// @BasePath /api/v1` en el `main.go` o un archivo de config
  dedicado.

- [ ] **T5**: Crear `internal/api/openapi/handler.go` con:
  ```go
  //go:embed docs/swagger.json
  var specBytes []byte

  func OpenAPIHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write(specBytes)
  }
  ```

- [ ] **T6**: Generar spec inicial con `swag init -g cmd/domain/main.go
  -o ./docs --parseDependency --parseInternal`. Commit
  `docs/swagger.json` (es build artifact pero se commitea para
  que `//go:embed` funcione sin correr swag antes del build).

- [ ] **T7**: Wire `mux.Handle("/api/v1/openapi.json",
  http.HandlerFunc(OpenAPIHandler))` en `cmd/domain/main.go`.
  Agregar a `handler.AuthAllowlist()` (público, sin auth).

- [ ] **T8**: Makefile targets:
  ```makefile
  openapi-generate:
      swag init -g cmd/domain/main.go -o ./docs --parseDependency --parseInternal
  openapi-validate: openapi-generate
      swagger-cli validate ./docs/swagger.json
  ```

- [ ] **T9**: GitHub Action `.github/workflows/release.yml`:
  - Trigger: `push tags: ['v*']`.
  - Step: setup Go, install swag, run `swag init`, upload
    `docs/swagger.json` como artifact del release.

## Tests

- [ ] **T-unit-1**: `TestOpenAPIHandler_ServesJSON**` — request a
  `/api/v1/openapi.json` → response 200, `Content-Type:
  application/json`, body parseable como JSON, tiene `openapi:
  "3.0.0"`.
- [ ] **T-unit-2**: `TestOpenAPIHandler_NoAuthRequired**` — request
  SIN Authorization → 200 (no está en auth required).
- [ ] **T-e2e-1**: `TestOpenAPISpec_AllHandlersPresent**` — listar
  los handlers en `internal/api/handler/`, parsear la spec, assserta
  que cada handler tiene su entry en `paths`. Falla si hay
  handler sin spec.
- [ ] **T-e2e-2**: `TestOpenAPISpec_ValidatesAgainstSchema**` —
  correr `swagger-cli validate` (vía `exec.Command`) sobre
  `docs/swagger.json` → exit 0.
- [ ] **T-e2e-3**: `TestOpenAPISpec_SecuritySchemes**` — la spec
  tiene `components.securitySchemes.bearerAuth` con type=http,
  scheme=bearer.
- [ ] **T-sabotaje**: Comentar `swag init` del Makefile (sabotaje:
  no regenerar) + agregar un handler nuevo SIN annotation → el
  spec NO incluye el nuevo endpoint → test e2e-1 DEBE FALLAR →
  restaurar Makefile + agregar annotation al handler → test
  verde. Documentar en commit body.
