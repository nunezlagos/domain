# Clean Architecture — Domain Project

Domain sigue Clean Architecture (Robert C. Martin) adaptada a Go, organizada por features (no por capas técnicas).

## Estructura de directorios

```
internal/
├── domain/          ← Entities + Value Objects + Repository interfaces
│   ├── memory/      ← Observation, Session, Prompt (entidades core)
│   ├── skill/       ← Skill, SkillVersion
│   ├── agent/       ← Agent, AgentRun
│   ├── flow/        ← Flow, FlowRun, Step
│   ├── project/     ← Project, ProjectTemplate, ProjectLink
│   └── auth/        ← User, ApiKey, Organization
│
├── service/         ← Use cases / application services
│   ├── memory/      ← SaveObservation, SearchObservations, etc.
│   ├── skill/       ← ExecuteSkill, RegisterSkill, etc.
│   ├── agent/       ← RunAgent, CreateAgent, etc.
│   └── ...
│
├── store/           ← Interface implementations (repositories)
│   └── pg/          ← Postgres implementations con pgx
│
├── mcp/             ← Interface adapters (MCP)
│   ├── server/      ← mark3labs/mcp-go setup
│   └── tools/       ← Tool handlers (domain_mem_save, etc.)
│
├── api/             ← Interface adapters (HTTP)
│   └── handler/     ← HTTP handlers (Gin o net/http)
│
├── config/          ← Configuración (env vars → struct)
└── llm/             ← LLM provider abstraction (OpenAI, Anthropic, etc.)

cmd/
├── domain/          ← Main + CLI commands (Cobra)
└── domain-mcp/      ← MCP server entrypoint

migrations/          ← SQL migraciones (golang-migrate)
```

## Reglas Clean Architecture

### Dependency Rule
Las dependencias SOLO apuntan hacia adentro (domain ← service ← store/mcp/api).

- `domain/` → NO depende de nada externo. Cero imports de frameworks, DB, librerías externas.
- `service/` → depende SOLO de `domain/` (usa interfaces, no implementaciones concretas)
- `store/pg/` → implementa interfaces de `domain/`, depende de pgx
- `mcp/`, `api/` → implementan interfaces de `service/` o `domain/`

### Por feature, no por capa técnica
NO agrupes por capa técnica (todos los models juntos, todos los handlers juntos).
Agrupá por **feature**: `memory/`, `skill/`, `agent/`, `flow/`, etc.

Cada feature tiene su propio:
- Entidad (`domain/memory/observation.go`)
- Repository interface (`domain/memory/repository.go`)
- Service (`service/memory/service.go`)
- Store impl (`store/pg/memory/store.go`)
- MCP tools (`mcp/tools/memory/`)

### Patrones de diseño

| Patrón | Uso |
|--------|-----|
| Repository | Acceso a datos. Interface en domain/, impl en store/pg/ |
| Factory | Creación de LLM providers, runners, skills |
| Strategy | Diferentes algoritmos según contexto (retry, cache, embedding) |
| Observer/Event | Activity log, webhooks, event bus |
| Chain of Responsibility | Middleware HTTP, validación de entrada |
| Adapter | MCP ↔ Service, HTTP ↔ Service |
| Unit of Work | Transacciones cross-entity (merge projects) |
| Builder | Construcción de queries complejas, flows DAG |

### Naming de packages
- Package name = feature name (singular): `memory`, `skill`, `agent`, `flow`
- Sin "utils", "common", "helpers" — si algo es compartido, tiene nombre semántico
- Interfaces en el package que las CONSUME, no en el que las implementa

### Tests por feature
```
internal/service/memory/
├── service.go
└── service_test.go        ← tests con store mockeado

internal/store/pg/memory/
├── store.go
└── store_test.go          ← tests de integración con Postgres real
```
