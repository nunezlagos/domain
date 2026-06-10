# Flow: `feature` — nueva capacidad

Implementar feature nueva. Wizard adaptive arranca con `mode=feature`.
El analyzer infiere lo que puede (audience, req_parent) y el planner
solo pregunta los slots faltantes.

## Ejemplo de prompt

> "Quiero implementar export de runs a CSV con streaming"

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
    participant Analyzer as Analyzer Pipeline
    participant Planner as Planner
    participant LLM as LLM Formulator
    participant Wizard as Wizard adaptive
    participant BD as Postgres

    U->>Cli: "quiero implementar export CSV..."
    Cli->>MCP: domain_prompt(raw_text)
    MCP->>Router: Route(rawText)
    Router->>Class: Classify(rawText)
    Class-->>Router: intent=feature conf=0.75

    Router->>Intake: Submit(raw_text)
    Intake->>BD: INSERT intake_payloads
    BD-->>Intake: intake_id

    Router->>Wizard: StartAdaptive(prompt, mode=feature)
    Wizard->>Analyzer: Analyze(prompt)

    par 4 fuentes en paralelo
        Analyzer->>BD: SELECT observations/knowledge FTS+embedding
        BD-->>Analyzer: matches
    and
        Analyzer->>BD: SELECT issues WHERE ts_match (HU dedup)
        BD-->>Analyzer: candidates
    and
        Analyzer->>Analyzer: grep internal/**/*.go por keywords
    and
        Analyzer->>BD: SELECT agent_runs reciente del user
        BD-->>Analyzer: related runs
    end

    Analyzer-->>Wizard: ContextEnvelope{<br/> intent, hu_matches,<br/> code_hits, memory,<br/> history, slots[]}

    Wizard->>BD: INSERT issue_drafts<br/>(answers=envelope JSON)
    Wizard->>Planner: NextQuestion(envelope)
    Planner->>Planner: pending = goal, summary, slug
    Planner->>LLM: FormulateQuestion(slot=goal, envelope)
    LLM-->>Planner: "Encontré issue-X.X similar y N hits en code/runs/..<br/>¿Qué se gana con este export?"
    Planner-->>Wizard: Question
    Wizard-->>MCP: Question{prompt, context_note, options}
    MCP-->>Cli: JSON
    Cli-->>U: pregunta con análisis inline

    loop hasta envelope completo
        U->>Cli: respuesta
        Cli->>MCP: domain_hu_create_answer(draft_id, slot, value)
        MCP->>Wizard: AnswerAdaptive(draft_id, slot, value)
        Wizard->>BD: UPDATE issue_drafts<br/>(answers[__envelope__].slots[slot]=provided)
        Wizard->>Planner: NextQuestion(envelope)
        alt pendientes
            Planner-->>Wizard: Question
            Wizard-->>Cli: Question
        else completo
            Planner-->>Wizard: NoMoreQuestionsErr
            Wizard->>BD: UPDATE issue_drafts status=finished
            Wizard-->>Cli: nil question + finished
        end
    end

    Cli->>MCP: domain_hu_create_preview(draft_id)
    MCP->>Wizard: BuildPreview()
    Wizard-->>Cli: Files{hu.md, proposal.md, design.md,<br/>tasks.md, state.yaml}

    Cli->>MCP: domain_hu_create_commit(draft_id)
    MCP->>Wizard: Commit()
    Wizard->>BD: UPDATE issue_drafts status=committed
    Wizard->>BD: audit_log hu_draft.committed

    Note over Cli: Agente IA escribe archivos<br/>en openspec/changes/REQ-XX/issue-XX.Y/<br/>+ crea user_story real
    Cli->>BD: INSERT issues (slug, title, req_id)
    Cli->>MCP: PromoteAttachmentsToHU(draft_id, issue_id)
    MCP->>Wizard: PromoteAttachmentsToHU()
    Wizard->>BD: UPDATE file_attachments<br/>SET entity_type='user_story', entity_id=issue_id
```

## Asserts BD post-flow

```sql
-- 1) Intake registrado con classification
SELECT classified_type, classified_confidence
FROM intake_payloads
WHERE source='agent'
ORDER BY created_at DESC LIMIT 1;
-- Expected: ('feature', >= 0.5)

-- 2) Draft committed
SELECT status, mode FROM issue_drafts WHERE id = <draft_id>;
-- Expected: ('committed', 'feature')

-- 3) Envelope persistido completo en answers JSONB
SELECT jsonb_extract_path(answers, '__envelope__', 'slots') FROM issue_drafts
WHERE id = <draft_id>;
-- Expected: todos los slots con status in ('provided','inferred')

-- 4) user_story creada
SELECT slug, status FROM issues WHERE slug = '<suggested_slug>';
-- Expected: row con status='proposed' o 'approved'
```

## Slots típicos para mode=feature

| Slot | Inferible? | Fuente típica |
|---|---|---|
| intent | sí | classifier |
| audience | a veces | memory dedup |
| req_parent | a veces | hu_dedup FTS |
| goal | NO | user |
| summary | NO | user |
| slug | NO | user (o derivar post-summary) |

En promedio: **2-4 preguntas** vs 8 fijas del v1.

Tests: `TestIssueType_Feature_StartsAdaptiveWizard` +
`TestIssueType_Feature_WithHUDedup_InfersReqParent`.
