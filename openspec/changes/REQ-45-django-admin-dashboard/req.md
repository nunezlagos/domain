# REQ-45-django-admin-dashboard

**Estado:** activo
**Creado:** 2026-06-20
**Fase:** F4

## Descripción

Reemplazo del admin panel (sunset de REQ-41 Angular CoreUI en commit `5d03726`) con un servicio **Django full-stack** dockerizado. Django permite HTML + backend + ORM en una sola app Python, alineado con el preference del operador.

HU-45.1 deja el **placeholder healthcheck-able**: container `domain-admin` respondiendo HTTP 200 en `/` con página default, hooked al routing de Caddy y al Makefile existente. Las HUs siguientes (45.2+) agregan ORM contra el Postgres compartido, auth, vistas reales (members/usage/audit), reportes.

## Justificación

- REQ-41 (Angular) se sunsetó en `5d03726` pero el slot `services/domain-admin/` quedó vacío. El Caddyfile y el Makefile siguen apuntando ahí.
- Django full-stack matchea el preference explícito del operador.
- ORM built-in reduce capas cuando llegue el momento de hablar con Postgres.
- Admin UI built-in (`django.contrib.admin`) es un acelerador futuro.

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-45.1-django-default-page | propuesta | Placeholder Django en :80 con página HTML default. Commit dedicado. |
| HU-45.2 | (futuro) | ORM: Django models contra el Postgres compartido (`domain` DB) |
| HU-45.3 | (futuro) | Auth: OTP contra el endpoint del MCP, sesión Django |
| HU-45.4 | (futuro) | Vistas: dashboard, members, usage, audit, tickets |
| HU-45.5 | (futuro) | django.contrib.admin habilitado para CRUD rápido |

## No-objetivos

- Reemplazar el backend MCP (el binario Go sigue siendo el motor de la plataforma)
- Migrar endpoints REST del MCP al ORM de Django (HU futura, decisión por tomar)
- Reescribir vistas que ya tienen UI en otra plataforma

## Convenciones

- Service name: `domain-admin` (consistente con REQ-41)
- Container port: 80 (interno, no publicado al host)
- Network: `domain_internal` (igual que el resto del stack)
- Imagen: `domain-admin:local`
- Sin TLS (Caddy delante)