# Domain

Plataforma de memoria persistente y orquestación para agentes AI.

**Stack:** Go 1.23+ · Postgres 16 + pgvector · pgx v5 · MCP (mark3labs/mcp-go) · golang-migrate

> Proyecto 100% generado con agentes IA dirigidos por humanos. Ver `.claude/rules/ai-generation.md`.

## Estado actual

**Fase 0 — Bootstrap dev environment** (completada parcialmente).

- 27 REQs / 149 HUs / 5 RFCs / 11 rules de conventions specificados en `openspec/`
- Roadmap detallado en `docs/roadmap.md` (6 fases)
- Índice de implementación en `openspec/INDEX.md`
- Catálogo de 10 personas en `docs/personas.md`
- 0 HUs implementadas (todas en estado `proposed`)

## Quick start (dev)

Pre-requisitos:
- Docker + Docker Compose v2
- Go 1.23+
- Make

```bash
# 1. Setup env
make env                    # copia .env.example → .env

# 2. Levantar stack dev (Postgres + MinIO + Adminer + Mailpit)
make dev-up

# 3. Verificar
make dev-ps                 # ver containers
curl http://127.0.0.1:9001  # MinIO console
curl http://127.0.0.1:8080  # Adminer
curl http://127.0.0.1:8025  # Mailpit
make dev-psql               # psql en Postgres

# 4. Build binarios (Fase 0 sólo trae `version`)
make build
./bin/domain version

# 5. Bajar stack (preserva data)
make dev-down

# Reset completo (borra data!)
make dev-reset
```

## Estructura del repo

```
.
├── cmd/
│   ├── domain/             # CLI + HTTP server entrypoint
│   └── domain-mcp/         # MCP server stdio entrypoint
├── internal/
│   ├── config/             # HU-01.2 config-system
│   ├── http/               # HU-13 HTTP API handlers + middleware
│   ├── store/pg/           # Postgres adapters (HU-01.1 + repos)
│   ├── mcp/                # HU-12 MCP server
│   ├── api/handler/        # HTTP handlers per feature
│   ├── llm/                # HU-06 LLM provider abstraction
│   └── seeds/              # HU-01.7 seeders framework + catalogs embebidos
├── migrations/             # golang-migrate SQL files (HU-01.1)
├── openspec/changes/       # Specs SDD (REQs / HUs)
├── openspec/INDEX.md       # Índice de implementación por fase
├── docs/
│   ├── roadmap.md          # Roadmap 6 fases
│   ├── personas.md         # Catálogo 10 personas
│   ├── rfc/                # RFCs de boundaries arquitectónicas
│   └── runbooks/           # Operacionales (futuro)
├── scripts/
│   └── postgres/init/      # SQL ejecutado al primer boot del container pg
├── deploy/                 # Helm chart + Kustomize (Fase 4)
├── .claude/
│   ├── rules/              # Conventions (db, api, security, etc.)
│   └── instructions.md     # Instrucciones agente IA
├── AGENTS.md               # Onboarding para agentes IA en este repo
├── CHANGELOG.md            # Keep a Changelog
├── docker-compose.yml      # HU-01.6 dev stack
├── Makefile                # Targets dev + build
├── go.mod
└── README.md               # Este archivo
```

## Workflow SDD para contribuir

Antes de tocar nada, leer:
1. `AGENTS.md` (orientación general)
2. `.claude/rules/ai-generation.md` (cómo trabaja el agente IA)
3. `.claude/rules/sdd.md` (flujo TDD obligatorio)
4. `.claude/rules/git.md` (Conventional Commits)
5. La HU específica que vas a implementar en `openspec/changes/REQ-XX/HU-XX.Y/`

## Documentación clave

- [`docs/roadmap.md`](docs/roadmap.md) — Roadmap 6 fases con HUs por fase
- [`openspec/INDEX.md`](openspec/INDEX.md) — Orden de implementación
- [`docs/personas.md`](docs/personas.md) — 10 actores del sistema
- [`docs/rfc/`](docs/rfc/) — Decisiones arquitectónicas (boundaries)
- [`.claude/rules/`](.claude/rules/) — Convenciones (db, api, security, testing, observability, migrations, git, ai-generation, sdd, clean-architecture, go)
- [`CHANGELOG.md`](CHANGELOG.md) — Changelog

## License

TBD
