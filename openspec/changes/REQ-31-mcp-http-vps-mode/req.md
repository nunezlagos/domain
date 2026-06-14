# REQ-31 — MCP HTTP Mode (refactor para VPS multi-tenant)

> **Origen**: sesión 2026-06-12. Decisión arquitectónica: domain pasa
> de "BD local del usuario" a "backend multi-tenant en VPS Contabo".
> Esto requiere que `domain-mcp` deje de conectarse directo a Postgres
> y empiece a hablar SOLO HTTP con el server domain remoto. CRÍTICO
> para seguridad: si los clientes tienen DSN directo de Postgres, el
> día que uno filtre su `.env` el atacante tiene acceso casi-DBA a la
> BD entera de TODOS los clientes.

## Contexto

Hoy `domain-mcp` hace:
```
domain-mcp (cliente)
  └── DSN directo → Postgres (expuesto a internet en VPS)
```

Necesita pasar a:
```
domain-mcp (cliente)
  └── HTTPS API key → Server domain en VPS (puerto 443)
        └── pool localhost privado → Postgres (NO accesible desde internet)
```

El refactor tiene que mantener compatibilidad con el modo local (DSN)
para que el flujo de desarrollo no se rompa: env var
`DOMAIN_REMOTE_URL=https://...` activa modo remoto; sin ella, sigue
modo local. Mismo binario, mismas tools.

Bug detectado HOY: el server HTTP no está abriendo el listener
correctamente (ver REQ-29.3). Esto es bloqueante para REQ-31: si el
HTTP no escucha, no hay modo remoto.

## Issues

| Issue | Slug | Esfuerzo | Descripción |
|-------|------|----------|-------------|
| 31.1 | `mcp-http-client-mode` | L | Refactor de `domain-mcp` para detectar `DOMAIN_REMOTE_URL`. Si está, no abre pool a Postgres; en su lugar, cada handler de tool MCP hace HTTP call al endpoint REST equivalente. Token Bearer = `DOMAIN_API_KEY`. Reintentos + circuit breaker. Diff con modo local enforceado por tests (cada tool MCP corre en ambos modos contra fixtures). |
| 31.2 | `endpoint-coverage-rest-audit` | M | Auditoría: cada tool MCP `domain_*` debe tener handler HTTP REST equivalente. Generar tabla mapping `tool → endpoint`. Crear los endpoints faltantes. CI test que falla si un tool MCP no tiene su par HTTP. |
| 31.3 | `vps-deploy-caddy-https` | M | Documentar + scriptear deploy a Contabo VPS con Caddy reverse-proxy + Let's Encrypt automático. Dominio + DNS. Health checks. Backup nightly. Postgres NO accesible externamente (solo localhost del VPS). MinIO firmando presigned URLs, no expuesto directo. |
| 31.4 | `client-config-remote-url` | S | `domain install --remote-url https://api.tudominio.com` configura `~/.config/domain/env` para modo remoto. Quita `DOMAIN_DATABASE_URL`, agrega `DOMAIN_REMOTE_URL`. Skip de pasos del install que solo aplican a server (docker, migrate, seed). |

## Prioridad: **alta** (bloqueante para SaaS)

Sin REQ-31, mover domain al VPS expone Postgres a Internet con
credenciales en cada cliente. Es la barrera de entrada para
multi-tenant.
