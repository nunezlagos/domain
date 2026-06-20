# Flow: setup plug-and-play end-to-end

Desde repo vacío hasta primer prompt funcionando en <5 min. Incluye los
3 puntos de entrada (MCP stdio, HTTP `/api/v1/prompt`, CLI directo).

## Secuencia: instalación + primer prompt

```mermaid
sequenceDiagram
    autonumber
    actor U as Dev
    participant Sh as Shell
    participant DC as docker compose
    participant DB as Postgres+pgvector
    participant Dom as bin/domain
    participant Cli as Claude Code
    participant MCP as bin/domain-mcp (stdio)
    participant Router as PromptRouter
    participant Wizard as Wizard adaptive

    U->>Sh: docker compose up -d postgres
    Sh->>DC: start
    DC->>DB: boot (pgvector/pg16)
    DB-->>DC: ready

    U->>Sh: ./bin/domain migrate up
    Sh->>Dom: migrate up
    Dom->>DB: aplica 72 migrations + 000072_grants
    DB-->>Dom: schema_migrations = 72
    Dom-->>U: 72 tablas listas

    U->>Sh: ./bin/domain dev-bootstrap
    Sh->>Dom: dev-bootstrap
    Dom->>DB: INSERT org dev + user admin@example.local
    Dom->>DB: INSERT api_keys (bcrypt hash)
    Dom->>Sh: escribe .env<br/>DOMAIN_API_KEY=dev_xxx...
    Dom-->>U: api_key (1 vez)

    U->>Sh: source .env && ./bin/domain server &
    Sh->>Dom: HTTP server :8000 + workers

    U->>Sh: ./bin/domain setup claude-code --auto-init
    Sh->>Dom: setup wizard
    Dom->>Sh: escribe ~/.config/claude/mcp.json
    Note over Dom: --auto-init activa<br/>workflowimport sobre cwd
    Dom->>Sh: backup .md de IA en BD<br/>(CLAUDE.md, .claude/**, ...)
    Dom->>Sh: stubs .md → apuntan a MCP
    Dom-->>U: ✓ setup completo

    U->>Cli: abre repo + tipea prompt
    Cli->>MCP: spawn stdio (domain-mcp)
    MCP->>MCP: lee DOMAIN_API_KEY del env
    Cli->>MCP: tool domain_prompt(raw_text)
    MCP->>Router: Route(rawText, principal)
    Router->>Wizard: por intent

    alt intent == chat / idea
        Wizard-->>Router: reply directo
    else intent == feature / fix / hotfix / refactor / doc / rfc
        Wizard-->>Router: Question (slot, formulación LLM)
        Note over Wizard: loop hasta envelope completo<br/>→ commit HU + tasks
    end

    Router-->>MCP: Response
    MCP-->>Cli: JSON
    Cli-->>U: muestra respuesta
```

## Tres puntos de entrada equivalentes

```mermaid
flowchart LR
    subgraph clients[Clientes]
        CC[Claude Code]
        OC[OpenCode]
        CU[Cursor]
        SH[curl / script]
        CLI[bin/domain CLI]
    end

    subgraph entry[Entry Points]
        STDIO[MCP stdio<br/>domain_prompt tool]
        HTTP[POST /api/v1/prompt<br/>Bearer auth]
        DIRECT[bin/domain prompt<br/>local exec]
    end

    Router[PromptRouter]
    Classifier[LLMClassifier + Heuristic fallback]
    Analyzer[wizardplan.Analyzer<br/>4 fuentes paralelas]
    Planner[wizardplan.Planner + LLM Formulator]
    Wizard[issuebuilder.AdaptiveService]
    BD[(Postgres)]

    CC --> STDIO
    OC --> STDIO
    CU --> STDIO
    SH --> HTTP
    CLI --> DIRECT

    STDIO --> Router
    HTTP --> Router
    DIRECT --> Router

    Router --> Classifier
    Classifier -->|intent| Router
    Router -->|chat/idea| BD
    Router -->|feature+| Wizard
    Wizard --> Analyzer
    Analyzer --> Planner
    Planner --> Wizard
    Wizard --> BD
```

## Componentes wire-up en runtime

Wire-up real en `cmd/domain/main.go::runServer()` y
`cmd/domain-mcp/main.go::main()`:

```mermaid
flowchart TB
    ENV[config.Load: DOMAIN_DATABASE_URL + AUTH_URL + ANTHROPIC_KEY]
    ENV --> Pools[db.OpenPools<br/>App + Auth + ReadOnly]

    Pools --> Wp[wizardplan.Analyzer<br/>sources: HUDedup + Codebase + Memory]
    Pools --> In[intake.Service]
    Pools --> Wi[workflowimport.Service]

    Wp --> Hb[issuebuilder.AdaptiveService<br/>wraps Service v1]
    In --> Pr[promptrouter.Router]
    Hb --> Pr

    Pr --> Mcpd[mcpserver.Deps]
    Pr --> Apid[handler.API.Deps]

    Mcpd --> Tool[domain_prompt MCP tool]
    Apid --> Http[POST /api/v1/prompt]

    Wi --> InitCmd[domain init / setup --auto-init]
```

## Asserts BD post-setup

```sql
SELECT slug, name FROM organizations WHERE slug = 'dev';
-- (1 row)

SELECT email FROM users WHERE email = 'admin@example.local';
-- (1 row)

SELECT key_prefix, name FROM api_keys
WHERE name LIKE 'dev-bootstrap-%' AND revoked_at IS NULL;
-- (1 row, key_prefix visible)

SELECT version, dirty FROM schema_migrations;
-- 72, false

SELECT COUNT(*) FROM platform_policies WHERE active = true;
-- 10 (PlatformPoliciesSeeder)

SELECT COUNT(*) FROM model_registry;
-- 15 (ModelRegistrySeeder)
```

Tests: `tests/e2e/full_flow_test.go::TestE2E_PlugAndPlay_HappyPath`.
