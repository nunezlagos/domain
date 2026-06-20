# Flow: `refactor` — mejora interna sin cambio funcional

Wizard arranca con `mode=refactor`. El analyzer hace énfasis en
**code grep** para identificar todos los call sites afectados; el
planner pregunta el alcance del refactor + invariantes a preservar.

## Ejemplo de prompt

> "Necesito refactor del módulo de auth para extract los handlers en
> archivos separados"

## Secuencia

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant Cli as Claude Code
    participant MCP as Domain MCP
    participant Router as PromptRouter
    participant Class as Classifier
    participant Code as CodebaseSource
    participant Memory as MemorySource
    participant Analyzer as Analyzer
    participant Wizard as Wizard adaptive
    participant BD as Postgres

    U->>Cli: "refactor auth, extract handlers..."
    Cli->>MCP: domain_prompt
    MCP->>Router: Route
    Router->>Class: Classify
    Class-->>Router: intent=refactor conf=0.70

    Router->>Wizard: StartAdaptive(mode=refactor)
    Wizard->>Analyzer: Analyze(prompt)

    par
        Analyzer->>Code: grep "auth" "handler" en *.go<br/>busca funcs/types/endpoints
        Code-->>Analyzer: hits[internal/auth/apikey/middleware.go:42,<br/>internal/auth/otp/handler.go:88,<br/>internal/api/handler/api.go:113]
        Note over Code: Infiere component=<br/>"internal/auth"
    and
        Analyzer->>Memory: search "refactor auth"<br/>en observations + knowledge
        Memory-->>Analyzer: matches con tasks/diseños previos
    end

    Analyzer-->>Wizard: Envelope{<br/> code.hits=3 (auth module)<br/> memory.matches=2}

    Wizard->>BD: INSERT issue_drafts mode=refactor

    Wizard-->>Cli: Question slot=goal<br/>"Detecté 3 hits en internal/auth/* y 2<br/>diseños previos sobre auth. ¿Qué meta<br/>concreta del refactor? (ej: separar handlers,<br/>extract middlewares, consolidar tipos)"

    Cli-->>U: pregunta + análisis
    U->>Cli: "separar handlers de OTP+API key<br/>en archivos por feature"
    Cli->>MCP: domain_hu_create_answer
    MCP->>Wizard: AnswerAdaptive(slot=goal)

    loop pocas preguntas (refactor flow es corto)
        Wizard-->>Cli: pregunta {summary, slug, req_parent}
        U->>Cli: respuesta
        Cli->>MCP: answer
    end

    Wizard->>BD: status=finished
    Cli->>MCP: Commit
    Cli->>BD: INSERT issues slug='auth-handlers-extract'

    Note over Cli: Agente IA hace el refactor:<br/>1) crea tests baseline (snapshot)<br/>2) mueve código en pasos pequeños<br/>3) tests siguen verdes en cada paso<br/>4) NO cambios funcionales
    Cli->>BD: INSERT tasks (1 por paso del refactor)
```

## Slots típicos para mode=refactor

| Slot | Inferible? | Fuente típica |
|---|---|---|
| intent | sí | classifier |
| component | sí | code grep (paths afectados) |
| goal | NO | user (qué concreto quieren cambiar) |
| invariants_preserved | NO | user (qué NO debe cambiar) |
| slug | NO | user / derivado |
| summary | NO | user |

## Por qué refactor depende más de code grep

Una feature crea código nuevo; un refactor **mueve código existente**.
Sin saber dónde está el código actual, el agente IA no puede planear el
refactor. El CodebaseSource es esencial acá.

## Asserts BD

```sql
SELECT mode FROM issue_drafts WHERE id = <draft_id>;
-- Expected: 'refactor'

-- code.hits debe tener al menos 1 entry
SELECT jsonb_array_length(
  jsonb_extract_path(answers, '__envelope__', 'code', 'hits')
) FROM issue_drafts WHERE id = <draft_id>;
-- Expected: >= 1
```

Tests: `TestIssueType_Refactor_StartsCorrectMode`.
