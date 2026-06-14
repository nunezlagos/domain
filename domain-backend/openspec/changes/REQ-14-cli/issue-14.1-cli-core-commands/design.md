# Design: issue-14.1-cli-core-commands

## Decisión arquitectónica

**Cobra + Viper:** Estándar de facto en Go para CLI. Cobra maneja routing de comandos, flags, ayuda. Viper maneja configuración (file + env + flags). Ambos son de spf13, se integran nativamente.

**Command tree:**
```
domain
├── memory
│   ├── save      POST /api/v1/observations
│   ├── list      GET  /api/v1/observations
│   ├── get       GET  /api/v1/observations/{id}
│   ├── delete    DELETE /api/v1/observations/{id}
│   └── search    GET  /api/v1/observations?q=...
├── skill
│   ├── list      GET  /api/v1/skills
│   ├── get       GET  /api/v1/skills/{id}
│   ├── create    POST /api/v1/skills
│   ├── delete    DELETE /api/v1/skills/{id}
│   └── run       POST /api/v1/skills/{id}/run
├── agent
│   ├── list      GET  /api/v1/agents
│   ├── get       GET  /api/v1/agents/{id}
│   ├── create    POST /api/v1/agents
│   ├── delete    DELETE /api/v1/agents/{id}
│   └── run       POST /api/v1/agents/{id}/run
├── flow
│   ├── list      GET  /api/v1/flows
│   ├── get       GET  /api/v1/flows/{id}
│   ├── create    POST /api/v1/flows
│   ├── delete    DELETE /api/v1/flows/{id}
│   └── execute   POST /api/v1/flows/{id}/execute
├── cron
│   ├── list      GET  /api/v1/crons
│   ├── get       GET  /api/v1/crons/{id}
│   ├── create    POST /api/v1/crons
│   └── delete    DELETE /api/v1/crons/{id}
└── config
    ├── get       get config value
    ├── set       set config value
    └── view      show full config
```

**Client HTTP structure:**
```go
type Client struct {
    baseURL string
    apiKey  string
    client  *http.Client
}

// One method per entity+action, all using the generic Do()
func (c *Client) CreateObservation(ctx context.Context, obs *Observation) (*Observation, error)
func (c *Client) ListObservations(ctx context.Context, params ListParams) (*ListResponse[Observation], error)
func (c *Client) GetObservation(ctx context.Context, id string) (*Observation, error)
func (c *Client) DeleteObservation(ctx context.Context, id string) error
// ... same pattern for skill, agent, flow, cron
```

**Config resolution order:**
1. Defaults (hardcoded)
2. Config file (`~/.config/domain/config.yaml`)
3. Environment variables (`DOMAIN_API_ENDPOINT`, `DOMAIN_API_KEY`)
4. CLI flags (`--api-endpoint`, `--api-key`)

## Alternativas descartadas

1. **cli.go framework (urfave/cli):** Cobra tiene mejor integración con Viper, más ecosistema, y es más usado en proyectos Go grandes (Kubernetes, Docker, Hugo).
2. **Single command with subcommands via args:** `domain memory-list` vs `domain memory list`. La estructura `entity action` es más extensible y legible.
3. **Client SDK generado desde OpenAPI:** Sobredimensionado. Cliente HTTP manual es simple y nos da control total.

## Diagrama

```
┌──────────────────────────────────────────────┐
│  cmd/domain/main.go                         │
│  ├── rootCmd (persistent flags)              │
│  ├── memoryCmd → memory_save/list/get/delete │
│  ├── skillCmd  → skill_list/get/create/run   │
│  ├── agentCmd  → agent_list/get/create/run   │
│  ├── flowCmd   → flow_list/get/create/exec   │
│  ├── cronCmd   → domain_cron_list/get/create/delete │
│  └── configCmd → config_get/set/view         │
└──────────────┬───────────────────────────────┘
               │ calls
               ▼
┌──────────────────────────────────────────────┐
│  internal/client/client.go                   │
│  Do(method, path, body, resp)                │
│  CreateObservation(), ListObservations(),    │
│  GetObservation(), DeleteObservation()       │
│  ... same for skill, agent, flow, cron       │
└──────────────┬───────────────────────────────┘
               │ HTTP
               ▼
┌──────────────────────────────────────────────┐
│  API Server (REQ-13)                         │
│  /api/v1/{entity}                            │
└──────────────────────────────────────────────┘
```

## TDD plan

1. **Red:** Test `TestCLI_MemoryList` ejecuta `domain memory list` y espera output
2. **Green:** Implementar rootCmd + memoryCmd + list subcommand mínimo
3. **Refactor:** Extraer client HTTP, factory de commands
4. **Iterar:** Todas las entidades, client methods, config file
5. **Sabotaje:** Comando sin API key configurada → error message check

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Mucho boilerplate por entidad | Factory function `func entityCmd(name, path string) *cobra.Command` |
| Config file race condition | Viper maneja locking, usar sync.Once para init |
| API key expuesta en process list | Flags efímeros, leer de file o env var preferido |
| HTTP timeouts en commands largos | Timeout configurable por comando, default 30s |
