# issue-32.4-sdk-typescript-from-openapi

**Origen:** `REQ-32-dashboard-readiness`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer del dashboard o de un cliente JS/TS externo
**Quiero** poder instalar `@tudominio/domain-sdk` desde npm y tener tipos + cliente HTTP tipados para todo el API
**Para** no escribir `fetch()` a mano con tipos copiados, evitando bugs de drift entre el SDK y el server

## Criterios de aceptación

### Escenario 1: SDK instalable desde npm

```gherkin
Dado que el SDK se publica en npm como `@tudominio/domain-sdk`
Cuando el developer corre `npm install @tudominio/domain-sdk`
Entonces el package se instala con sus types
Y puede `import { DomainClient } from '@tudominio/domain-sdk'`
Y `new DomainClient({baseURL, apiKey})` retorna un cliente con métodos tipados
```

### Escenario 2: Cada endpoint es un método tipado

```gherkin
Dado que el server tiene `GET /api/v1/observations?project=...&limit=...`
Cuando uso el SDK
Entonces hay `client.observations.list({project, limit})` que retorna `Promise<Observation[]>`
Y `client.observations.create({content, project_slug})` que retorna `Promise<Observation>`
Y los types son inferidos del OpenAPI spec (no escritos a mano)
```

### Escenario 3: Autenticación dual (API key + sesión)

```gherkin
Dado que el cliente se inicializa con `{baseURL, apiKey}` o `{baseURL, getSessionCookie}`
Cuando hace requests
Entonces con apiKey: header `Authorization: Bearer <apiKey>`
Y con sessionCookie: `credentials: 'include'` (browser maneja cookie)
Y ambos funcionan contra el mismo server (32.1)
```

### Escenario 4: Pipeline de generación reproducible

```gherkin
Dado que `make sdk-generate` corre en CI
Cuando se ejecuta
Entonces:
  1. Descarga la última spec de `https://api.tudominio.com/api/v1/openapi.json` (o usa la local)
  2. Corre `openapi-typescript-codegen` (o `openapi-generator`) para producir TS
  3. Escribe `sdks/typescript/src/` con los archivos generados
  4. Corre `npm run build` y `npm test`
Y exit 0 si todo OK
```

### Escenario 5: SDK compila contra cada endpoint nuevo

```gherkin
Dado que un endpoint nuevo se agrega al server (con su spec regenerada)
Cuando corre el CI
Entonces `make sdk-generate` regenera el SDK con el método nuevo
Y `npm run typecheck` y `npm run build` pasan
Y `npm test` corre los E2E contra un server mockeado (vía msw o
similar) y verifica que el método funciona
Y si el endpoint nuevo no compila, CI rojo
```

### Escenario 6: Sabotaje — SDK con tipos desactualizados

```gherkin
Dado que un campo nuevo se agrega a `Observation` en el server (e.g. `tags: string[]`)
Y el SDK NO se regenera (sabotaje)
Cuando el developer intenta `client.observations.create({content, tags: ['x']})`
Entonces TypeScript error: "Object literal may only specify known properties, and 'tags' does not exist in type CreateObservationInput"
Y el CI rojo (test que assserta "SDK tiene el campo tags")
Cuando se regenera el SDK
Entonces el test verde
```

### Escenario 7: Edge case — endpoint con response union (200 | 404)

```gherkin
Dado que un endpoint puede retornar 200 con `Observation` o 404 con `ErrorResponse`
Cuando uso el SDK
Entonces el método retorna `Promise<{ok: true, data: Observation} | {ok: false, error: ErrorResponse}>`
Y el caller DEBE chequear `ok` antes de acceder a `data`
Y los types forzan este pattern (no hay `throw` implícito)
```

### Escenario 8: Publicación a npm (manual o automática)

```gherkin
Dado que tag `v0.2.0` se pushea
Cuando la GitHub Action `release.yml` corre
Entonces después de subir el OpenAPI artifact, también:
  1. `make sdk-generate`
  2. `npm version 0.2.0`
  3. `npm publish --access public`
Y el package `@tudominio/domain-sdk@0.2.0` queda disponible en npm
Y el CHANGELOG del SDK lista los cambios
```

## Notas

- Generador preferido: `openapi-typescript-codegen` (activo,
  output typeado, sin runtime overhead). Alternativa:
  `openapi-generator` (más viejo, más features pero más verboso).
- Decisión final: `openapi-typescript-codegen` por simplicidad y
  porque el output es type-only (sin runtime).
- El SDK NO incluye lógica de retry/circuit-breaker (eso es
  responsabilidad del caller o de un middleware separado). Solo
  hace HTTP requests tipados.
