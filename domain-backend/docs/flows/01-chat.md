# Flow: `chat` — respuesta directa sin SDD

El usuario hace una pregunta conversacional ("¿cómo se configura X?").
Domain MCP responde directo, NO entra al wizard.

## Ejemplo de prompt

> "¿Cómo se configuran las migrations de postgres en este proyecto?"

## Secuencia

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant Cli as Claude Code
    participant MCP as Domain MCP
    participant Router as PromptRouter
    participant Class as Classifier
    participant BD as Postgres

    U->>Cli: tipea prompt
    Cli->>MCP: domain_prompt(raw_text)
    MCP->>Router: Route(rawText)
    Router->>Class: Classify(rawText)
    Class-->>Router: intent=chat<br/>confidence=0.65

    Note over Router: Intent es chat<br/>→ NO entra al SDD
    Router-->>MCP: Response{<br/> outcome=chat,<br/> reply="..." }
    MCP-->>Cli: JSON con reply

    Cli-->>U: muestra respuesta natural

    Note over BD: NO se crea intake_payload<br/>NO se crea hu_draft<br/>NO se crea user_story
```

## Asserts BD post-flow

```sql
-- Después de un prompt clasificado como chat:
SELECT COUNT(*) FROM intake_payloads;  -- 0
SELECT COUNT(*) FROM issue_drafts;        -- 0
SELECT COUNT(*) FROM issues;     -- 0 (no se materializó nada)
```

## Por qué importa

Sin esta rama, el wizard se dispararía por cada pregunta y agotaría
paciencia del usuario. El router actúa como **gate**: solo trabajo
estructurado (feat/fix/etc) atraviesa el SDD.

Tests: `TestIssueType_Chat_SkipsWizardAndReplies` en
`tests/e2e/issue_types_test.go`.
