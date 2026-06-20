# Flow: SDD pipeline orchestrator (issue-08.10)

El orquestador SDD canaliza prompts `feature` / `fix` / `refactor` /
`hotfix` / `doc` / `rfc` desde `domain_prompt` a un pipeline gobernado de
10 fases (Full) o 2 fases (Express), ejecutadas por el cliente IDE con
estado persistido en `flow_runs` + `flow_run_steps`.

Ver guía completa en [`docs/agents/sdd-pipeline.md`](../agents/sdd-pipeline.md).

## DAG canónico

```mermaid
flowchart TD
    A[sdd-explore] --> B[sdd-spec]
    B --> C[sdd-propose]
    C --> D[sdd-design<br/>D5: adr required]
    D --> E[sdd-tasks]
    E --> F[sdd-apply<br/>D5: code_reference required]
    F --> G[sdd-verify]
    G --> H[sdd-judge<br/>D5: sabotage_record required]
    H --> I[sdd-archive]
    I --> J[sdd-onboard]

    style D fill:#fff4e6
    style F fill:#fff4e6
    style H fill:#fff4e6
```

**Express** ejecuta sólo `sdd-apply` → `sdd-verify` (fast path D1).

## Secuencia Full mode end-to-end

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant Cli as Claude Code
    participant MCP as Domain MCP
    participant Router as PromptRouter
    participant Orch as Orchestrator
    participant Repo as Repository
    participant BD as Postgres

    U->>Cli: "implementar export CSV con streaming"
    Cli->>MCP: domain_prompt(raw_text)
    MCP->>Router: Route(raw_text, userID, orgID)
    Router->>Router: Classify → intent=feature

    Router->>Orch: Run(input, Mode=Full)
    Orch->>Repo: GetFlowIDBySlug("sdd-pipeline-v1")
    Repo->>BD: SELECT flows WHERE slug AND org_id
    BD-->>Repo: flow_id
    Repo-->>Orch: flow_id

    Note over Orch: BuildFullPlan: 10 steps,<br/>sólo step[0] con UserPrompt hidratado

    Orch->>Repo: hydrateSystemPrompts → 10x lookup
    Repo->>BD: SELECT system_prompt FROM agent_templates<br/>WHERE slug=$1 AND org_id=$2
    BD-->>Repo: system_prompt
    Repo-->>Orch: hidratado

    Orch->>Repo: persistPlan
    Repo->>BD: INSERT flow_runs (status=pending,<br/>cursor={orchestrator_run_id, mode, raw_text})
    Repo->>BD: INSERT 10 flow_run_steps (status=pending)

    Orch-->>Router: OrchestrateResult{flow_run_id, plan, snapshot_prompt}
    Router-->>MCP: Response{outcome=orchestrator_started}
    MCP-->>Cli: JSON con plan + snapshot_prompt del step[0]

    loop por cada step
        Cli->>Cli: Ejecuta fase con system+user prompt
        Cli->>MCP: domain_orchestrate_phase_result(step_id, output, memory_refs)
        MCP->>Orch: RecordPhaseResult(input)

        Orch->>Repo: GetFlowRunStep(step_id)
        Orch->>Orch: ValidateRequiredSaves D5
        alt D5 ok
            Orch->>Orch: handler.Validate(output)
            alt validación ok
                Orch->>Repo: MarkStepCompleted(outputs)
                Orch->>Repo: ListFlowRunSteps → calcula aggregate status

                alt aún hay pending
                    Orch->>Orch: rebuildNextStepPrompt(handler.Build con PriorOutputs)
                    Orch->>Repo: UpdateStepInputs(user_prompt nuevo)
                else todos completed
                    Orch->>Repo: UpdateFlowRunStatus("completed")
                end

                Orch-->>Cli: PhaseResultResult{next_step_prompt}
            else handler reject
                Orch->>Repo: MarkStepFailed → propagateFlowStatusAfterFailure
                Orch-->>Cli: error tipado
            end
        else D5 falla
            Orch->>Repo: MarkStepFailed("required_save_missing")
            Orch->>Repo: UpdateFlowRunStatus("failed")
            Orch-->>Cli: *RequiredSaveError
        end
    end
```

## Path Express con D1 confirm condicional

```mermaid
sequenceDiagram
    autonumber
    participant Cli as Cliente IDE
    participant Orch as Orchestrator
    participant Repo as Repository

    Note over Cli,Repo: BuildExpressPlan → 2 steps pre-armados

    Cli->>Orch: phase_result(apply_step, output={files: [...], lines: 25})
    Orch->>Orch: shouldRequireConfirm(step, output)
    Note right of Orch: mode=express + lines>10 → true

    Orch->>Repo: MarkStepCompleted(apply)
    Orch->>Repo: MarkStepBlocked(verify, "D1 confirm required")
    Orch-->>Cli: PhaseResultResult{requires_confirm: true,<br/>next_step_id, next_step_prompt}

    alt user acepta
        Cli->>Orch: domain_orchestrate_confirm(flow_run_id, true)
        Orch->>Repo: MarkStepPending(verify)
        Orch-->>Cli: NextStepPrompt para verify
    else user rechaza
        Cli->>Orch: domain_orchestrate_confirm(flow_run_id, false)
        Orch->>Repo: MarkStepFailed(verify, "user_rejected_confirm")
        Orch->>Repo: UpdateFlowRunStatus("failed")
        Orch-->>Cli: flow_run terminado
    end
```

## Reanudación cross-session

```mermaid
flowchart LR
    A[Sesión cortada<br/>flow_run_id persistido] --> B[CLI: domain workflow resume]
    B --> C[GetFlowStatus]
    C --> D{status?}
    D -->|completed| E[✓ nada que hacer]
    D -->|failed| F[✗ ver error de step]
    D -->|running| G[Imprime prompt del próximo<br/>step pending]
    G --> H[Usuario copia prompt<br/>al cliente IDE]
    H --> I[Cliente reporta phase_result<br/>→ flujo continúa]
```
