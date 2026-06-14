# Proposal: issue-25.3-migration-linter

## Intención

Linter automático para migraciones SQL en CI que detecta patterns peligrosos (NOT NULL sin DEFAULT, CREATE INDEX sin CONCURRENTLY, DROP sin IF EXISTS, etc.) y bloquea merge si encuentra issues.

## Scope

**Incluye:**
- Squawk (`sbdchd/squawk`) o Atlas migrate lint integrado a CI
- Reglas configuradas en `.squawk.toml`
- Comment-based override mechanism
- Makefile target `db-lint` para local
- Header convention para migrations (linter check)
- Integration con issue-19.1 ci-lint-test workflow

**No incluye:**
- Lint runtime (post-apply) — solo pre-merge
- Schema design suggestions (out of scope)

## Enfoque técnico

1. Squawk binario instalado en CI step
2. `.squawk.toml` con reglas habilitadas/deshabilitadas
3. Override via comentarios SQL inline
4. CI fail si severidad >= warning configurable

## Riesgos

- Falsos positivos: override mecanismo + tunable
- Squawk no detecta todo: complementar con atlas o code review checklist

## Testing

- Migration con CREATE INDEX sin CONCURRENTLY → CI fails
- Override comment hace pasar
- ALTER NOT NULL sin DEFAULT → CI fails
- Migration limpia → CI pass
- Makefile db-lint local funciona
