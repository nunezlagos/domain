# Getting Started — Domain en 5 minutos

Setup mínimo para tener Domain MCP corriendo + conectado a Claude Code.

## Pre-requisitos

- Go 1.25+
- Postgres 16+ con pgvector
- Docker (opcional, para el devstack)
- Claude Code u OpenCode instalado

## 1) Levantar Postgres

```bash
docker compose up -d postgres
# o usar postgres existente; setear DOMAIN_DATABASE_URL más abajo
```

## 2) Build

```bash
go build -o ./bin/domain ./cmd/domain
go build -o ./bin/domain-mcp ./cmd/domain-mcp
```

## 3) Aplicar migrations

```bash
export DOMAIN_DATABASE_URL="postgres://app_admin:pass@localhost:5432/domain?sslmode=disable"
./bin/domain migrate up
```

Salida esperada: `applied 72 migrations` (incluye seed initial + grants
defensivos de la migration 000072).

## 4) Bootstrap dev — crea org + api_key + .env

```bash
./bin/domain dev-bootstrap
```

Esto:
- Crea org `dev` + user `admin@example.local`
- Emite api_key fresh
- Escribe `DOMAIN_API_KEY=...` en `.env`

Output:

```
✓ org id=... slug=dev
✓ user id=... email=admin@example.local
✓ api_key id=... prefix=dom_dev_xxxxxx

API KEY (guardalo, no se vuelve a mostrar):

  dom_dev_xxxxxx...

✓ .env actualizado: DOMAIN_API_KEY=...
```

## 5) Levantar el server

```bash
source .env
./bin/domain server
```

En otra terminal, verificá:

```bash
curl http://localhost:8000/health
# {"status":"ok",...}

curl -H "Authorization: Bearer $DOMAIN_API_KEY" \
     -d '{"raw_text":"hola, ¿cómo se configuran las migrations?"}' \
     -H "Content-Type: application/json" \
     http://localhost:8000/api/v1/prompt
# {"data":{"outcome":"chat","intent":"chat","reply":"..."}}
```

## 6) Conectar al agente IA (Claude Code)

```bash
./bin/domain setup claude-code \
  --api-key "$DOMAIN_API_KEY" \
  --base-url http://localhost:8000 \
  --auto-init
```

Esto:
- Agrega `domain-mcp` al config de Claude Desktop
- Detecta archivos `.md` de IA en tu repo current
  (`CLAUDE.md`, `.claude/**/*.md`, `.opencode/**/*.md`, `.cursorrules`, etc.)
- Hace **backup en BD** + reemplaza por stubs que apuntan al MCP

Reiniciá Claude Desktop y empezá a usar prompts normales — Domain MCP
los intercepta vía `domain_prompt`.

## 7) Flow completo en Claude Code

Después del setup, en Claude Code escribís:

> "El botón export de runs falla con 500, no funciona"

Domain MCP:

1. Clasifica intent → `fix` (confidence 0.75)
2. Crea `intake_payload` en BD
3. Arranca el wizard adaptive (`mode=bug-fix`)
4. **Analyzer pipeline** corre 4 fuentes en paralelo:
   - Memory search vs observations + knowledge
   - HU dedup vs issues existentes (sugiere `req_parent`)
   - Code grep en `internal/**/*.go` (sugiere `affected_component`)
   - Agent runs history reciente
5. Solo pregunta lo no inferible (típicamente 3-5 cosas vs 8 fijas)
6. LLM (Claude Haiku) formula cada pregunta con contexto inline:

> "Detecté issue-03.1 similar y 2 hits en handler.go. ¿Cuán crítico
>  es este bug? Opciones: critical, high, medium, low."

7. Tras responder todo → `Commit()` → escribe HU + Gherkin + Proposal +
   Design + Tasks en BD + filesystem
8. Agent IA implementa el fix con TDD strict

## Diagramas por tipo de issue

Ver [`docs/flows/README.md`](./flows/README.md) — 8 diagramas Mermaid:

- chat, idea (no entra al SDD)
- feature, fix, hotfix, refactor, doc, rfc (wizard adaptive)

## Troubleshooting

### `error: prompt_router_unavailable`

El binario `domain server` no está cableando el router. Verificá:

```bash
./bin/domain version
# Domain X.Y.Z (commit abc123)
```

Si la version es vieja, rebuild: `go build -o ./bin/domain ./cmd/domain`.

### Migration falla con "duplicate file 000038"

Update a commit más reciente: el bug se resolvió en `fix(migrations):
duplicate 000038 + missing flow_run_steps` (c2054f3 o posterior).

### El wizard pregunta muchas cosas (no parece adaptive)

Sin Anthropic key, el LLMClassifier no se enciende; el wizard usa
heurística + templates. Para activar LLM real:

```bash
export DOMAIN_ANTHROPIC_KEY=sk-ant-xxx
./bin/domain server
```

### Rollback del override de .md

```bash
./bin/domain workflow list                    # ver imported
./bin/domain workflow restore CLAUDE.md       # restaurar uno
# o via MCP: domain_workflow_restore(rel_path="CLAUDE.md")
```

## Next steps

- [docs/flows/README.md](./flows/README.md) — diagramas por flow
- `openspec/changes/` — 139 HUs implementadas con spec + design + tests
- `.claude/rules/` — conventions; ahora también en `platform_policies` BD
  (issue-01.8)
- `tests/e2e/` — 14 tests E2E del flow real
