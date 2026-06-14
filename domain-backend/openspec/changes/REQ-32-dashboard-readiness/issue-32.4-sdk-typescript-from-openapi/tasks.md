# Tasks: issue-32.4-sdk-typescript-from-openapi

## Backend

- [ ] **T1**: Crear estructura `sdks/typescript/`:
  - `package.json` con `name: "@tudominio/domain-sdk"`, `version:
    0.1.0`, scripts `build`, `test`, `typecheck`.
  - `tsconfig.json` con `target: ES2020`, `module: ESNext`,
    `strict: true`.
  - `src/index.ts` (vacío, se llena con la generación).
  - `test/` con setup de msw.
  - `README.md` con cómo usar.

- [ ] **T2**: Generar SDK inicial con la spec actual:
  ```bash
  npx openapi-typescript-codegen \
    --input ./docs/swagger.json \
    --output ./sdks/typescript/src \
    --client fetch
  ```
  Commit los archivos generados.

- [ ] **T3**: Wrapper de autenticación dual: crear
  `sdks/typescript/src/client.ts` que wrappea el `DomainClient`
  generado y agrega la lógica de auth:
  ```typescript
  export class DomainClient {
    constructor(private config: ClientConfig) {}
    observations = new ObservationsService(this);
    // etc.
  }
  ```
  Los `*Service` son wrappers delgados sobre el código
  generado que solo agregan headers de auth.

- [ ] **T4**: Setup de msw para tests E2E:
  - `sdks/typescript/test/server.ts` con handlers que mockean
    cada endpoint.
  - `sdks/typescript/test/client.test.ts` con tests que verifican
    que `DomainClient.observations.list()` llama al endpoint
    correcto y parsea la response.

- [ ] **T5**: Makefile targets:
  ```makefile
  OPENAPI_URL ?= http://localhost:8000/api/v1/openapi.json
  sdk-generate:
      cd sdks/typescript && \
        npx --yes openapi-typescript-codegen \
          --input $(OPENAPI_URL) --output ./src --client fetch
  sdk-test: sdk-generate
      cd sdks/typescript && npm ci && npm test
  sdk-publish:
      cd sdks/typescript && \
        npm version $(VERSION) && npm publish --access public
  ```

- [ ] **T6**: GitHub Action `.github/workflows/sdk.yml`:
  - Trigger: tag `v*` o push a `main`.
  - Steps: download OpenAPI artifact → `make sdk-generate` →
    `npm ci` → `npm run typecheck` → `npm test` → si tag, `npm
    publish` con `NPM_TOKEN` secret.

- [ ] **T7**: Documentar uso en `sdks/typescript/README.md`:
  ```typescript
  import { DomainClient } from '@tudominio/domain-sdk';

  const client = new DomainClient({
    baseURL: 'https://api.tudominio.com',
    apiKey: 'sk_xxx',
  });

  const obs = await client.observations.list({project: 'foo'});
  ```

## Tests

- [ ] **T-unit-1**: `TestDomainClient_AddsBearerHeader**` — request
  con `apiKey` → mock server recibe `Authorization: Bearer sk_xxx`.
- [ ] **T-unit-2**: `TestDomainClient_AddsCookieCreds**` — request
  con `getSessionCookie` → mock recibe `credentials: include`.
- [ ] **T-e2e-1**: `TestSDK_ListObservations**` — mock msw que
  retorna `[{id, content}]` → `client.observations.list()` retorna
  el array tipado.
- [ ] **T-e2e-2**: `TestSDK_CreateObservation**` — mock que acepta
  POST → `client.observations.create({content})` retorna
  `Observation` con `id`.
- [ ] **T-e2e-3**: `TestSDK_HandlesErrors**` — mock que retorna 401
  → `client.observations.list()` lanza error con `status: 401,
  message: 'unauthenticated'`.
- [ ] **T-e2e-4**: `TestSDK_TypeChecks**` — `npm run typecheck` pasa
  sin errores. Si un endpoint cambia y el SDK no se regenera,
  este test detecta tipos faltantes.
- [ ] **T-sabotaje**: Agregar un campo `tags: string[]` a
  `Observation` en el server (con la spec regenerada) pero NO
  regenerar el SDK (sabotaje) → test e2e-4 DEBE FALLAR
  (`Observation` no tiene `tags`) → restaurar SDK regenerado →
  test verde. Documentar en commit body.
