# Proposal: issue-25.13-schema-conventions-linter

## Intención

Linter Go propio que enforce las conventions de `.claude/rules/db.md` sobre migrations en CI. Complementa squawk (issue-25.3 safety) con reglas semánticas de naming/tipos/columnas obligatorias.

## Scope

**Incluye:**
- Parser SQL ligero (pg_query_go o regex) sobre migrations
- Rules engine Go en `cmd/domain-lint-schema/`
- Reglas listadas en hu.md
- Override syntax `-- domain-lint-ignore-next: <rule>`
- Modo `--fix` para auto-correcciones
- Baseline marker para excluir migrations legacy
- Integration con CI issue-19.1 + Makefile target

**No incluye:**
- Validar runtime schema vs reglas (eso es issue-25.4 schema drift)
- Sugerir refactorings semánticos no-trivial

## Enfoque técnico

1. `github.com/pganalyze/pg_query_go/v5` para parse seguro SQL
2. AST walker aplicando rules
3. Output formato Reviewdog (CI annotation friendly)
4. Salida también JSON para integración future

## Riesgos

- Parser SQL no cubre 100% sintaxis: fallback a regex con warning
- Reglas demasiado estrictas frustran: override + fix mode
- Rules drift de docs: rule registry self-tests aseguran sync con db.md

## Testing

- Migration violando cada rule → error específico
- Override comment → skip + audit
- Fix mode → diff aplicado
- Baseline marker → skip <= N
- Migration limpia → 0 issues
