# HU-04.7-interactive-hu-builder

**Origen:** `REQ-04-opsx-sdd`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer o agente IA que va a especificar una nueva HU
**Quiero** un wizard interactivo MCP que haga preguntas dirigidas con opciones para clarificar el alcance antes de generar la HU
**Para** que ninguna HU quede ambigua, todas tengan audience/REQ/effort/path/goals declarados, y el agente IA pueda generar specs consistentes sin reinventar formato

## Diferencia con SDD "free-form" actual

Hoy una HU se escribe a mano o vía agente IA con prompts libres. Resultado: variabilidad en formato, ambigüedad, audience inferida.

Este wizard formaliza el "hard-spec" — antes de armar la HU, el agente DEBE preguntar:
1. ¿Qué tipo de cambio? (feature / bug-fix / refactor / doc / rfc)
2. ¿Audience destinataria? (de catálogo HU-01.9)
3. ¿REQ padre + esfuerzo + prioridad?
4. ¿Path/módulos sospechados?
5. ¿Goals + pains + success metrics?
6. ¿Hay HUs relacionadas?
7. (validación) — confirma resumen antes de crear

**Todo el state vive en BD** (tabla `hu_drafts`), consistente con el principio "todo en BD" del proyecto. El MCP/CLI/futura Web UI son clientes del orquestador.

## Modos soportados

| modo | uso | salida |
|------|-----|--------|
| `feature` | nueva capacidad user-facing | HU bajo REQ existente o nuevo REQ |
| `bug-fix` | reportar bug con repro + fix | HU tipo "fix" + audit del flujo de incidente |
| `refactor` | mejora interna sin cambio funcional | HU tipo "refactor" + RFC opcional |
| `doc` | mejora documentación | HU tipo "docs" o update directo si trivial |
| `rfc` | decisión arquitectónica trans-REQ | RFC en `docs/rfc/NNNN-name.md` |

## Criterios de aceptación

### Escenario 1: Iniciar wizard MCP

```gherkin
Dado que un agente IA conectado al MCP de Domain
Cuando invoca tool `domain_hu_create_start({mode: "feature", initial_idea: "Quiero que los users puedan exportar sus runs a CSV"})`
Entonces se crea draft row en `hu_drafts` con id UUID
Y se devuelve `{draft_id, next_question: {key, prompt, options[]}, progress: "1/8"}`
Y la primera pregunta es "¿Qué audience principal se beneficia?" con opciones del catálogo HU-01.9
```

### Escenario 2: Responder pregunta

```gherkin
Dado que existe draft activo
Cuando el agente invoca `domain_hu_create_answer({draft_id, answer: "dx-engineer"})`
Entonces se persiste la respuesta en hu_drafts.answers JSONB
Y se devuelve la próxima pregunta según el flujo del modo elegido
Y progress se actualiza ("2/8")
```

### Escenario 3: Preguntas adaptativas

```gherkin
Dado que el modo es "feature" y el usuario responde "audience=dx-engineer"
Cuando el wizard avanza
Entonces la próxima pregunta ofrece opciones de REQ contextual filtradas (skill/agent/memory/cli para dx-engineer)
Y NO ofrece REQs operacionales (backup-dr, observability) que no aplican
```

### Escenario 4: Validación final + preview

```gherkin
Dado que todas las preguntas respondidas
Cuando se invoca `domain_hu_create_finish(draft_id)`
Entonces se renderiza preview de hu.md, proposal.md, design.md, tasks.md, state.yaml
Y se devuelve `{preview: {...}, suggested_slug: "HU-XX.Y-name", target_path: "openspec/changes/REQ-XX-name/HU-XX.Y-name/"}`
Y el agente puede invocar `domain_hu_create_commit(draft_id)` para escribir los archivos
```

### Escenario 5: Confirmar dudas antes de generar

```gherkin
Dado que respuestas tienen ambigüedad detectada (ej. effort no especificado, audience conflicting con REQ)
Cuando wizard procesa
Entonces devuelve `pending_clarifications: [{key, why, options}]` antes de finalizar
Y el agente debe responder cada clarificación antes de poder commit
```

### Escenario 6: Audience target valida contra catálogo

```gherkin
Dado que existe tabla `audiences` (HU-01.9) seedeada con 10 audiences
Cuando wizard pide audience
Entonces options se generan dinámicamente desde `SELECT slug, name FROM audiences`
Y respuesta inválida (slug no existe) → error con sugerencias
```

### Escenario 7: REQ padre — existente o nuevo

```gherkin
Dado que wizard pregunta REQ padre
Cuando agente responde slug existente "REQ-03-memory-system"
Entonces se valida que existe en BD
Cuando responde "new" → flujo paralelo crea REQ + HU en mismo wizard
```

### Escenario 8: Generar Gherkin scenarios stub

```gherkin
Dado que llegó al paso de criterios de aceptación
Cuando wizard procesa
Entonces sugiere 3-5 escenarios Gherkin esqueleto basados en goals + audience
Y agente puede aceptar, editar o regenerar
```

### Escenario 9: Path inference

```gherkin
Dado que tipo="feature" y REQ padre="REQ-03-memory-system"
Cuando wizard sugiere target_path
Entonces propone "openspec/changes/REQ-03-memory-system/HU-03.{next}-{slug}"
Y next_number se calcula auto desde max(HU.N) existente
Y el agente puede override si tiene razón fuerte
```

### Escenario 10: Modo RFC

```gherkin
Dado que mode="rfc"
Cuando wizard procesa
Entonces preguntas distintas: status (draft/accepted), related REQs, alternativas consideradas, consecuencias
Y output va a `docs/rfc/NNNN-name.md` con numero auto
Y NO crea HU
```

### Escenario 11: Sabotaje — draft expirado

```gherkin
Dado que un draft tiene `created_at < now() - 24h` y no fue committed
Cuando se intenta finish
Entonces 410 "draft_expired"
Y cron purge limpia drafts >7 días
```

### Escenario 12: Multi-agent workflow

```gherkin
Dado que humano dice a Claude "quiero exportar runs"
Cuando Claude llama `domain_hu_create_start`
Entonces el wizard devuelve preguntas
Y Claude las contesta usando su contexto + le pregunta cosas al humano cuando hay ambiguity
Y al final propone preview, humano aprueba → commit a filesystem (vía Edit/Write del agente)
Y el state hu_drafts queda como audit trail
```

## Esquema BD

```sql
CREATE TABLE hu_drafts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID REFERENCES organizations(id),
  created_by UUID REFERENCES users(id),
  mode VARCHAR(20) NOT NULL,          -- feature|bug-fix|refactor|doc|rfc
  initial_idea TEXT NOT NULL,
  answers JSONB NOT NULL DEFAULT '{}',
  current_step INT NOT NULL DEFAULT 0,
  total_steps INT NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'in_progress',
  -- in_progress|finished|committed|expired|abandoned
  pending_clarifications JSONB DEFAULT '[]',
  preview JSONB,                       -- snapshot of hu.md/proposal/design/tasks ready to commit
  target_path TEXT,
  committed_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX ON hu_drafts (created_by, status);
CREATE INDEX ON hu_drafts (expires_at) WHERE status = 'in_progress';

CREATE TABLE hu_draft_steps_log (
  id BIGSERIAL PRIMARY KEY,
  draft_id UUID NOT NULL REFERENCES hu_drafts(id) ON DELETE CASCADE,
  step_key VARCHAR(50) NOT NULL,
  question TEXT NOT NULL,
  options JSONB,
  answer JSONB,
  answered_at TIMESTAMPTZ DEFAULT NOW()
);
```

## MCP tools

| tool | input | output |
|------|-------|--------|
| `domain_hu_create_start` | `{mode, initial_idea}` | `{draft_id, next_question, progress}` |
| `domain_hu_create_answer` | `{draft_id, answer}` | `{next_question \| pending_clarifications, progress}` |
| `domain_hu_create_preview` | `{draft_id}` | `{preview: {files}, target_path, suggested_slug}` |
| `domain_hu_create_commit` | `{draft_id, confirm: true}` | `{written_files[], audit_log_id}` |
| `domain_hu_create_abandon` | `{draft_id, reason}` | `{abandoned: true}` |
| `domain_hu_drafts_list` | `{status?}` | `{drafts[]}` |

## Step flow per mode

### feature (8 steps)
1. Audience (rol funcional)
2. REQ padre (existente filtered por audience, o "new")
3. Tipo (feature default, override OK)
4. Prioridad (alta/media/baja)
5. Esfuerzo (S/M/L/XL)
6. Goals + pains + success metrics (3 inputs textuales)
7. Path/módulos sospechados (sugerido vía REQ → carpetas)
8. Confirm + 3-5 Gherkin sugeridos editables

### bug-fix (6 steps)
1. Severidad (P0/P1/P2/P3)
2. Componente afectado
3. Reproducción steps
4. Comportamiento esperado vs actual
5. Impact + workaround temporal
6. Confirm

### refactor (5 steps)
1. Componente target
2. Motivo (perf/maintainability/security/etc.)
3. Surface change (none/api/db/config)
4. Plan migración (si applies)
5. Confirm

### doc (3 steps)
1. Doc target path
2. Tipo cambio (new/update/rewrite)
3. Confirm

### rfc (7 steps)
1. Tema
2. REQs relacionados
3. Decisión propuesta
4. Alternativas consideradas
5. Consecuencias positivas + negativas
6. Open questions
7. Status (draft/accepted/rejected)

## Análisis breve

- **Qué pide:** wizard MCP-driven con state en BD que clarifica antes de generar HU
- **Módulos sospechados:** internal/sdd/wizard, internal/mcp/tools/sdd, schema migration hu_drafts
- **Riesgos:** drafts huérfanos (cron purge), preguntas mal-secuenciadas frustran user, opcionalidad excesiva → ambigüedad
- **Esfuerzo tentativo:** L
