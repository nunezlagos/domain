# Proposal: HU-22.2-sdk-typescript

## Intención

SDK TypeScript oficial `@domain/sdk` con types completos generados desde OpenAPI, build dual ESM/CJS, retries, pagination async iterator, publicado a npm.

## Scope

**Incluye:**
- Generator `openapi-typescript` para types + thin client manual
- tsup build dual ESM/CJS + d.ts
- AbortSignal support nativo
- Retries con `p-retry`
- Async iterator pagination
- `package.json` con exports condicionales
- CI publish npm on tag

**No incluye:**
- React hooks package (separado posible: `@domain/react`)
- Vue/Svelte adapters

## Enfoque técnico

1. `openapi-typescript` v7+ genera solo types (no client) — más control
2. Cliente manual delgado wrapping `fetch` (nativo en Node 18+)
3. Build con tsup (esbuild under the hood)
4. tsx tests + vitest

## Riesgos

- Node <18 sin fetch: requerir Node 18+ en package.json engines
- Tipos enormes para schemas grandes → exportar subpaths

## Testing

- vitest con MSW (Mock Service Worker)
- Integration contra dev compose
- Smoke en Deno
- typecheck tsc strict
