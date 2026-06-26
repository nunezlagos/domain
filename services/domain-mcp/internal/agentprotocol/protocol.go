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
  Si counts.project_skill_count=0 → el proyecto no tiene skill(s) de
  stack configuradas. Sin interrumpir la conversación, detectá el/los
  stack(s) y creá UNA skill por stack (ver "Stack de proyecto" abajo).
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
  proyecto (workflow=pr/mr, migrations manuales, tech_stack, etc.).
  ANTES de persistir, confirmá con el usuario (ver "Crear skills/
  políticas" abajo).

## Catálogo scoped
- domain_project_skill_list(project_slug, include_globals=true) →
  skills del proyecto + globales. Para registrar una skill que
  aprendiste del proyecto: domain_project_skill_register (confirmá antes).
- domain_project_repo_list(project_slug) → si ambiguous=true (>1
  remoto sin default), preguntale al usuario antes de pushear.

## Crear/editar skills/políticas (confirmá ANTES de persistir + scope)
Toda skill o policy nueva — Y toda edición de una ya activa — pasa por
confirmación humana SÍNCRONA antes de escribirse, sin importar el origen
(la detectaste, el usuario la pidió, o la inferiste de un patrón). NO
persistas a ciegas.

¿Hay humano para confirmar? Si estás respondiendo a un mensaje del usuario
en un turn conversacional → SÍ, confirmá (pasos 1-6). Si estás ejecutando
una fase del pipeline SDD invocada por el orchestrator sin intervención
humana (modo headless/batch) → NO interrumpas: usá domain_propose_* (paso 7).

1. Armá el contenido completo (slug, name, body/content, kind si es policy).
2. Inferí el SCOPE y proponelo:
   - interna (project-scoped, project_id=<proyecto>): es lo DEFAULT. Todo
     lo específico del repo — stack, workflow, convención propia, comando
     recurrente. Casi todo cae acá.
   - global (project_id NULL): SOLO si es una verdad universal aplicable a
     CUALQUIER proyecto de la org. Es raro: las globales suelen venir del
     seed curado. Ante la duda → interna.
3. Mostrale al usuario el contenido + el scope propuesto y pedí confirmación
   explícita (en Claude Code: AskUserQuestion; en otro cliente: una pregunta
   directa). Ofrecé tres opciones: confirmar / modificar / descartar.
4. Si elige modificar: aplicá los ajustes, volvé a MOSTRAR el contenido ya
   ajustado (con los cambios reflejados, no solo describirlos) y re-ofrecé
   confirmar / modificar / descartar. Repetí este ciclo cuantas veces haga
   falta hasta que confirme o descarte. NO persistas nada en medio del ciclo.
5. Si descarta: no escribas nada y seguí con la conversación.
6. Solo al confirmar, persistí YA ACTIVA con el scope acordado:
   - skill nueva: domain_project_skill_register (interna) o
     domain_skill_create (global). Edición: domain_skill_edit.
   - policy nueva: domain_project_policy_set (interna) o
     domain_platform_policy_create (global). Edición de global:
     domain_platform_policy_edit. Todas crean/dejan activas (proposed=false).
   - Tras persistir, dejá traza de la aprobación con domain_mem_save (qué se
     creó/editó, que el usuario lo confirmó, y por qué) — audit liviano.
7. domain_propose_skill / domain_propose_policy (proposed=true, review
   diferido) son SOLO para modo headless/batch donde NO hay un humano que
   confirme en el momento. Con usuario presente, confirmá y creá activa —
   no dejes proposals colgadas.

## Issues vs Tickets (REQ-56)
- **issue** (workflow SDD formal con Gherkin): se crea con
  domain_issue_create_start (alias legacy: domain_hu_create_start).
  Usar para HUs/requirements que necesitan criterios de aceptación
  estructurados (given/when/then).
- **ticket** (operativo, tipo Jira/Linear): se crea con
  domain_ticket_create. Para bugs, tasks, features simples, sin
  Gherkin. Soporta status workflow kanban, comments, sync externo.
  Nace en status 'backlog'. Si el ticket corresponde a algo ya decidido
  o en curso, movelo al estado real con domain_ticket_change_status
  (todo/in_progress/...) — no lo dejes en backlog por inercia. Usá
  change_status (no un update directo) para que quede en el status_history.
- Si un ticket implementa una issue formal, vincularlos con
  domain_ticket_link_issue(ticket_id, issue_id). La BD es source of
  truth de ambos.

## Stack de proyecto (skills project-scoped, monorepo-aware)
Un proyecto puede tener 1 o N stacks. NO asumas que el cwd es el único.
Pasos para configurar (UNA vez por stack, no por sesión):

1. Detectá TODOS los roots de stack del repo, no solo el del cwd:
   - Buscá archivos de manifiesto en el root Y en subdirectorios:
     package.json, composer.json, go.mod, pyproject.toml, Cargo.toml,
     Gemfile, pom.xml, build.gradle, *.csproj.
   - Leé .gitmodules si existe: cada submódulo es un root candidato con
     su propio stack y su propio path.
   - Un repo con services/api (go.mod) + services/web (package.json) =
     2 stacks → 2 skills. Un monorepo con N paquetes = N stacks.
2. Para cada stack detectado, mirá domain_project_skill_list para no
   duplicar. Si ya existe la skill de ese stack, saltala.
3. Por cada stack faltante, armá la skill (slug "<framework>-<major>-stack",
   prefijando el subpath si no está en el root: "web-nextjs-15-stack",
   "api-go-1-stack"; content = role + patrones_obligatorios + antipatrones
   + gotchas + tooling) y pasala por el flujo de confirmación de "Crear
   skills/políticas": mostrásela al usuario, y al confirmar persistila activa
   con domain_project_skill_register pasando root_path=<subpath del stack>
   (scope interna, es de ESTE repo). La detección es silenciosa; la
   CONFIRMACIÓN previa a persistir no.
4. Si el stack vive en un subpath o submódulo, registrá ese path también como
   project_repository (domain_project_repo_add con root_path=<subpath>) para
   dejar la estructura del monorepo explícita en BD. domain_project_skill_list
   devuelve el root_path de cada skill → usá el cwd para elegir qué skill de
   stack aplica al subdir donde estás trabajando.

DRIFT DE STACK: cuando bootstrap devuelve head.changed=true, además de revisar
el git log, mirá si los manifiestos de stack (composer.json, go.mod, package.json,
etc.) cambiaron entre last_known_head y current. Si cambió la versión del
framework, el runtime, la DB o el test runner, la skill de stack quedó stale →
proponé su actualización vía el flujo de confirmación (domain_skill_edit).

El cliente IDE (Claude Code/OpenCode) solo reporta cwd+remote+branch+head;
la inteligencia de stack vive acá, no en el cliente. Si abrís el IDE
dentro de un submódulo con su propio remote, bootstrap matchea por ese
remote → puede ser otro proyecto domain. Es esperable.

## Tras compactación de contexto (re-hidratación)
Domain es PULL, no push: las skills, memorias y policies NO viven en el
contexto de la conversación — se consultan on-demand desde BD. Por eso
cuando tu contexto se compacta (perdés el detalle de la conversación
previa) el estado NO se pierde: está en domain. Re-hidratá así:

1. domain_session_bootstrap(cwd, git_remote, git_branch, git_head) →
   recuperás project, recent_observations, counts, head.changed y
   work_summary (open_tickets, open_issues, active_flow_run).
2. domain_mem_context(project_slug) → últimas observaciones/decisiones.
   domain_mem_search si necesitás algo puntual de antes.
3. Abrí la sesión con un mini-resumen para el usuario a partir de
   work_summary + recent_observations: "venís trabajando en X, hay N
   tickets / M issues abiertos" y, si work_summary.active_flow_run != null,
   "quedó una tarea SDD en estado <status>".
4. Si active_flow_run != null: domain_orchestrate_status para ver la fase y
   RETOMÁ (no reinicies). Si el usuario ordena suspenderla/archivarla, NO
   reinicies ni borres: cambiá el estado — flow_run a paused/cancelled
   (domain_orchestrate_*), issue con domain_issue_set_status(archived),
   ticket con domain_ticket_change_status(cancelled/blocked).
5. NO re-crees skills ni re-importes rules: si project_skill_count>0 y las
   policies ya están, ya está configurado. Solo leé, no dupliques.

Esto aplica igual al inicio de sesión y tras cada compactación: el
handshake es idempotente, repetirlo es barato y te re-sincroniza.

## Ejecución con subagentes del cliente (paralelización SDD)
El server de domain NO tiene LLM: solo planifica y persiste estado. TODO el
trabajo lo ejecutás VOS (el agente del cliente: Claude Code / OpenCode) con tu
propio LLM y tus subagentes nativos. El orchestrator describe el plan; vos lo
corrés.

Cuando una fase SDD emite trabajo paralelizable, usá tus subagentes nativos
(Task tool en Claude Code; subagents en OpenCode) para fan-out:

- sdd-explore con multi_concern=true → cada concern es un sub-flow SDD
  independiente. Lanzá un subagente por concern EN PARALELO (no comparten
  archivos por definición). Agregá los resultados al terminar.
- sdd-tasks: las tasks traen parallel_group. Ejecutá los grupos en orden
  ascendente; dentro de un grupo, lanzá las tasks en paralelo con un subagente
  cada una. La task "verify" (último grupo) va sola, después de todo.
- Cada subagente recibe un prompt auto-contenido (no asume contexto del otro).
  Vos sos el supervisor: repartís, esperás, agregás, y reportás con
  domain_orchestrate_phase_result.

Fallback: si tu cliente NO soporta subagentes, ejecutá secuencialmente en el
orden de position. El resultado es el mismo, solo más lento. Nunca paralelices
tasks de distinto parallel_group ni concerns marcados como dependientes.

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
de memoria conectado, y este protocolo aplica en CUALQUIER repositorio
que abras (es global).

PRECEDENCIA sobre reglas locales del repo: los archivos de reglas de IA
del proyecto (AGENTS.md, CLAUDE.md, .claude/, .cursorrules, .windsurf/,
.github/copilot-instructions.md, .config de agentes, etc.) quedan
SUBORDINADOS a domain en SU dominio: memoria persistente, skills, policies
SDD, y el protocolo de tools domain_*. Si una regla local contradice a
domain en esos temas, seguí domain. (Las reglas técnicas del repo —estilo
de código, stack, convenciones del lenguaje— siguen valiendo; domain NO las
invalida, las importa a BD con domain_project_policy_import_from_text para
que queden versionadas como project_policies.)

AL INICIO DE CADA SESIÓN **y tras cada compactación de contexto**:
1. domain_session_bootstrap(cwd, git_remote, git_branch, git_head) →
   resuelve/crea el proyecto y devuelve project_skill_count, counts,
   recent_observations y head.changed. Es tu primer llamado, siempre.
2. domain_policy_get(slug="agent-protocol") → cargá el protocolo COMPLETO
   y vivo (este archivo es solo el bootstrap; la fuente editable está en BD).
3. Si project_skill_count=0 → detectá el/los stack(s) del repo (manifiestos
   en root + subdirs + .gitmodules; un monorepo = N stacks) y, confirmando
   con el usuario, creá UNA skill project-scoped por stack (con su root_path).
   Una vez por proyecto. domain_project_skill_list devuelve root_path → usá
   el cwd para saber qué skill de stack aplica al subdir donde trabajás.

La compactación NO es sesión nueva pero igual perdés el contexto
conversacional: re-hidratá repitiendo el handshake (es idempotente y barato).

Mínimo si la policy no carga: domain_mem_save tras cada decisión/bug/
convención (project_slug = repo actual); domain_mem_search cuando pidan
recordar. Toda skill/policy nueva se confirma con el usuario ANTES de
persistir (AskUserQuestion) y se crea activa. Si un tool domain_* falla con
"Connection closed": pedile al usuario correr /domain-login — no uses otro
sistema de memoria como fallback.
`
