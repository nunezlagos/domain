# Flow: `rfc` — decisión arquitectónica con tradeoffs

Wizard con `mode=rfc` pregunta por: problema a resolver, alternativas
consideradas, decisión + tradeoffs. Output va a `docs/rfc/NNNN-name.md`
(NO crea HU).

## Ejemplo de prompt

> "RFC: diseño arquitectura del nuevo sistema de cache multi-tier,
> tradeoffs entre redis vs pgvector"

## Secuencia

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant Cli as Claude Code
    participant MCP as Domain MCP
    participant Router as PromptRouter
    participant Class as Classifier
    participant History as AgentHistorySource
    participant Memory as MemorySource
    participant Wizard as Wizard adaptive
    participant BD as Postgres

    U->>Cli: "RFC cache multi-tier..."
    Cli->>MCP: domain_prompt
    MCP->>Router: Route
    Router->>Class: Classify
    Class-->>Router: intent=rfc conf=0.70

    Router->>Wizard: StartAdaptive(mode=rfc)

    par
        Wizard->>History: agent_runs con keyword "cache"
        History-->>Wizard: 2 runs previos (intentos anteriores)
    and
        Wizard->>Memory: search "cache" en knowledge_docs<br/>+ RFC dirs
        Memory-->>Wizard: matches con docs/rfc/0002-* (RFC vieja)
    end

    Wizard-->>Cli: Question slot=goal<br/>"Veo 2 attempts previos sobre cache y RFC 0002<br/>relacionada. ¿Cuál es el problema concreto<br/>que esta RFC resuelve?"

    Cli-->>U: pregunta + referencias
    U->>Cli: "latencia de LLM calls repetidos al<br/>mismo prompt + costo USD"

    Wizard-->>Cli: Question slot=alternatives<br/>"¿Qué alternativas consideraste?"
    U->>Cli: "1) redis ttl 1h<br/>2) pgvector similarity<br/>3) embed key-value en sqlite"

    Wizard-->>Cli: Question slot=decision<br/>"¿Cuál elegirías + por qué?"
    U->>Cli: "pgvector por cosine sim<br/>+ no agregar otra dep"

    Wizard->>BD: status=finished

    Cli->>MCP: Commit
    Note over Cli: Agente IA escribe<br/>docs/rfc/NNNN-llm-semantic-cache.md<br/>NO crea HU (RFC es decisión, no implementación)

    Note over BD: NO se crea user_story.<br/>El RFC vive en filesystem +<br/>se referencia en HUs futuras
```

## Slots típicos para mode=rfc

| Slot | Inferible? | Fuente típica |
|---|---|---|
| intent | sí | classifier |
| goal | NO | user (problema a resolver) |
| alternatives | NO | user (al menos 2-3) |
| decision | NO | user (qué eligió) |
| tradeoffs | NO | user |
| summary | NO | user |
| slug | NO | user / derivado |

## Output del flow RFC

Diferencias clave vs feature/fix:

1. **NO crea user_story**. Sólo doc.
2. **NO entra a tasks.md**. Las tasks vendrán cuando una HU futura
   implemente la decisión del RFC.
3. **Numero NNNN auto-incrementado** desde el max(docs/rfc/[0-9]+).
4. **Status workflow**: `draft → accepted | rejected | superseded`.

## Asserts BD

```sql
SELECT mode FROM issue_drafts WHERE id = <draft_id>;
-- Expected: 'rfc'

-- NO debe haber user_story creado
SELECT COUNT(*) FROM issues
WHERE slug LIKE jsonb_extract_path_text(answers, 'slug') || '%';
-- Expected: 0 (las RFC no materializan en issues)
```

Tests: `TestIssueType_RFC_StartsCorrectMode`.
