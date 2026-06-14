# Design: issue-32.3-openapi-spec-generation

## Contexto

El dashboard y futuros clientes externos necesitan una spec
autoritativa del API. Mantenerla a mano (escribir 50+ endpoints en
YAML) es insostenible: cada cambio en un handler requiere sync
manual. La solución estándar: generar la spec desde el código Go
(annotations + reflexión) o desde un router introspection.

## Decisión arquitectónica

**Estrategia:** `swaggo/swag` para generar desde annotations +
servir la spec en runtime + GitHub Action para versionar en
releases.

1. **Annotations en handlers Go:** cada handler exporta su path,
   method, params, request body, responses via comments
   `// @Summary`, `// @Param`, `// @Success`, etc. Ejemplo:
   ```go
   // ObservationList godoc
   // @Summary List observations
   // @Description Returns paginated observations for the user's org.
   // @Tags Observations
   // @Produce json
   // @Param project_slug query string false "Filter by project"
   // @Param limit query int false "Max results (default 50, max 200)"
   // @Success 200 {array} Observation
   // @Failure 401 {object} ErrorResponse
   // @Router /observations [get]
   // @Security bearerAuth
   func (h *Handler) ObservationList(w http.ResponseWriter, r *http.Request) {
   ```
   Las annotations se parsean con `swag init` (CLI) que produce
   `docs/swagger.json` + `docs/swagger.yaml`.

2. **Build time generation:**
   ```makefile
   openapi-generate:
       swag init -g cmd/domain/main.go -o ./docs --parseDependency --parseInternal
   openapi-validate: openapi-generate
       swagger-cli validate ./docs/swagger.json
   ```
   `swag init` se corre como pre-commit o en CI antes del build.

3. **Runtime serving:** embed la spec con `//go:embed docs/swagger.json`
   y servirla en `/api/v1/openapi.json` (público, sin auth). El
   handler es trivial:
   ```go
   //go:embed docs/swagger.json
   var openAPISpec []byte

   func OpenAPIHandler(w http.ResponseWriter, r *http.Request) {
     w.Header().Set("Content-Type", "application/json")
     w.Write(openAPISpec)
   }
   ```

4. **Versionado en releases:** GitHub Action `release.yml` que en
   `push tag v*`:
   - Corre `swag init`.
   - Sube `docs/swagger.json` como artifact del release.
   - Opcionalmente: commitea a `openapi/vX.Y.Z.json` en branch
     `openapi-archive` para histórico.

5. **Validación:** `@apidevtools/swagger-cli validate` (npm) o
   `python -m openapi_spec_validator` (py). CI corre esto post-generate.
   Exit !=0 si la spec no es OpenAPI 3.0 válida.

6. **Tags y grouping:** las annotations `@Tags Observations`
   agrupan. La spec final tiene
   `tags: [{name: "Observations", description: "..."}]` que
   Swagger UI usa para agrupar visualmente.

7. **Security schemes:** annotations globales
   `// @securityDefinitions.apikey bearerAuth ApiKey auth,key=Authorization,in=header`
   (o `// @securityDefinitions.http` para Bearer). En la spec
   queda:
   ```yaml
   components:
     securitySchemes:
       bearerAuth:
         type: http
         scheme: bearer
         bearerFormat: API key
   ```

8. **Esquema recursivo (Observation.metadata):** definir como
   `type: object, additionalProperties: true` para no enumerar
   keys. `swag` soporta esto con `swaggertype:"object"` en el
   struct tag.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Spec escrita a mano en YAML | Insostenible: 50+ endpoints, cada cambio requiere sync. |
| B | Generar desde router introspection (httprouter, chi) | Posible pero limitado: no captura summaries, descriptions, examples. Annotations son más ricos. |
| C | `oapi-codegen` (generar Go desde spec, no spec desde Go) | Es el flow inverso: spec-first, Go-second. Útil cuando se quiere la spec como contrato, pero el server YA EXISTE con handlers. swag es el camino. |
| D | GraphQL | No aplica: el API ya es REST. Migrar a GraphQL es otro REQ. |

## Por qué swag + annotations gana

- **Source of truth:** las annotations viven al lado del handler.
  Imposible que diverjan.
- **Rico:** summaries, descriptions, examples, tags, security —
  todo se modela.
- **Estándar:** `swag` es la librería más usada en Go para esto.
  Comunidad y docs amplias.
- **CI-friendly:** la validación corre automático, el release se
  archiva.

## Detalle de implementación

- Agregar `github.com/swaggo/swag/cmd/swag` (CLI, dep de build) +
  `github.com/swaggo/swag` (runtime) a `go.mod`.
- Crear `docs/docs.go` (generado, no editar a mano).
- Crear `internal/api/openapi/handler.go` con el handler
  `OpenAPIHandler` que sirve el embed.
- Wire en `cmd/domain/main.go`:
  `mux.Handle("/api/v1/openapi.json", http.HandlerFunc(OpenAPIHandler))`.
- Makefile: `openapi-generate`, `openapi-validate`, `openapi-serve`
  (este último arranca un servidor de docs con `redoc-cli`).
- GitHub Action: `.github/workflows/release.yml` con step
  `swag init && upload artifact`.

## Riesgos

- **R1:** `swag` no soporta Go generics en structs. **Mitigación:**
  los structs del API no usan generics aún. Si se agregan,
  workaround: definir type alias o struct manual.
- **R2:** Annotations se desincronizan del código (alguien agrega
  un param al handler y olvida `@Param`). **Mitigación:** test E2E
  que valida spec vs router (similar a REQ-31.2 pero cross-spec).
- **R3:** El spec es grande. **Mitigación:** paginación opcional
  (e.g. `?include=Observation,Project`). swag no lo soporta nativamente;
  se puede hacer split en múltiples specs por tag y un agregador.
