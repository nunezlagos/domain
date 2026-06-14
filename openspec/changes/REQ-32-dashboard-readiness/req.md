# REQ-32 — Dashboard Readiness (API lista para frontend separado)

> **Origen**: sesión 2026-06-12. Decisión del usuario: la Web UI NO va
> dentro de domain. Va a ser otro proyecto (dashboard) que consume el
> API de domain. Para que ese dashboard funcione, domain tiene que
> exponer su API REST con CORS, auth de sesión web (no solo Bearer
> API key), y un SDK TypeScript generado.

## Contexto

Domain hoy es 100% backend para CLIs y MCP. Para alimentar una webUI
React/Vue/Svelte separada hospedada en Vercel/Cloudflare Pages, hace
falta:

1. **Sesión web**: el dashboard maneja usuarios con login/email-OTP, no
   con API key directa. Necesita cookies o JWT con refresh.
2. **CORS**: hoy deshabilitado por convención (clientes son SDKs
   server-to-server). El dashboard es SPA en otro dominio →
   necesita CORS con allowlist.
3. **SDK auto-generado**: escribir 50+ endpoints a mano con `fetch` es
   masoquismo. Generar SDK TypeScript desde OpenAPI mantiene parity
   automática.

## Issues

| Issue | Slug | Esfuerzo | Descripción |
|-------|------|----------|-------------|
| 32.1 | `session-web-auth-cookies-jwt` | L | Capa de auth web sobre OTP existente: login con email → OTP → emite sesión (cookies httpOnly o JWT + refresh). Endpoint `/auth/me`. `/auth/login`, `/auth/logout`. Sesiones expiran (default 7d). Revoke endpoint. Coexiste con API key Bearer del MCP. |
| 32.2 | `cors-allowlist-configurable` | S | Habilitar CORS para `/api/v1/*` con allowlist de origins (env var `DOMAIN_CORS_ORIGINS=https://app.tudominio.com,https://dashboard.tudominio.com`). Default deny. Headers permitidos: Authorization, Content-Type. Métodos GET/POST/PATCH/DELETE/OPTIONS preflight. |
| 32.3 | `openapi-spec-generation` | M | Generar OpenAPI 3.0 desde anotaciones o desde los handlers Go. Publicar `openapi.json` en `/api/v1/openapi.json`. Versionado: cada release tag genera y publica el spec. |
| 32.4 | `sdk-typescript-from-openapi` | M | Pipeline (Makefile target o GitHub Action) que genera SDK TypeScript desde el OpenAPI. Publicable como npm package `@tudominio/domain-sdk`. Test E2E: SDK compila contra cada endpoint nuevo. |

## Prioridad: **media** (cuando dashboard arranque)

No es urgente HOY porque el dashboard es proyecto futuro. Pero estos
issues son **prerequisito** para que el dashboard se pueda construir
sin reescribir cosas.
