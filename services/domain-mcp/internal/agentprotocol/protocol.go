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

## Primer turn de cada sesión (REQ-45)
PRIMERO de todo, antes de responder, detectá en el cwd qué archivos
AI-rules existen (AGENTS.md, CLAUDE.md, .claude/CLAUDE.md, .cursorrules,
.windsurf/rules, .github/copilot-instructions.md, openspec/) y llamá:

    domain_session_bootstrap(
      cwd=<working directory absoluto>,
      git_remote=<git remote get-url origin, vacío si no es git>,
      git_branch=<git branch --show-current>,
      git_head=<git rev-parse HEAD>,
      existing_rules_files=[<paths relativos a cwd>],
    )

El response te dice:
- known=true → leé recent_observations + counts; si head.changed=true
  ejecutá git log <head.last_known>..<head.current> y persistí con
  mem_save lo que sea relevante. Si existing_rules_files tiene paths
  que NO matchean policies importadas previas (revisá con
  domain_project_policy_list), considerá leerlos con tu tool Read y
  llamar domain_project_policy_import_from_text por cada uno.
  Si counts.project_skill_count=0 → el proyecto no tiene skill de stack
  configurada. Sin interrumpir la conversación, leé los archivos de
  configuración del repo (package.json, composer.json, go.mod,
  pyproject.toml, Cargo.toml, etc.) y llamá domain_project_skill_register
  para crear la skill del stack. Esta acción es UNA VEZ por proyecto.
- known=false → preguntale al usuario los datos del suggestion +
  workflow del repo + estructura (mono-repo? servicios? migrations
  manuales?), llamá domain_session_register, y DESPUÉS arrancá un
  project index (REQ-62): domain_project_index_start →
  domain_project_index_submit con los archivos del manifest. El
  server clasifica y persiste como project_policies + knowledge_docs
  con source='seed_imported'. Esto deja al repo "indexado" en BD,
  estilo Cursor, y futuras sesiones tienen el contexto en memoria
  persistente sin volver al filesystem.

  Para projects ya conocidos: si domain_project_index_status devuelve
  has_run=false o el último run es de >7 días o de git_head distinto
  al actual, considerá re-indexar.

NO uses tools domain_* de proyecto (mem_save, policy_get, ...) sin
pasar el project_slug que sale de bootstrap/register.

NO pises archivos del repo (AGENTS.md, CLAUDE.md, etc.). El import
crea una copia versionada en BD; el archivo del usuario queda intacto.

## Memoria (proactivo — no esperes a que te lo pidan)
- domain_mem_save tras CADA decisión, bug resuelto, convención o
  descubrimiento. project_slug = el que devolvió bootstrap/register.
- domain_mem_search / domain_search_global cuando el usuario pida
  recordar algo ("acordate", "qué hicimos") o empieces trabajo que
  pudo hacerse antes.
- domain_mem_context al inicio de sesión para recuperar contexto.
- domain_prompt_capture(content, project_slug?) UNA vez
  por turn, con el raw_text del usuario.

## Policies (antes de tocar código)
- domain_policy_get(slug, project_slug=<actual>) → resolver
  jerárquico: si hay project_policy para ese slug en este proyecto,
  la devuelve; si override_platform=false trae también la platform
  como contexto adicional; si no hay, fallback a platform.
- domain_project_policy_set para registrar reglas que aprendiste del
  proyecto (workflow=pr/mr, migrations manuales, tech_stack, etc.)
  con source='llm_generated'.

## Catálogo scoped
- domain_project_skill_list(project_slug, include_globals=true) →
  skills del proyecto + globales. Para registrar una skill que
  aprendiste del proyecto: domain_project_skill_register.
- domain_project_repo_list(project_slug) → si ambiguous=true (>1
  remoto sin default), preguntale al usuario antes de pushear.

## Issues vs Tickets (REQ-56)
- **issue** (workflow SDD formal con Gherkin): se crea con
  domain_issue_create_start (alias legacy: domain_hu_create_start).
  Usar para HUs/requirements que necesitan criterios de aceptación
  estructurados (given/when/then).
- **ticket** (operativo, tipo Jira/Linear): se crea con
  domain_ticket_create. Para bugs, tasks, features simples, sin
  Gherkin. Soporta status workflow kanban, comments, sync externo.
- Si un ticket implementa una issue formal, vincularlos con
  domain_ticket_link_issue(ticket_id, issue_id). La BD es source of
  truth de ambos.

## Si un tool domain_* falla
- "Connection closed" / key inválida → indicale al usuario correr
  el installer. NO cambies a otro sistema de memoria como fallback
  silencioso.
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
