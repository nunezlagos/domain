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

Ver [`docs/flows/README.md`](./flows/README.md) — 9 diagramas Mermaid:

- chat, idea (no entra al SDD)
- feature, fix, hotfix, refactor, doc, rfc (wizard adaptive o orquestador)
- orquestador SDD plug-and-play (issue-08.10) — ver [`docs/flows/09-orchestrator.md`](./flows/09-orchestrator.md)

## 8) Primer prompt con el orquestador SDD (issue-08.10)

Cuando el operador tiene `Router.Orchestrator` configurado (default en
`cmd/domain-mcp` desde el commit `44567e2`), los intents `feature` /
`refactor` / `doc` / `rfc` / `fix` / `hotfix` arrancan el **pipeline SDD
plug-and-play** en lugar del wizard legacy. El cliente IDE recibe
prompts construidos por el servidor y ejecuta cada fase en orden.

### Bootstrap por org (una sola vez)

Antes de invocar el orquestador, la org debe tener los catálogos
seedeados:

```bash
# Desde cmd/domain server (auto al boot via dev-bootstrap) o manual:
./bin/domain dev-bootstrap   # incluye SeedAgentTemplatesForOrg + SeedFlowsForOrg
```

Sin esto, el orquestador devuelve `ErrFlowNotSeeded` o `ErrAgentTemplateNotFound`.

### Ejemplo Express (fast path, fix pequeño)

En Claude Code:

> "fix: corregir typo en CHANGELOG.md línea 42"

El servidor clasifica `intent=fix` → `mode=express` → arranca el flow
con 2 fases pre-armadas:

1. `sdd-apply` con `system_prompt` desde `agent_templates` + `user_prompt`
   que cita el raw_text del usuario
2. `sdd-verify` con un prompt genérico (sdd-verify tolera ausencia del
   output de apply en Express porque el cliente lo tiene en su contexto)

Claude Code ejecuta `sdd-apply`, hace el edit, corre tests, y reporta:

```jsonc
domain_orchestrate_phase_result({
  "flow_run_step_id": "<step_apply.id>",
  "output": {
    "files_changed": ["CHANGELOG.md"],
    "lines_changed": 1,
    "summary": "fix typo línea 42"
  },
  "memory_refs_saved": [
    { "type": "code_reference", "id": "<observation_id>" }
  ]
})
```

D5 valida que `code_reference` esté presente (es **required** en
sdd-apply). Si todo OK, devuelve `NextStepPrompt` con el prompt de
`sdd-verify`. Cliente ejecuta los Gherkin scenarios, reporta verify
completed, flow termina.

### Ejemplo Full (10 fases, refactor)

> "refactor: extraer ResponseShape a un paquete propio en internal/api/response"

`intent=refactor` → `mode=full` → 10 steps en BD, sólo `sdd-explore`
con prompt construido up-front. Cliente avanza fase por fase; cada
phase_result reconstruye el prompt del siguiente step usando los
outputs acumulados (lazy build).

Las fases **D5 required** (`sdd-design`, `sdd-apply`, `sdd-judge`) van
a fallar el step si el cliente no guardó la `memory_ref` del tipo
correcto antes de reportar.

### D1 confirm condicional (Express only)

Si en Express el `sdd-apply.output` reporta `files_changed > 1` o
`lines_changed > 10`, el `sdd-verify` step se marca **`blocked`** y la
respuesta de phase_result trae `requires_confirm: true`. El cliente
debe llamar:

```jsonc
domain_orchestrate_confirm({
  "flow_run_id": "<flow_run.id>",
  "confirmed": true     // false para abortar
})
```

El step pasa a `pending` y el cliente continúa con su prompt cacheado.

### Consultar estado / reanudar

```bash
./bin/domain workflow resume <flow_run_id>
```

Imprime tabla numerada de los 10 steps con su status + preview del
prompt del próximo step pending o blocked. Útil después de una sesión
cortada.

## Diagramas por tipo de issue (anterior)

Ver [`docs/flows/README.md`](./flows/README.md) (legacy wizard) y
[`docs/agents/sdd-pipeline.md`](./agents/sdd-pipeline.md) (orquestador
nuevo).

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
