# issue-12.3-mcp-agent-tools

**Origen:** `REQ-12-mcp-server`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** agente o LLM con capacidad de invocar tools MCP
**Quiero** interactuar programáticamente con la plataforma Domain: ejecutar skills, gestionar agentes, iniciar flows, consultar cron jobs y buscar en knowledge base
**Para** automatizar flujos complejos sin intervención humana directa

## Criterios de aceptación

### Escenario 1: domain_skill_execute ejecuta un skill

```gherkin
Dado un skill con ID "sk_abc123" registrado en la plataforma
Cuando invoco `domain_skill_execute` con:
  | skill_id | "sk_abc123"             |
  | params   | {"topic":"arquitectura"} |
  | context  | "Proyecto Domain"      |
Entonces el skill se ejecuta en el runner
Y la respuesta contiene el resultado del skill
Y el resultado incluye el output y la duración
```

### Escenario 2: domain_skill_search busca skills disponibles

```gherkin
Dado que existen skills registrados en la plataforma
Cuando invoco `domain_skill_search` con:
  | query | "docker" |
  | limit | 5        |
Entonces devuelve hasta 5 skills que coinciden con la búsqueda
Y cada skill tiene: id, name, description, version
```

### Escenario 3: domain_agent_run ejecuta un agente

```gherkin
Dado un agente con ID "ag_abc123"
Cuando invoco `domain_agent_run` con:
  | agent_id    | "ag_abc123"               |
  | input       | "Analiza el código..."   |
  | session_id  | "ses_abc123"             |
Entonces el agente inicia una ejecución
Y la respuesta contiene el agent_run_id
Y el estado inicial es "running"
```

### Escenario 4: domain_agent_create crea un agente

```gherkin
Dado el servidor MCP con agent_tools registrados
Cuando invoco `domain_agent_create` con:
  | name        | "Code Reviewer"           |
  | description | "Revisa código en PRs"   |
  | model       | "gpt-4"                   |
  | system_prompt | "Eres un revisor..."   |
  | skills      | ["sk_review","sk_lint"]  |
Entonces se crea un nuevo agente en la plataforma
Y la respuesta contiene el ID del agente creado
```

### Escenario 5: domain_flow_run ejecuta un flow

```gherkin
Dado un flow con ID "fl_abc123"
Cuando invoco `domain_flow_run` con:
  | flow_id     | "fl_abc123"   |
  | params      | {"repo":"..."} |
  | webhook_url | "https://..." |
Entonces el flow inicia su ejecución
Y la respuesta contiene el flow_run_id
Y el estado inicial del run
```

### Escenario 6: domain_flow_create crea un flow

```gherkin
Dado el servidor MCP
Cuando invoco `domain_flow_create` con:
  | name     | "Deploy Pipeline"        |
  | steps    | [                       |
  |          |  {"type":"code_exec",   |
  |          |   "code":"deploy.sh"}   |
  |          | ]                       |
  | triggers | [{"type":"webhook",     |
  |          |   "event":"deploy"}]    |
Entonces se crea un nuevo flow
Y la respuesta contiene el ID del flow creado
```

### Escenario 7: domain_flow_status consulta estado

```gherkin
Dado un domain_flow_run con ID "fr_abc123" que está ejecutándose
Cuando invoco `domain_flow_status` con:
  | run_id | "fr_abc123" |
Entonces devuelve:
  | status       | "running"           |
  | current_step | 3                   |
  | total_steps  | 5                   |
  | started_at   | "2026-06-07T..."   |
  | duration     | 12.345             |
```

### Escenario 8: domain_cron_list lista cron jobs

```gherkin
Dado que existen cron jobs configurados
Cuando invoco `domain_cron_list` con:
  | project | "Domain" |
  | active  | true      |
Entonces devuelve los cron jobs activos del proyecto
Y cada cron tiene: id, name, schedule, last_run, next_run
```

### Escenario 9: domain_knowledge_search busca en knowledge base

```gherkin
Dado que existen documentos en la knowledge base
Cuando invoco `domain_knowledge_search` con:
  | query   | "PostgreSQL optimización" |
  | project | "Domain"                 |
  | limit   | 3                         |
Entonces devuelve hasta 3 documentos relevantes
Y cada documento tiene: id, title, snippet, score
```

## Análisis breve

- **Qué pide realmente:** 9 tools MCP para interacción programática con la plataforma: skills, agentes, flows, cron, knowledge base. Permiten que agentes externos controlen Domain vía MCP.
- **Módulos sospechados:** `internal/mcp/tools/agent/`, `internal/mcp/tools/skill/`, `internal/mcp/tools/flow/`, `internal/mcp/tools/cron/`, `internal/mcp/tools/knowledge/`
- **Riesgos / dependencias:** Depende de los servicios subyacentes (SkillService, AgentService, FlowService, CronService, KnowledgeService). Operaciones asincrónicas (domain_agent_run, domain_flow_run) requieren manejo de estados.
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
