# issue-01.7-seeders-system

**Origen:** `REQ-01-core-platform`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** mantenedor de Domain
**Quiero** un sistema de seeders idempotente que pueble la BD al boot con catálogos esenciales (plans, model registry, templates, policies)
**Para** que toda info crítica para correr la plataforma viva en BD, no en archivos sueltos, y se versione con el binario

## Catálogos seedeados

| catálogo | tabla destino | fuente |
|----------|---------------|--------|
| Plans (Free, Pro, Enterprise) | `plans` | `seeds/plans.go` |
| LLM model registry + pricing | `model_registry` | `seeds/models.go` con embed yaml |
| Agent templates (5 built-in) | `agent_templates` | `seeds/agents/*.yaml` |
| Skill templates (built-in catalog) | `skill_templates` | `seeds/skills/*.yaml` |
| Flow templates (ejemplos starter) | `flow_templates` | `seeds/flows/*.yaml` |
| Prompt templates (notification, otp_email, etc.) | `notification_templates` | `seeds/notifications/*.yaml` |
| Platform policies (rules markdown) | `platform_policies` | `seeds/policies/*.md` (de `.claude/rules/`) |
| Error codes (catalog) | `api_error_codes` | `seeds/error_codes.yaml` |
| Cron jobs predefinidos | `system_crons` | `seeds/crons.go` |

## Criterios de aceptación

### Escenario 1: Seed run al boot

```gherkin
Dado que la app arranca con `DOMAIN_SEED_ON_BOOT=true` (default true)
Cuando termina migración golang-migrate
Entonces se ejecuta `seeds.RunAll(ctx, db, env)`
Y cada seeder se ejecuta secuencial
Y log info "seeder X: created N, updated M, skipped K"
Y arranque HTTP server después
```

### Escenario 2: Idempotencia UPSERT

```gherkin
Dado que app ya corrió y seedeó plans
Cuando reinicia
Entonces seeders ejecutan UPSERT (no INSERT)
Y rows existentes con misma slug se actualizan con valores nuevos del binary
Y rows borradas manualmente por admin NO se re-crean (track via `seed_managed` flag)
```

### Escenario 3: Versioning de seeds

```gherkin
Dado que existe tabla `seed_versions` con (seeder_name, version, applied_at)
Cuando un seeder corre
Entonces compara con `version` declarado en código
Y si version > applied → re-aplica + bump
Y si version <= applied → skip (idempotent fast path)
```

### Escenario 4: Embedded seeds via go:embed

```gherkin
Dado que `seeds/agents/code-reviewer.yaml` está en repo
Cuando build el binary
Entonces `//go:embed seeds/agents/*.yaml` incluye los YAMLs en el binary
Y el binary deployado contiene todos los seeds, sin filesystem dep
```

### Escenario 5: Per-env seeding

```gherkin
Dado que `DOMAIN_ENV=dev`
Cuando seeders ejecutan
Entonces además de catálogos common, ejecutan `seeds.DevOnly(...)` con:
  - Demo organization "acme-demo"
  - Demo users (alice@example.test, bob@example.test)
  - Demo projects, observations, runs
Y en prod NO ejecuta DevOnly
```

### Escenario 6: Disable global

```gherkin
Dado que `DOMAIN_SEED_ON_BOOT=false`
Cuando arranca
Entonces NO ejecuta seeders
Y log warn "seeders disabled"
Y CLI `domain seed --all` permite ejecutar manualmente
```

### Escenario 7: Selective seeding via CLI

```gherkin
Dado que admin quiere reseedear solo policies
Cuando ejecuta `domain seed --only=policies`
Entonces solo ese seeder corre
Y se reporta diff
```

### Escenario 8: Override desde DB (preservar customizaciones)

```gherkin
Dado que admin editó manualmente un agent_template via API
Y marcó `is_user_modified = true`
Cuando seeder ejecuta UPSERT
Entonces respeta `is_user_modified` y NO sobrescribe
Y solo actualiza campos `version` y `updated_at_from_seed`
Y log warn "skipped X due to user modifications"
```

### Escenario 9: Validation antes de apply

```gherkin
Dado que YAML embebido tiene shape inválida
Cuando seeder parsea
Entonces fail-fast con error claro
Y la app NO arranca (boot-time validation)
```

### Escenario 10: Dry-run

```gherkin
Dado que admin ejecuta `domain seed --all --dry-run`
Cuando se procesa
Entonces NO modifica BD
Y reporta qué crearía/actualizaría/skipearía
```

## Análisis breve

- **Qué pide:** framework Go seeders + go:embed catalogs + UPSERT idempotent + per-env + CLI + version tracking
- **Esfuerzo:** M
- **Riesgos:** override de customizations user; race en seed concurrente entre pods; tamaño binary con todos los embeds
