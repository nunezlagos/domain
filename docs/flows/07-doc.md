# Flow: `doc` — actualización de documentación

Wizard mínimo: pregunta target (qué archivo/sección) + qué cambio.
Sin tests, sin Gherkin extensos. La spec es lightweight.

## Ejemplo de prompt

> "Hay que actualizar la documentación del README con los nuevos endpoints"

## Secuencia

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant Cli as Claude Code
    participant MCP as Domain MCP
    participant Router as PromptRouter
    participant Class as Classifier
    participant Memory as MemorySource
    participant Wizard as Wizard adaptive
    participant BD as Postgres

    U->>Cli: "actualizar README con endpoints..."
    Cli->>MCP: domain_prompt
    MCP->>Router: Route
    Router->>Class: Classify
    Class-->>Router: intent=doc conf=0.70

    Router->>Wizard: StartAdaptive(mode=doc)
    Wizard->>Memory: search "README endpoints"
    Memory-->>Wizard: matches con docs previos

    Wizard-->>Cli: Question slot=goal<br/>"¿Qué hay que documentar exactamente?"
    Cli-->>U: pregunta + sugerencias
    U->>Cli: "agregar tabla con endpoints /api/v1/*<br/>nuevos del último release"
    Cli->>MCP: answer

    Wizard-->>Cli: Question slot=summary
    U->>Cli: respuesta
    Cli->>MCP: answer

    Wizard-->>Cli: Question slot=slug
    U->>Cli: "docs-api-endpoints-update"
    Cli->>MCP: answer

    Wizard->>BD: status=finished

    Cli->>MCP: Commit
    Cli->>BD: INSERT user_stories priority=baja

    Note over Cli: Agente IA edita los .md targets<br/>directo (no TDD strict para docs)
```

## Slots típicos para mode=doc

| Slot | Inferible? | Fuente típica |
|---|---|---|
| intent | sí | classifier |
| req_parent | a veces | memory match |
| goal | NO | user (qué sección documentar) |
| summary | NO | user |
| slug | NO | user / derivado |

Más cortito que feature porque NO necesita:
- audience (es para "developers" implícito)
- severity (no es bug)
- gherkin scenarios (docs no tienen criteria testeables formalmente)

## Asserts BD

```sql
SELECT mode, priority FROM hu_drafts
JOIN user_stories ON user_stories.slug = jsonb_extract_path_text(answers, 'slug')
WHERE hu_drafts.id = <draft_id>;
-- Expected: mode='doc', priority='baja' o 'media'
```

Tests: `TestIssueType_Doc_StartsCorrectMode`.
