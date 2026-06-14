# Proposal: issue-09.1-cloud-config-auth

## Intención

Permitir al usuario configurar la conexión cloud (server URL, token, modo insecure) y persistirla en `cloud.json`. Soporte de configuración via CLI y variables de entorno con la jerarquía estándar: env var > config file > defaults.

## Scope

**Incluye:**
- `cloud.json` en `$ENGRAM_CONFIG_DIR` o `~/.config/engram/cloud.json`
- `GetCloudServer()`, `GetCloudToken()`, `IsInsecureNoAuth()` functions
- `engram cloud config --server URL` CLI command
- Soporte de env vars: `ENGRAM_CLOUD_TOKEN`, `ENGRAM_CLOUD_INSECURE_NO_AUTH`
- Token sanitizado en output y logs

**No incluye:**
- Enrollment o upgrade (issue-09.2)
- Cloud server API (issue-09.3)
- Dashboard (issue-09.4)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Config file | JSON en `~/.config/engram/cloud.json`, permisos 0600 |
| Env var priority | ENV > file > default |
| Default server | `https://api.memoria.dev` (future; ahora requiere explicit flag) |
| Token display | Reemplazar con `***` excepto últimos 4 chars en output específico |
| Lazy loading | Config se carga en primera llamada, se cachea en memoria |

