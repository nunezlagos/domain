# REQ-29 — Install Quick Fixes

> **Origen**: sesión 2026-06-12. Bugs detectados durante el día de uso real
> (instalación E2E + smoke MCP + observación del usuario). Son fixes
> chicos pero con impacto alto en la experiencia: irritación cotidiana,
> bugs silenciosos.

## Contexto

Tres problemas reales encontrados:

1. **Spam de backups del `.env`** del repo y del `opencode.json` global:
   60+ archivos en una semana. Cada corrida del install genera un
   backup aunque el contenido no haya cambiado.
2. **`domain install` sin guard de cwd**: si se corre desde un dir
   random (no el repo), `upsertEnvFile(".env", ...)` puede tocar el
   `.env` ajeno del proyecto donde está parado el usuario.
3. **`domain.service` "active" pero sin listener HTTP**: detectado hoy
   (ss -ltn vacío en :8000 aunque el proceso esté vivo y loguee
   "runtime config refreshed" en loop). Fallo silencioso del boot del
   server: el resto del binario corre, pero el listener no se abre y
   nadie aborta. RUNS UNDERCOVER COMO BUG.

## Issues

| Issue | Slug | Esfuerzo | Descripción |
|-------|------|----------|-------------|
| 29.1 | `install-cwd-guard` | S | `domain install` detecta si está en el repo (chequea `.env.example` + `docker-compose.yml`). Si NO está, aborta con mensaje claro: "corré `bash install.sh` o pasá `--src /path/al/repo`". Test sabotaje: ejecutar desde `/tmp` debe fallar limpio sin tocar nada. |
| 29.2 | `backup-deduplication-by-hash` | S | Antes de escribir cualquier `.bak.*` / `.backup-*`, calcular hash del archivo original y comparar con el último backup. Si idéntico, no generar archivo nuevo. Aplica a `.env`, `opencode.json`, `.mcp.json`, `credentials.json`. Test sabotaje: 10 corridas seguidas sin cambios = 1 backup, no 10. |
| 29.3 | `server-http-listener-boot-fail-loud` | M | Investigar y arreglar el caso `domain.service` corriendo pero `/health` 000 y sin listener en :8000. Hipótesis A: error de bind silenciado por algún `_ = err`. Hipótesis B: la goroutine del HTTP server crashea y el process group sigue vivo. Fix: panic o log.Fatal explícito si `srv.ListenAndServe()` retorna `!= http.ErrServerClosed`. Test sabotaje: si listener falla, service queda en estado `failed`, no `active`. |
| 29.4 | `opencode-json-untracked-permanent` | XS | `opencode.json` (formato local con paths absolutos) está en `.gitignore` (commit hecho hoy). Agregar test que falle si alguna vez vuelve a ser tracked. Pre-commit hook opcional. |

## Prioridad: **alta** (entrega esta semana)

Estos son micro-fixes con valor inmediato. Resolverlos quita ruido
visible y un bug latente serio (HTTP listener silencioso).
