# Flow: `idea` — exploración sin compromiso

El usuario propone una idea (`"y si agregamos…"`, `"se me ocurre…"`).
Domain responde reconociendo la idea + opcionalmente ofrece materializarla.

## Ejemplo de prompt

> "Se me ocurre una idea: y si agregamos un modo TUI offline para ver
> agent_runs en consola."

## Secuencia

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant Cli as Claude Code
    participant MCP as Domain MCP
    participant Router as PromptRouter
    participant Class as Classifier

    U->>Cli: tipea idea exploratoria
    Cli->>MCP: domain_prompt(raw_text)
    MCP->>Router: Route(rawText)
    Router->>Class: Classify(rawText)
    Class-->>Router: intent=idea<br/>confidence=0.65

    Router-->>MCP: Response{<br/> outcome=chat,<br/> intent=idea,<br/> reply="Anoté la idea..." }
    MCP-->>Cli: respuesta + sugerencia

    Cli-->>U: "Anoté la idea. Si querés convertirla en feature concreto,<br/>pasame el alcance y arrancamos el wizard SDD."

    Note over U: User puede:<br/>1) abandonar (idea queda en chat history)<br/>2) responder con scope → arranca feature
```

## Asserts BD

Igual que `chat`: ninguna fila nueva en intake/hu_drafts/user_stories.
La idea queda en el contexto conversacional del agente IA, NO en BD
estructurada.

## Diferencia con `feature`

Una idea sin compromiso de ejecución NO se materializa como HU. Para
convertir idea → feature, el usuario debe re-prompt con verbo de
implementación: "implementemos X", "necesito Y", "quiero Z".

Tests: `TestIssueType_Idea_SkipsWizardAndReplies`.
