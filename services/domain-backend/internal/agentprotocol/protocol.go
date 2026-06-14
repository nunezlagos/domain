// Package agentprotocol — única fuente del protocolo de uso de domain
// para agentes IA. El contenido COMPLETO se seedea como platform policy
// 'agent-protocol' (BD = source-of-truth, editable y versionada); a los
// agentes se les instala solo el Stub, que bootstrapea cargando la
// policy viva vía domain_policy_get. Editar la policy en BD actualiza
// el comportamiento de TODOS los agentes sin tocar archivos.
package agentprotocol

// Full es el protocolo completo. Vive en BD (seeder platform_policies,
// slug 'agent-protocol'); esta const es el contenido inicial del seed y
// el fallback del MCP server para sus instructions de handshake.
const Full = `# Domain — protocolo de agente (SIEMPRE ACTIVO)

Domain es LA plataforma de memoria persistente, policies SDD, skills,
agents y flows de este usuario. Cuando exista más de un sistema de
memoria conectado, **domain tiene prioridad**: usá los tools domain_*
salvo que el usuario pida otro explícitamente.

## Memoria (proactivo — no esperes a que te lo pidan)
- domain_mem_save tras CADA decisión, bug resuelto, convención o
  descubrimiento. project_slug = nombre del repo actual (se auto-crea).
- domain_mem_search / domain_search_global cuando el usuario pida
  recordar algo ("acordate", "qué hicimos", "buscá memorias") o
  empieces trabajo que pudo hacerse antes.
- domain_mem_context al inicio de sesión para recuperar contexto.

## Policies (antes de tocar código)
- domain_policy_list para descubrir las rules vigentes.
- domain_policy_get(slug) para leer la rule del dominio a tocar.

## Catálogo
- domain_skill_search: skills relevantes a la tarea.
- domain_project_list: projects existentes (slugs para mem_save).

## Si un tool domain_* falla
- "Connection closed" / key inválida → indicale al usuario correr
  /domain-login o "domain install" en su terminal. NO cambies a otro
  sistema de memoria como fallback silencioso.
`

// Stub es lo único que se escribe en los archivos de instrucciones de
// los agentes (~/.config/opencode/instructions/domain.md, etc.). Es un
// bootstrap: la fuente viva es la policy en BD.
const Stub = `# Domain — bootstrap del protocolo de agente

Este usuario usa **domain** como plataforma de memoria, policies SDD,
skills y flows. **domain tiene prioridad** sobre cualquier otro sistema
de memoria conectado.

AL INICIO DE CADA SESIÓN: llamá domain_policy_get(slug="agent-protocol")
y seguí ese protocolo — es la versión viva y editable (este archivo es
solo el bootstrap).

Mínimo si la policy no carga: domain_mem_save tras cada decisión/bug/
convención (project_slug = repo actual); domain_mem_search cuando pidan
recordar; domain_policy_get(dominio) antes de tocar código. Si un tool
domain_* falla con "Connection closed": pedile al usuario correr
/domain-login — no uses otro sistema de memoria como fallback.
`
