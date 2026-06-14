# issue-25.11-anonymization-staging

**Origen:** `REQ-25-db-tooling-hardening`
**Prioridad tentativa:** media
**Tipo:** tooling

## Historia de usuario

**Como** developer/QA
**Quiero** poder copiar dump prod → staging/dev con PII redacted/anonymized
**Para** debug con datos realistas sin riesgo de leak

## Campos a anonimizar

| tabla | campos |
|-------|--------|
| users | email → `user_<id>@example.test`, name → `User-<id>`, rut → fake_chile_rut(seed=id) |
| organizations | name → `Org <id>`, slug → `org-<id>` |
| observations | content → fake_text(deterministic seed=id) |
| knowledge_docs | body → fake_text |
| prompts | body → fake_text |
| api_keys | key_hash → re-hashed dummy; key_encrypted → null |
| secrets | encrypted_value → null + flag `anonymized: true` |
| stripe_* | tokens reemplazados por `anon_<id>` |
| sessions | invalidated all |
| otp_codes | DELETE all |
| invitations | DELETE all |

## Criterios de aceptación

### Escenario 1: Tool exportar anonymized dump

```gherkin
Dado que ejecuto `domain-mcp anonymize-dump --source prod --output /tmp/staging-dump.sql.gz`
Cuando se procesa
Entonces:
  - hace pg_dump --data-only
  - aplica transforms per-table (Go pipeline)
  - genera SQL output gzipped
Y NO contiene PII real (email/name/content/RUT)
Y RUT generated valida módulo 11 (fake pero parseable)
Y data preserves cardinality and referential integrity (FKs OK)
```

### Escenario 2: Restore en staging

```gherkin
Dado que existe staging cluster vacío
Cuando ejecuto `psql staging < staging-dump.sql.gz`
Entonces todas las queries de la app funcionan
Y referential integrity preservada
Y NO hay PII real
```

### Escenario 3: Deterministic anonymization

```gherkin
Dado que se ejecuta dump 2 veces de mismo source
Cuando se procesa
Entonces user con id X siempre se anonimiza a `User-<derivable-from-X>`
Y same RUT fake
Y permite testing reproducible
```

### Escenario 4: Audit del dump

```gherkin
Dado que se ejecuta export
Cuando termina
Entonces se logea audit "db.dump.anonymized" con who, when, output_size
Y NO se permite ejecutar sin RBAC platform_admin
```

### Escenario 5: Faker library

```gherkin
Dado que para content se usa `gofakeit` o similar
Cuando genera fake text
Entonces preserva longitud aproximada del original (±20%)
Y respeta encoding UTF-8
```

## Análisis breve

- **Qué pide:** CLI tool + per-table transforms + deterministic + RBAC + audit
- **Esfuerzo:** M
- **Riesgos:** se olvida un campo PII → leak; performance con datasets grandes; FK preservation
