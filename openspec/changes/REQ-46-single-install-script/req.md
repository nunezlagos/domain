# REQ-46-single-install-script

**Estado:** activo
**Creado:** 2026-06-20
**Fase:** F4

## Descripción

Consolidar el bootstrap del stack domain-services en un único `services/install.sh` que el operador corre en el VPS. Reemplaza la trinidad actual (`install-vps.sh` + `scripts/deploy-vps.sh` + `.env.vps`) por un solo script idempotente que genera UUIDs para credenciales en install fresco, preserva credenciales en reinstall, y las imprime al final.

## Justificación

- Un solo punto de entrada reduce confusión sobre "qué script corro y dónde"
- No requiere tooling local (sshpass, rsync) — corre en el VPS directamente
- Credenciales en formato UUID v4 son seguras por default, no requiere que el operador invente passwords
- Reinstall preserva credenciales → operador nunca pierde acceso si re-corre el script
- Print final asegura que el operador tiene las credenciales sin tener que `cat .env`

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-46.1-install-sh-uuid-credentials | propuesta | services/install.sh con UUID gen + preservado + print |

## No-objetivos

- Rotación automática de credenciales (manual si el operador quiere)
- Vault externo de secrets (overkill para self-hosted)
- Certs válidos Let's Encrypt (HU separada)
- Multi-VPS deploys (HU separada)