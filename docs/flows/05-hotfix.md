# Flow: `hotfix` — bug urgente / producción caída

Idéntico a `fix` pero con dos diferencias importantes:
1. **Severity defaulteada a `critical`** por el classifier (confidence ≥ 0.85).
2. **Notificaciones agresivas** al owner del HU (HU-20 notifications).

## Ejemplo de prompt

> "URGENTE: producción caída, todos los logins fallan, esto es critical bug"

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
    participant Wizard as Wizard adaptive
    participant Notif as NotificationChannel
    participant BD as Postgres

    U->>Cli: "URGENTE: prod caída..."
    Cli->>MCP: domain_prompt
    MCP->>Router: Route
    Router->>Class: Classify
    Note over Class: Detecta keywords:<br/>"URGENTE", "prod down",<br/>"critical bug", "p0"
    Class-->>Router: intent=hotfix<br/>confidence=0.85<br/>severity=critical

    Router->>Intake: Submit
    Intake->>BD: INSERT intake_payloads<br/>classified_severity=critical
    Note over BD: trigger notif por severity=critical

    Intake->>Notif: enqueue email/slack al owner
    Notif-->>Notif: dispatch async

    Router->>Wizard: StartAdaptive(mode=bug-fix)
    Wizard->>Wizard: Analyzer pipeline
    Note over Wizard: severity ya inferida=critical<br/>NO se pregunta
    Wizard-->>Cli: Question slot=root_cause<br/>"prod caída, alta criticidad confirmada.<br/>¿Causa probable (logic / race / perf /<br/>security / ux)?"

    Note over U,Cli: Flow rápido — menos preguntas porque<br/>severity + has_repro suelen ser obvios
    loop pocas preguntas
        Cli-->>U: pregunta
        U->>Cli: respuesta
        Cli->>MCP: answer
        MCP->>Wizard: AnswerAdaptive
    end

    Wizard->>BD: status=finished
    Cli->>MCP: Commit + agent IA fixea
    Cli->>BD: INSERT user_stories<br/>priority=critical
    BD->>Notif: emit hu.created + severity=critical
    Notif->>U: "HU-XX creada, asignada a oncall"
```

## Diferencias clave vs `fix` normal

| Aspecto | fix | hotfix |
|---|---|---|
| `classified_severity` | high (inferida) | **critical** (alta conf) |
| Notificación al crear intake | no | **sí** (email + slack) |
| Skip de pregunta severity | a veces | **siempre** (ya inferida ≥0.85) |
| Tareas async (workers) | normales | con priority boost |
| SLA | 24h | **<2h** |

## Asserts BD

```sql
SELECT classified_severity, classified_confidence
FROM intake_payloads
WHERE id = <intake_id>;
-- Expected: ('critical', >= 0.8)

-- Verifica notification dispatched
SELECT channel, recipient, status FROM notification_deliveries
WHERE event_type = 'intake.hotfix';
-- Expected: row con status='sent' o 'queued'
```

Tests: `TestIssueType_Hotfix_HighConfidenceCritical`.
