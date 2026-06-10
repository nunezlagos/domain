# REQ-22-sdk-clients: SDKs oficiales del HTTP API de Domain en Python, TypeScript y Go.

**Estado:** activo
**Creado:** 2026-06-06
**Fase:** F4

## Descripción

Clientes oficiales del API HTTP (REQ-13) generados desde OpenAPI spec, con typings completos, autenticación por API key, retries, pagination helpers y publicación a npm, PyPI y proxy.golang.org.

## Criterios de éxito

- OpenAPI 3.1 spec generada desde el código Go (source of truth) y versionada por release
- SDK Python (`domain-sdk`) generado con typing completo, soporte async/sync, publicado en PyPI
- SDK TypeScript (`@domain/sdk`) con tipos exhaustivos, ESM + CJS, publicado en npm
- SDK Go (`go.domain.sh/sdk`) idiomatic con context.Context, publicado vía proxy.golang.org
- Pagination helpers, retry con backoff, error tipado con códigos del API
- CI publica nueva versión de los 3 SDKs cuando se taggea release del API

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-22.1-sdk-python | proposed | SDK Python desde OpenAPI, async/sync, typing, publish a PyPI |
| issue-22.2-sdk-typescript | proposed | SDK TS con tipos, ESM+CJS, publish a npm |
| issue-22.3-sdk-go | proposed | SDK Go idiomatic, context.Context, publish proxy.golang.org |
