# Flow: `fix` — bug en producción / función rota

Wizard arranca con `mode=bug-fix`. Pregunta severity, root_cause,
expected/actual, etc. Cuando el analyzer detecta hits en código, sugiere
`affected_component` automáticamente.

## Ejemplo de prompt

> "El endpoint POST /api/v1/observations falla con error 500 al hacer
> click — no funciona el botón export"

## Secuencia

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant Cli as Claude Code
    participant MCP as Domain MCP
    participant Router as PromptRouter
    participant Class as Classifier
    participant Intake as IntakeService
    participant Worker as IntakeWorker
    participant Analyzer as Analyzer
    participant Code as CodebaseSource
    participant Dedup as HUDedupSource
    participant Planner as Planner
    participant Wizard as Wizard adaptive
    participant BD as Postgres

    U->>Cli: prompt de bug con keywords:<br/>"falla", "error 500", "no funciona"
    Cli->>MCP: domain_prompt(raw_text)
    MCP->>Router: Route
    Router->>Class: Classify
    Class-->>Router: intent=fix conf=0.75 severity=high
    Router->>Intake: Submit
    Intake->>BD: INSERT intake_payloads<br/>status=received
    BD-->>Intake: intake_id

    Note over Worker: Async — corre del lado server,<br/>no bloquea el flow del user
    Worker->>BD: SELECT WHERE status=received
    Worker->>BD: UPDATE classification

    Router->>Wizard: StartAdaptive(mode=bug-fix)
    Wizard->>Analyzer: Analyze(prompt)

    par
        Analyzer->>Code: grep "observations" "POST" en *.go
        Code-->>Analyzer: hits[handler.go:88, service.go:421]
        Note over Analyzer: infiere component=<br/>"internal/api/handler"
    and
        Analyzer->>Dedup: SELECT issues matching
        Dedup-->>Analyzer: issue-03.1 observations-crud (sim 0.4)
        Note over Analyzer: infiere req_parent=<br/>"REQ-03-memory-system"
    end

    Analyzer-->>Wizard: ContextEnvelope{<br/> intent=fix<br/> component=inferred<br/> req_parent=inferred<br/> hu_matches[]<br/> code_hits[]}

    Wizard->>Planner: NextQuestion
    Note over Planner: Pendientes: severity, root_cause,<br/>has_repro, expected, actual,<br/>slug, summary
    Planner-->>Wizard: Question slot=severity<br/>"Encontré issue-03.1 + 2 hits en handler.go.<br/>¿Cuán crítico es?"

    loop slot por slot
        Wizard-->>Cli: Question
        Cli-->>U: pregunta con análisis
        U->>Cli: respuesta
        Cli->>MCP: domain_hu_create_answer
        MCP->>Wizard: AnswerAdaptive
        Wizard->>BD: UPDATE issue_drafts.answers
        Wizard->>Planner: NextQuestion
    end

    Planner-->>Wizard: NoMoreQuestionsErr
    Wizard->>BD: status=finished

    Cli->>MCP: BuildPreview + Commit
    Wizard->>BD: status=committed

    Note over Cli: Agente IA escribe HU formal +<br/>implementa el fix con TDD +<br/>crea test sabotaje
    Cli->>BD: INSERT issues<br/>+ proposal + design + tasks
```

## Slots típicos para mode=bug-fix

| Slot | Inferible? | Fuente típica |
|---|---|---|
| intent | sí | classifier |
| severity | a veces | classifier (hotfix→critical) |
| component | a veces | code grep |
| root_cause | NO | user |
| has_repro | NO | user |
| expected | NO | user |
| actual | a veces | extract del prompt original |
| slug | NO | user / derivado |
| summary | NO | user |

## Asserts BD

```sql
-- Verifica classification
SELECT classified_type, classified_severity, classified_confidence
FROM intake_payloads
WHERE id = <intake_id>;
-- Expected: ('fix', 'high', >= 0.5)

-- Verifica draft con envelope serializado
SELECT mode, status,
       jsonb_extract_path(answers, '__envelope__', 'code', 'hits')
FROM issue_drafts WHERE id = <draft_id>;
-- Expected mode=bug-fix; code.hits con al menos 1 entry
```

Tests: `TestIssueType_Fix_PersistsClassificationAndDraft` +
`TestIssueType_FullHappyPath_FixWithCommit`.
