# AI-Generated Project — Domain

**Este proyecto se construye 100% con agentes IA.** Cada línea de spec, código, test, doc y migration es generada por un LLM (Claude, etc.) dirigido por humanos.

Esto NO es código generado de forma autónoma sin supervisión: el humano dirige el qué, el por qué, y revisa el resultado. La IA genera el cómo siguiendo specs y rules estrictos.

## Implicaciones para cómo se debe escribir TODO en este repo

### 1. Specs (HUs, RFCs, rules) son la fuente de verdad

- Las HUs en `openspec/changes/REQ-*/issue-*/` son **el contrato** que el agente debe cumplir
- Gherkin scenarios son **tests ejecutables conceptualmente** — el código generado debe satisfacerlos todos
- Si una HU es ambigua, **NO se implementa** — primero se clarifica el spec (regla en `.claude/rules/sdd.md`)
- Cambios de spec REQUIEREN revisión humana; el agente NO debe modificar specs salvo orden explícita

### 2. Rules son contratos máquina-leíbles

- `.claude/rules/*.md` son leídas por el agente cada vez que toca código
- Las rules tienen **enforcement automatizado** vía linters (issue-25.13, issue-13.9, issue-17.* PII, issue-26.1 stateless)
- Si un linter falla en CI, el código NO mergea. El agente debe respetar las rules o el lint detiene el merge
- Cuando una rule se actualiza: la versión en BD (`platform_policies` tabla, issue-01.8) es la activa; el agente la consulta via MCP tool `domain_policy_get`

### 3. Tests como segunda fuente de verdad

- Cada HU declara escenarios Gherkin → tests Go que los implementan
- TDD obligatorio (`.claude/rules/sdd.md`): test → impl mínima → refactor → sabotaje
- "Sabotaje test" (cada HU declara uno): rompe invariante intencional para confirmar que el test atrapa. Garantiza que el test no es "always green"
- Coverage targets: 70% global, 80% service+domain

### 4. Documentación como output, no input

- Docs se generan desde código o specs cuando posible:
  - OpenAPI spec generada desde tipos Go → SDKs auto-generados
  - Helm README via `helm-docs`
  - CHANGELOG via `goreleaser` o `git-cliff` desde Conventional Commits
  - Slow query reports generados por cron (issue-25.2)
  - Index suggestions generadas por pg_qualstats (issue-25.12)
- Documentos hand-written: solo HUs/RFCs/rules + runbooks operativos

### 5. Commits firmados sin AI attribution

Por regla del CLAUDE.md global del usuario:
- NUNCA `Co-Authored-By: Claude` ni similar
- Commits en español según Conventional Commits (`.claude/rules/git.md`)
- El humano que dirigió la generación es el autor; la IA es herramienta no co-autor

### 6. Reproducibilidad obligatoria

Que un agente IA distinto pueda re-generar (o continuar) el proyecto requiere:
- Specs autocontenidas (cada HU debe ser entendible sola con sus refs)
- Rules versionadas (issue-01.8 mantiene history)
- Seeders idempotentes (issue-01.7) garantizan misma BD inicial
- Reproducibility snapshots para flows (issue-09.11) para debugging
- Migration ordering inmutable; nada de "renumerar"
- CI bloquea drift entre spec, código y tests

### 7. Workflows pensados para multi-agent

Domain mismo soporta agentes que colaboran (REQ-08 + RFC 0002 multi-agent patterns). El proyecto se construye así:

- Un **planning agent** lee HU y diseña approach
- Un **coder agent** implementa siguiendo `.claude/rules/*`
- Un **reviewer agent** valida que cumple Gherkin + rules
- Un **tester agent** genera tests adicionales y casos edge
- **Humano** aprueba merge

Esta orquestación se reflecta en el patrón:
- HUs pequeñas (S/M effort) en lugar de monolitos
- Tasks atómicas en `tasks.md` (cada bullet es una unit de trabajo)
- Sabotaje tests aseguran que reviewer detecte regressions

### 8. Cuando hay ambigüedad: preguntar al humano

- El agente NUNCA inventa specs nuevos
- Si una HU dice "el sistema debe ser eficiente" sin métrica → preguntar al humano qué número
- Si dos rules entran en conflicto → preguntar al humano cuál prevalece
- Si una task requiere decisión arquitectónica no documentada → proponer 2-3 opciones con tradeoffs

### 9. Naming conventions optimizadas para IA

Naming explícito > naming breve. El agente NO infiere por contexto, lee literalmente.

- `domain_mem_save_observation` mejor que `save`
- `customer_billing_subscription_id` mejor que `cb_sub_id`
- Comments cuando el "por qué" no es obvio (regla CLAUDE.md global)
- Sin abreviaturas no-estándar (`agent_run`, no `agt_rn`)

### 10. Test fixtures con datos no-PII

Fixtures usan fakers deterministic (issue-25.11 anonymization). Nunca datos reales aunque sean "de prueba".

## Workflow estándar para agregar feature

```
1. Humano: define qué quiere a alto nivel
2. Agente: propone HU bajo REQ existente (o nuevo REQ si fuera del alcance)
   - hu.md con Gherkin
   - proposal.md con scope
   - design.md con decisión + alternativas
   - tasks.md con bullets atómicos
   - state.yaml proposed
3. Humano: revisa spec, ajusta, aprueba
4. Agente coder: implementa siguiendo tasks.md
   - TDD: test → impl
   - Respeta rules linters
   - Sabotaje test incluido
5. Agente reviewer: valida cumple Gherkin + rules
6. CI: lint + test + integration + linters custom verde
7. Humano: aprueba merge
8. Tag automático eventual; CHANGELOG via Conventional Commits
```

## Lo que el agente NO debe hacer

- **NO** modificar HUs/specs sin orden explícita del humano
- **NO** crear archivos `.md` documentación fuera del workflow SDD (la regla CLAUDE.md global lo prohíbe)
- **NO** ejecutar `git push`, `helm install`, `kubectl apply`, `migrate up` en prod sin confirmación
- **NO** committear `.env`, secrets, o cualquier cosa en `.gitignore`
- **NO** skipear linters con `--no-verify`
- **NO** asumir que algo funciona — verificar antes (regla CLAUDE.md "VERIFICA antes de afirmar")
- **NO** agregar `Co-Authored-By` ni atribución de IA a commits

## Lo que el agente DEBE hacer

- **SÍ** leer rules antes de tocar código relacionado
- **SÍ** consultar `domain_policy_get(slug)` cuando trabaja sobre un dominio (post-issue-01.8 implementada)
- **SÍ** ejecutar tests local antes de proponer commit
- **SÍ** preguntar cuando hay ambigüedad
- **SÍ** justificar decisiones no-obvias en commit body o PR description
- **SÍ** usar Gherkin scenarios como criterio de "done"
- **SÍ** respetar el roadmap (`docs/roadmap.md`) — no implementar HUs de fases futuras sin permiso

## Velocidad esperada

Este modelo permite que un humano + agente IA produzca aproximadamente:
- **3-5x velocidad** vs humano solo en tareas estandarizadas (CRUD, migraciones, tests)
- **1.5-2x velocidad** vs humano en tareas creativas (RFCs, arquitectura)
- **0.5-0.8x** en tareas que requieren contexto sutil tribal (legacy code, custom DSLs)

Para Domain (greenfield, specs detallados), velocidad esperada **3x equivalente** humano solo.

Roadmap 9-12 meses con 4-6 devs senior → con IA assistido bien dirigido: **6-8 meses con 2-3 devs senior + agents**.
