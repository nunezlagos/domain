# Design: issue-22.2-sdk-typescript

## Decisión arquitectónica

**Types gen:** `openapi-typescript` v7+ (genera solo types, no client)
**Cliente:** manual delgado sobre `fetch` (Node 18+, browser, Deno)
**Build:** tsup (esbuild)
**Test:** vitest + MSW

## Estructura repo

```
sdks/typescript/
  package.json    # exports field { ".": { "import": "./dist/index.mjs", "require": "./dist/index.cjs" } }
  tsconfig.json
  tsup.config.ts
  src/
    index.ts
    client.ts
    types.ts         # re-export generated
    errors.ts
    pagination.ts
    _generated/types.ts  # openapi-typescript output
  test/
```

## TDD plan

1. Vitest + MSW: list endpoint mocked → typed response
2. AbortSignal cancela durante retry
3. Async iterator pagina hasta agotar
4. tsc strict pasa
5. Smoke Deno + Node + browser (vite preview)
