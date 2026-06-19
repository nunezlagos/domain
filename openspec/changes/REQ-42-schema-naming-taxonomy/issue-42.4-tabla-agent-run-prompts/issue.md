# issue-42.4-tabla-agent-run-prompts

**Origen:** `REQ-42-schema-naming-taxonomy`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** operador de la plataforma (rol `admin` u `owner`) que audita y depura agentes
**Quiero** que cada iteracion de un `agent_run` persista el PROMPT EFECTIVO que la plataforma le arma y le manda al LLM (system prompt resuelto + mensajes ensamblados + tools expuestas)
**Para** poder auditar/reproducir runs, medir el costo en tokens del system prompt vs el del usuario, y — cruzando con `captured_prompts` — enseñarle al usuario orquestador cómo su prompt crudo se transformó en el prompt final que vio el modelo

## Criterios de aceptación

```gherkin
Feature: Captura del prompt efectivo por iteracion del agente

  Background:
    Given existe un agent_run en estado running creado por el orquestador
    And el agente tiene un system_prompt resuelto y skills asignadas como tools

  Scenario: Se persiste una fila por iteracion del loop de completion
    Given el agente corre 3 iteraciones (1 con tool calls, 1 con tool calls, 1 final)
    When el orquestador llama a provider.Complete() en cada iteracion
    Then la tabla agent_run_prompts tiene 3 filas para ese agent_run_id
    And cada fila tiene iteration = 1, 2 y 3 respectivamente
    And cada fila guarda el system_prompt, los messages serializados y los tool_slugs vigentes en esa llamada

  Scenario: La fila refleja el prompt REAL enviado al modelo
    Given la iteracion 2 incluye en messages un assistant con tool_calls y un tool result
    When se persiste el snapshot de la iteracion 2
    Then el campo messages contiene el array completo (user + assistant + tool) tal cual fue a opts.Messages
    And char_count es la suma de longitudes de system_prompt + contenido de cada message
    And estimated_tokens es el resultado de tokens.EstimateMessages(system_prompt, messages)

  Scenario: La FK cae en cascada al borrar el run
    Given existen prompts persistidos para un agent_run
    When se elimina ese agent_run
    Then las filas de agent_run_prompts de ese run se borran automaticamente (ON DELETE CASCADE)

  Scenario: El INSERT del snapshot nunca aborta el run
    Given el pool de base de datos rechaza el INSERT del snapshot (error transitorio)
    When el orquestador intenta persistir el prompt de la iteracion
    Then el error se ignora silenciosamente (patron best-effort de appendLog)
    And el run continua con la llamada al LLM normalmente

  Scenario: Reintento de una misma iteracion sobrescribe el snapshot
    Given ya existe una fila con (agent_run_id, iteration) = (R, 2)
    When el orquestador reintenta esa misma iteracion y vuelve a persistir
    Then el UNIQUE (agent_run_id, iteration) dispara ON CONFLICT DO UPDATE
    And la fila se actualiza con los nuevos messages/char_count/estimated_tokens y updated_at = NOW()

  Scenario: Single-org sin organization_id ni RLS
    When se crea la tabla agent_run_prompts
    Then la tabla NO tiene columna organization_id
    And la tabla NO tiene Row Level Security habilitada
    And el trigger set_updated_at_agent_run_prompts mantiene updated_at en cada UPDATE
```

## Diferencia con `captured_prompts` (mig 000104)

| Eje | `captured_prompts` (000104) | `agent_run_prompts` (esta HU, 000150) |
|---|---|---|
| Qué guarda | El RAW TEXT que el USUARIO escribe en su IDE (claude-code/cursor/...), sin filtro | El prompt EFECTIVO que la PLATAFORMA ensambla y envía al MODELO (system + messages + tools) |
| Propósito | Analizar la CALIDAD DE REDACCIÓN del humano (claridad, longitud, ambigüedad, intent shifts) y devolver recomendaciones | Auditar/depurar qué vio realmente el LLM, reproducir runs, medir token cost system vs user |
| Atado a | `session_id` / `user_id` / `project_id` | `agent_run_id` + `iteration` |
| Conoce el agente/run | NO (es input crudo del humano) | SÍ (es la verdad de runtime del orquestador) |
| Búsqueda | `content_tsv` (GIN, full-text español) | índice por `(agent_run_id, iteration)` y por `created_at` |
| Cardinalidad | 1 prompt humano | N filas (N iteraciones de N agentes derivados) |

Un `captured_prompt` (1 humano) puede derivar en N `agent_run_prompts` (N iteraciones de N agentes). Cruzando ambas tablas se obtiene el DELTA entre lo que el usuario pidió y lo que el sistema realmente mandó al modelo.

## Taxonomía

- **Grupo:** `agent`
- **Prefijo:** `agent_`
- **Tabla nueva:** `agent_run_prompts` (acompaña a `agents`, `agent_versions`, `agent_templates`, `agent_runs`, `agent_run_logs`)
- **Convención de nombre:** snake_case plural, prefijo del grupo. Coherente con `agent_run_logs` (mismo padre `agent_runs`, misma FK ON DELETE CASCADE).

## Análisis breve

- **Qué pide realmente:** crear una tabla nueva `agent_run_prompts` que capture el prompt efectivo (system + messages + tools) por iteración de cada `agent_run`, y conectarla al orquestador para que escriba una fila justo antes de cada llamada al LLM.
- **Módulos a tocar:**
  - Migration `000150_create_agent_run_prompts.{up,down}.sql` (nueva tabla).
  - `internal/runner/agent/runner.go`: helper `appendPromptSnapshot` + llamada dentro del loop de `Run()` antes de `provider.Complete()`.
- **Riesgos / dependencias:** volumen de filas con `messages` JSONB grande; solapamiento conceptual con `agent_run_logs` (entró vs salió del LLM); el INSERT corre en el hot path (best-effort, no debe abortar). Detalle en `design.md`.
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Confirmar que `000146` es la última migration aplicada y que `000150` está libre (no existe ya en `migrations/`)
- [ ] Confirmar que la función `set_updated_at()` existe globalmente (mig 000001-000003)
- [ ] Confirmar que `agent_runs(id)` es la PK destino de la FK (patrón `agent_run_logs`)
- [ ] Confirmar que ya NO hay `organization_id` ni RLS en tablas runtime (mig 000142 dropeó la columna global, mig 000132 deshabilitó RLS)
- [ ] Confirmar en `runner.go` que `runID` (líneas 158-163), `iterations` (línea 200), `agent.SystemPrompt`, `agent.Model`, `messages` y `tools` están en scope dentro del loop antes de la línea 218
- [ ] Confirmar que `tokens.EstimateMessages` está importado (línea 128) y acepta `(systemPrompt string, messages []llm.Message)`

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
