# Design: HU-46.1-install-sh-uuid-credentials

## Decisión arquitectónica

**Un único script, ejecutado en el VPS, sin dependencias locales.** Razones:

1. El operador clona el repo en el VPS y corre `bash services/install.sh`. No necesita nada local (ni sshpass, ni rsync, ni credenciales en el dev).
2. La generación de credenciales vive en el script. Si re-corrés install, las preserva. Esto centraliza la lógica de secrets.
3. El print final es la ÚNICA forma de recuperar credenciales (no quedan en el dev). Es deliberadamente visible.

## Alternativas descartadas

- **Mantener install-vps.sh + deploy-vps.sh separados**: dos puntos de entrada, dos credenciales (local + VPS), confusión sobre cuál usar.
- **UUIDs vía `uuidgen` o `python -c`**: dependencias externas. `/dev/urandom` es universal.
- **Credenciales en Vault externo**: complica el deploy. Para un self-hosted VPS no se justifica.
- **Generar credenciales en cada install (sin preservado)**: rompe reinstalación, operador pierde acceso.

## Diagrama

```
OPERATOR                          VPS
────────                          ───
git clone <repo>     ─────►       cd /tmp/domain-services
                                   bash services/install.sh
                                                  │
                                                  ▼
                                  ┌─────────────────────────┐
                                  │ 1. Validate OS          │
                                  │ 2. Install docker       │
                                  │ 3. Clone/update repo    │
                                  │ 4. Generate .env        │
                                  │    - read existing?     │
                                  │    - gen UUIDs for new  │
                                  │ 5. Generate certs       │
                                  │ 6. Build + Up           │
                                  │ 7. Systemd timers       │
                                  │ 8. Print creds to stdout│
                                  └─────────────────────────┘
                                                   │
                                                   ▼
                                  ┌────────────────────────────────┐
                                  │ DOMAIN ADMIN CREDENTIALS       │
                                  │ Postgres:  uuid-...            │
                                  │ App User:  uuid-...            │
                                  │ MinIO:     uuid-...            │
                                  │ Backup GPG: uuid-...           │
                                  │ Dashboard: http://IP/          │
                                  │ Save these! (also in .env)     │
                                  └────────────────────────────────┘
```

## TDD plan

N/A — infra script. Testing = ejecutar el script en el VPS real y verificar.

## Riesgos y mitigación

| Riesgo | Mitigación |
|---|---|
| Operador no ve el print (logs en scrollback) | Print usa `>&2` (stderr) y `cat <<EOF` que aparece aún con pipes |
| Script ejecutado con sudo, pierde env vars | `sudo -E` o re-export explícito de PATH |
| `.env` con passwords viejos inseguros | No validación; el operador debe regenerar manualmente si quiere rotar |
| Falta de `uuidgen` en Ubuntu minimal | `/dev/urandom` siempre presente |