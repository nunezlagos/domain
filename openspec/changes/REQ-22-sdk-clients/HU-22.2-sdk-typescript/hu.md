# HU-22.2-sdk-typescript

**Origen:** `REQ-22-sdk-clients`
**Persona:** integrator
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer TypeScript/JavaScript integrando Domain
**Quiero** un SDK TypeScript oficial con tipos exhaustivos publicado a npm
**Para** consumir el API con autocomplete y type-safety en Node/browser/Deno

## Criterios de aceptación

### Escenario 1: Generación + dual ESM/CJS

```gherkin
Dado que existe `openapi/v1.yaml`
Cuando ejecuto `pnpm sdk:generate`
Entonces se genera `sdks/typescript/src/` con tipos completos
Y se buildea con `tsup` produciendo dist con ESM (.mjs) y CJS (.cjs) + .d.ts
Y el paquete `@domain/sdk` puede importarse desde Node, browser, Deno
```

### Escenario 2: Cliente unificado

```gherkin
Dado que existe `DomainClient`
Cuando hago
  ```ts
  import { DomainClient } from "@domain/sdk";
  const client = new DomainClient({ baseUrl: "...", apiKey: "..." });
  const list = await client.observations.list({ projectId });
  ```
Entonces `list` es typado como `Observation[]`
Y errores son tipados como `DomainError` discriminated union
```

### Escenario 3: Pagination async iterator

```gherkin
Dado que endpoint list es paginado
Cuando hago `for await (const obs of client.observations.iterAll())`
Entonces el async iterator pagina transparente
```

### Escenario 4: Retry + abort

```gherkin
Dado que pasamos `AbortSignal`
Cuando se cancela mientras retry pendiente
Entonces se aborta inmediatamente y se rejeta con `DomainAbortError`
```

### Escenario 5: Publicación npm

```gherkin
Dado que API publica v1.2.3
Cuando CI corre
Entonces se publica `@domain/sdk@1.2.3` en npm
Y se actualiza `docs.domain.sh/sdk/typescript`
```

## Análisis breve

- **Qué pide:** generator (`openapi-typescript`), build dual con tsup, publish npm
- **Esfuerzo:** M
- **Riesgos:** fetch polyfill en Node <18; tipos generados verbose
