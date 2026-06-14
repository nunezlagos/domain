# Design: issue-32.4-sdk-typescript-from-openapi

## Contexto

Una vez que el server tiene OpenAPI spec (32.3), el siguiente paso
natural es generar el SDK para que el dashboard (y futuros
clientes) lo consuman sin escribir boilerplate. La opción manual
(50+ métodos con tipos copiados) es bug-prone: cualquier cambio en
el server requiere sync manual.

El SDK se inspira en `stripe-node`, `@aws-sdk/client-...`, y
`tRPC` (que es type-only). La generación automática mantiene
paridad y permite que el server evolucione sin romper clientes.

## Decisión arquitectónica

**Estrategia:** `openapi-typescript-codegen` para generar SDK
type-only desde el OpenAPI + tests E2E con `msw` (mock server)
+ publicación a npm.

1. **Generador:** `openapi-typescript-codegen` (paquete npm).
   - Input: `openapi.json`.
   - Output: `sdks/typescript/src/` con archivos `.ts` tipados
     (interfaces + clase `DomainClient` + métodos por endpoint).
   - Sin runtime: el código generado NO depende del generador.
     Solo de `fetch` o `node-fetch`.

2. **Estructura del package:**
   ```
   sdks/typescript/
     package.json          ← @tudominio/domain-sdk
     tsconfig.json
     src/
       index.ts            ← re-exports
       client.ts           ← DomainClient
       models/             ← tipos
       services/           ← Observations, Projects, etc.
     test/
       e2e/                ← tests con msw
     README.md
   ```

3. **CLI de generación:**
   ```bash
   npx openapi-typescript-codegen \
     --input https://api.tudominio.com/api/v1/openapi.json \
     --output ./src \
     --client fetch
   ```
   O desde local: `--input ./docs/swagger.json`.

4. **Autenticación dual:** la clase `DomainClient` tiene 2 modos
   en su config:
   ```typescript
   interface ClientConfig {
     baseURL: string;
     apiKey?: string;          // → Authorization: Bearer
     getSessionCookie?: () => string | undefined;  // → credentials: include
     fetch?: typeof fetch;     // para tests (inyectar msw)
   }
   ```
   En cada request: si `apiKey` está, agregar header. Si
   `getSessionCookie` está, `credentials: 'include'`. Si ambos
   están, `apiKey` gana (es para service-to-service).

5. **Response union pattern (Escenario 7):** el generador
   openapi-typescript-codegen por defecto retorna
   `Promise<Observation>` y `throw` en error. Para forzar el
   pattern `ok: true | ok: false`, se necesita un custom
   transformer o wrapper post-generation. Decisión: usar el
   default (throw on error) por simplicidad. Documentar que el
   caller debe usar try/catch. Si en el futuro se quiere union,
   es un wrapper opt-in.

6. **Testing E2E con msw:**
   ```typescript
   import { setupServer } from 'msw/node';
   import { rest } from 'msw';

   const server = setupServer(
     rest.get('*/observations', (req, res, ctx) => {
       return res(ctx.json([{id: 'obs_1', content: 'test'}]));
     })
   );

   it('lists observations', async () => {
     const client = new DomainClient({baseURL: 'http://api.test', apiKey: 'sk_test'});
     const obs = await client.observations.list();
     expect(obs).toHaveLength(1);
   });
   ```

7. **CI pipeline (`.github/workflows/sdk.yml`):**
   - Trigger: push a `main` o tag `v*`.
   - Steps: download latest spec (artifact de release) →
     `make sdk-generate` → `npm ci` → `npm run typecheck` →
     `npm test` → si tag, `npm publish`.

8. **Versionado:** el SDK sigue semver. Cada release del server
   bump-ea el SDK (major si breaking, minor si additive).

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | SDK escrito a mano | Insostenible: 50+ endpoints. |
| B | `openapi-generator` (el viejo) | Genera código verboso, runtime client incluido. Más complejo de mantener. |
| C | tRPC (type-only, sin OpenAPI) | Requiere que el server también sea TS. El server es Go. |
| D | GraphQL Codegen | No aplica, el server es REST. |
| E | Publicar a un registry privado (no npm público) | El user quiere npm público (`@tudominio/domain-sdk`). Decisión confirmada en REQ-32. |

## Por qué openapi-typescript-codegen gana

- **Output type-only:** sin runtime inflado, ~5KB de código
  generado por endpoint.
- **Comunidad activa:** mantenido por Ferdium, ~3K stars, soporte
  OpenAPI 3.0/3.1.
- **Compatible con msw:** el output usa `fetch`, fácil de mockear.
- **Customizable:** se puede post-procesar con un script si el
  default no alcanza.

## Detalle de implementación

- `sdks/typescript/` con la estructura de arriba.
- `Makefile`:
  ```makefile
  sdk-generate:
      cd sdks/typescript && \
        npx --yes openapi-typescript-codegen \
          --input $(OPENAPI_URL) \
          --output ./src \
          --client fetch
  sdk-test: sdk-generate
      cd sdks/typescript && npm ci && npm test
  sdk-publish: sdk-test
      cd sdks/typescript && npm version $(VERSION) && npm publish --access public
  ```
- GitHub Action: `.github/workflows/sdk.yml` con los steps.

## Riesgos

- **R1:** El generador cambia su output entre versiones →
  diffs raros. **Mitigación:** pinear la versión en package.json
  (`openapi-typescript-codegen@0.x.y`).
- **R2:** npm publish requiere auth. **Mitigación:** usar
  `NPM_TOKEN` secret en GitHub Actions.
- **R3:** El SDK crece a 50+ métodos, el bundle size del
  dashboard crece. **Mitigación:** tree-shaking (el generador
  produce imports por servicio, no monolítico). El dashboard
  solo importa `Observations` si usa ese.
