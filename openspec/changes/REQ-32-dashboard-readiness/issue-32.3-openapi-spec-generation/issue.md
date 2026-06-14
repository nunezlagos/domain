# issue-32.3-openapi-spec-generation

**Origen:** `REQ-32-dashboard-readiness`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer del dashboard o de un cliente externo
**Quiero** tener disponible la spec OpenAPI 3.0 del server domain
**Para** poder generar SDKs, mock servers, docs interactivas, y validar que mis requests cumplen el contrato

## Criterios de aceptación

### Escenario 1: Spec servido en `/api/v1/openapi.json`

```gherkin
Dado que el server está corriendo
Cuando hago `GET https://api.tudominio.com/api/v1/openapi.json` SIN auth (ruta pública)
Entonces el response es 200 con `Content-Type: application/json`
Y el body es un JSON OpenAPI 3.0 válido con:
  - openapi: "3.0.0"
  - info: {title, version, description}
  - servers: [production URL]
  - paths: {todos los endpoints REST}
  - components: {securitySchemes, schemas}
Y el archivo tiene <500KB
```

### Escenario 2: Cada endpoint tiene su operación

```gherkin
Dado el spec generado
Cuando inspecciono `paths['/api/v1/observations']`
Entonces tiene:
  - get: {summary, parameters, responses: {200, 401, 500}}
  - post: {summary, requestBody, responses: {201, 400, 401}}
Y los schemas referencian `components.schemas.Observation` (con
campos: id, content, project_id, created_at, etc.)
```

### Escenario 3: Tags agrupan por dominio

```gherkin
Dado el spec generado
Cuando inspecciono `tags`
Entonces hay tags: ["Observations", "Projects", "Agents",
"Flows", "Skills", "Policies", "Sessions", "Auth"]
Y cada path referencia al menos 1 tag
Y la UI de Swagger agrupa los endpoints visualmente
```

### Escenario 4: Versionado: cada release tag genera spec

```gherkin
Dado que tag `v0.2.0` se pushea a GitHub
Cuando corre la GitHub Action `release.yml`
Entonces se genera `openapi/v0.2.0.json` (con el spec de ese tag)
Y se commitea a `openapi/` branch (o se sube como artifact)
Y el CHANGELOG referencia el spec
```

### Escenario 5: Spec validation contra schema oficial

```gherkin
Dado el spec generado
Cuando corro `make openapi-validate`
Entonces el comando usa `@apidevtools/swagger-cli validate` o
similar para verificar que el JSON cumple OpenAPI 3.0 spec
Y exit code 0 si OK, !=0 si malformado
Y forma parte del CI
```

### Escenario 6: Sabotaje — spec desactualizado con respecto a handlers

```gherkin
Dado que un endpoint nuevo se registra en `internal/api/handler/`
Y el código de generación del spec NO incluye este endpoint (sabotaje)
Cuando hago `GET /api/v1/openapi.json`
Entonces el spec NO tiene el endpoint nuevo
Y el test e2e que assserta "cada handler tiene entry en el spec"
DEBE FALLAR
Cuando restauro la lógica de generación automática
Entonces el test verde
```

### Escenario 7: Edge case — endpoint con auth requerida

```gherkin
Dado el spec generado
Cuando inspecciono `paths['/api/v1/observations'].get`
Entonces tiene `security: [{bearerAuth: []}]`
Y `components.securitySchemes.bearerAuth` está definido
  (type: http, scheme: bearer, bearerFormat: API key)
```

### Escenario 8: Edge case — schema recursivo (e.g. observation con metadata)

```gherkin
Dado el spec generado
Cuando inspecciono `components.schemas.Observation.metadata`
Entonces es `type: object, additionalProperties: true` (no
requiere definir cada key del metadata)
Y NO genera ciclos infinitos
```

## Notas

- OpenAPI spec se genera en build time (no en runtime) usando
  `github.com/swaggo/swag` o `github.com/oapi-codegen/oapi-codegen`.
  Decisión: usar `swag` por su madurez (más adoption que oapi-codegen
  para generar spec desde código Go).
- El spec se SERVE en runtime también (`/api/v1/openapi.json`) —
  se genera al boot si no existe, o se incluye como embed.FS.
- La spec es SOURCE OF TRUTH para el SDK TS (32.4) y para el
  contrato con clientes externos.
