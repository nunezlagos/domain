# Proposal: issue-25.14-rls-http-wiring

## Intención

Cerrar el gap entre la RLS ya aplicada (issue-25.5 + migration 000085) y los handlers HTTP/MCP que hoy no corren dentro de tx con `SET LOCAL`. Sin este wireup, la RLS rompe producción.

## Scope

**Incluye:**
- Helper `txctx.WithTxContext` / `TxFromContext` para inyectar la tx en `context.Context`.
- Modificación de `apikey.Middleware` para abrir tx + SET LOCAL post-auth.
- Refactor de servicios que tocan `observations` y `sessions` para usar la tx del ctx (con fallback a pool legacy).
- Wireup equivalente en el MCP server.
- Test E2E con testcontainers + HTTP server real que verifica aislamiento cross-org.
- Test sabotaje: handler con bug que ignora tx → RLS bloquea.

**No incluye:**
- RLS en tablas no-Tier-1 (knowledge, prompts, etc.) — fuera del scope 25.5/25.14.
- Refactor de los 9 handlers que leen Principal pero NO tocan tablas RLS (sin urgencia).
- BYPASSRLS para app_user (anularía defense-in-depth).

## Enfoque técnico

1. **Helper context-based** (`txctx`): `WithTxContext` + `TxFromContext`. Patrón Go idiomático, 0 magic.
2. **Middleware HTTP**: después de autenticar, abre tx con `BeginTx` + ejecuta `set_config('app.current_org_id', ..., true)` + `set_config('app.current_user_id', ..., true)`. Inyecta en ctx. `defer Rollback` por si handler no hace Commit.
3. **Repos con fallback**: cada repo intenta `TxFromContext`; si nil, usa el pool con el filtro de org en WHERE (legacy path, defensa de profundidad 2).
4. **MCP**: helper `WithOrgTxForPrincipal(ctx, pool, principal, fn)` que los tools usan para envolver cada call.
5. **Tests**: testcontainers + `httptest.NewServer` con el router real. Cross-org isolation assertions.

## Riesgos

| Riesgo | Mitigación |
|---|---|
| Regresión masiva en 11 archivos | Refactor atómico por archivo, suite verde por commit |
| MCP no es HTTP — wireup diferente | Helper dedicado en `mcp/server/` |
| Handler hace su propio `BeginTx` dentro de la tx del middleware | Documentar regla: "si `txctx.TxFromContext` no nil, usala; no abrir otra" |
| Performance: cada request abre tx | `BeginTx` con TxOptions{IsoLevel: pgx.ReadCommitted} (default), overhead despreciable |
| Tests E2E lentos con testcontainers | Marcados con build tag `integration`; CI corre serial |

## Testing

- **Unit:** `txctx/context_test.go` (round-trip del ctx).
- **Integration (E2E HTTP):** `txctx_http_e2e_test.go` — levanta testcontainer, crea 2 orgs + 2 keys + 2 obs, hace GET con cada key, asserts.
- **Integration (E2E MCP):** `mcp_memory_e2e_test.go` — stdio MCP, valida que `domain_memory_get` filtra por org.
- **Sabotaje:** handler que ignora tx y usa pool → RLS bloquea (0 rows).

## Rollback plan

Si el wireup causa regresión en producción:
1. Revert de los commits 3-6 (mantiene migration 000085 + tests, quita wireup).
2. Forward fix: bug puntual en el handler que rompió.
3. La migration 000085 down desactiva RLS en observations/sessions (rollback completo).

## Out of scope (futuro)

- RLS en `knowledge`, `prompts`, `skills`, `agents`, `flows`, `crons`, `webhooks`, `audit_diffs` (no existen), `api_key_scopes` (no existe). Estas tablas usan filtro de org en WHERE explícito en la app; la RLS no es estrictamente necesaria para defense-in-depth, pero podría aplicarse en una HU 25.15+.
- Tests de performance (<5% regression) — diferido a issue-27.4 benchmarks.
- Linter que detecte queries sin helper — diferido a issue-25.13.
