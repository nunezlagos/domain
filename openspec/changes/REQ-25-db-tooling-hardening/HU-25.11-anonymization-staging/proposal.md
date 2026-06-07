# Proposal: HU-25.11-anonymization-staging

## Intención

CLI `domain-mcp anonymize-dump` que genera dump de prod con PII reemplazada deterministicamente, preservando estructura y FKs, para uso en staging/dev sin riesgo de leak.

## Scope

**Incluye:**
- Subcomando CLI
- Pipeline per-table con transforms tipados
- Deterministic anonymizer (seed = id) para reproducibilidad
- Faker library (gofakeit)
- RBAC platform_admin only
- Audit log
- Tests adversariales: grep PII real en output

**No incluye:**
- Anonymization live (in-place) — solo export
- Differential privacy / k-anonymity formal (futuro)

## Enfoque técnico

1. Walker que lee row-by-row con pgx
2. Per-table transform function en Go
3. Output stream a archivo gzip
4. PII detector test verifica output

## Riesgos

- Olvido un campo: catalog completo + test que falla si agrego columna sin transform declarado
- FKs: preservar id ↔ id, sólo anonimizar campos no-clave
- Performance: streaming + batch

## Testing

- Export + grep emails reales → 0 matches
- Export + grep RUTs reales → 0 matches
- FK integrity preserved (restore + check constraints)
- Reproducible: 2 exports mismo seed → idénticos
- RBAC enforce
