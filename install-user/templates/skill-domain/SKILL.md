---
name: domain
description: Bootstrap del protocolo de uso de domain MCP. Carga la policy 'agent-protocol' desde la BD viva al inicio de cada sesión. Usar cuando el usuario mande cualquier mensaje y haya tools domain_* disponibles.
---

# domain — bootstrap

El protocolo completo vive en BD como policy `agent-protocol` (editable,
versionada). Este archivo es solo el bootstrap.

## Al cargar este skill

1. `domain_policy_get(slug="agent-protocol")` → sigue ese protocolo.
2. Si la policy no carga (server caído / key inválida): sigue el mínimo
   abajo y pídele al usuario `./install-user.sh --uninstall && ./install-user.sh`.

## Mínimo si la policy no carga

- `domain_mem_save` tras cada decisión, bug fix, convención, descubrimiento.
  `project_slug` = nombre del repo actual (se auto-crea).
- `domain_mem_search` cuando el usuario pida recordar o vayas a hacer algo
  que pudo hacerse antes.
- `domain_mem_context` al inicio de la sesión.
- `domain_policy_get(slug=<dominio>)` antes de tocar código del dominio.
- Si un tool `domain_*` falla con "Connection closed": pídele al usuario
  correr el installer. NO uses otro sistema de memoria como fallback.
