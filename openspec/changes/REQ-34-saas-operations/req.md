# REQ-34 — SaaS Operations (operar sin tiers ni paywall)

> **Origen**: sesión 2026-06-12. Decisiones del usuario sobre cómo
> manejar lifecycle de datos en multi-tenant sin venir con sistema
> comercial complejo: self-service export, GDPR delete simple, SMTP
> real, backups nightly.

## Contexto

Para que domain sea operable como SaaS, hace falta:

1. **Self-service backup**: cada usuario exporta SUS datos. No hay
   backup/restore granular per-tenant del lado de domain (sería
   complejidad enorme). Lo que sí: backup global del Postgres entero
   por si pasa algo catastrófico.

2. **GDPR delete**: comando admin que borre todo de una org. NO un
   sistema. Con `ON DELETE CASCADE` ya hecho en el schema, son ~20
   líneas Go.

3. **SMTP real**: Mailpit es solo dev. Producción necesita Resend
   (free tier 100/día), AWS SES o Sendgrid para que los OTP lleguen
   a emails reales de clientes.

4. **Backups nightly**: pg_dump → Backblaze B2 (~$0.50/mes). No es
   crítico ni urgente, pero el día que algo se rompa querés tenerlo.

## Issues

| Issue | Slug | Esfuerzo | Descripción |
|-------|------|----------|-------------|
| 34.1 | `self-service-export-zip` | M | Endpoint `GET /api/v1/export` que devuelve ZIP con: observations, prompts, knowledge_docs, configs (skills/agents/flows propios) de la org del caller. Stream directo, no almacena. Audit log entry. Format JSON-lines comprimido. Restore del lado del usuario (no provee endpoint de import directo — el usuario decide qué hacer con el ZIP). |
| 34.2 | `org-delete-gdpr-cascade` | S | Comando admin `domain org delete <slug> --confirm` + endpoint `DELETE /api/v1/admin/orgs/{id}`. Usa CASCADE de Postgres (FKs ya están). Limpia prefijo S3 de la org. Idempotente. Audit trail ANTES del delete. Requiere flag `--confirm` o segundo factor. |
| 34.3 | `smtp-real-providers` | S | Adapter SMTP: Resend (default), AWS SES, Sendgrid. Configurable por env var (`DOMAIN_SMTP_PROVIDER=resend|ses|sendgrid` + credenciales). Mailpit queda SOLO en `docker-compose.yml` dev. En prod, no se levanta. |
| 34.4 | `backup-nightly-b2` | S | Cron nightly: `pg_dump` del Postgres entero → upload a Backblaze B2 (S3-compatible, ~$0.005/GB/mes). Retention 30 días. Notificación por email si falla 2 noches seguidas. Documentar el restore manual paso a paso. |
| 34.5 | `audit-log-multi-tenant` | S | Asegurar que `audit_log` registra origin_org_id en cada acción crítica. Endpoint admin para query `audit_log` por org. Útil para soporte (cliente reporta "no veo X") y forense (algo se borró ¿quién?). |

## Prioridad: **media** (antes del primer cliente externo)

Self-service export y GDPR delete son requisitos legales en muchas
jurisdicciones. SMTP real es requisito funcional. Backups son
"insurance".
