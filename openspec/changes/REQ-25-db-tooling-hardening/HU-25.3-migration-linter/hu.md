# HU-25.3-migration-linter

**Origen:** `REQ-25-db-tooling-hardening`
**Prioridad tentativa:** alta
**Tipo:** infrastructure

## Historia de usuario

**Como** developer
**Quiero** un linter automático en CI que rechace migraciones peligrosas
**Para** evitar locks largos, downtime, o pérdida de datos en deploys

## Patterns peligrosos a detectar

| pattern | severidad | reason |
|---------|-----------|--------|
| `ALTER TABLE ... ADD COLUMN ... NOT NULL` sin DEFAULT | error | rewrite full table, locks largos |
| `CREATE INDEX` sin `CONCURRENTLY` | error | bloquea writes |
| `DROP TABLE` / `DROP COLUMN` sin `IF EXISTS` deprecation marker | error | data loss potencial |
| `ALTER TABLE ... ALTER COLUMN ... TYPE` que rewrite | warning | locks variable |
| `VACUUM FULL` | error | exclusive lock |
| `ALTER TABLE ... ADD FOREIGN KEY` sin `NOT VALID` + posterior `VALIDATE` | warning | tabla locked durante validación |
| `LOCK TABLE` explícito | error | salvo override comment |
| Migration sin comentario header (descripción + autor + issue ref) | warning | docs incompletas |

## Criterios de aceptación

### Escenario 1: PR con migración peligrosa rechazada

```gherkin
Dado que un PR agrega `migrations/000999_xxx.up.sql` con `CREATE INDEX idx_foo ON observations(content)`
Cuando CI ejecuta `squawk` o equivalente
Entonces se reporta error: "CREATE INDEX must use CONCURRENTLY"
Y el check status falla
Y la branch protection bloquea merge
```

### Escenario 2: Override explícito

```gherkin
Dado que la migración tiene comentario `-- squawk-ignore-next-statement: prefer-text-field`
Cuando el linter procesa
Entonces se ignora ese statement con audit del override en CI log
```

### Escenario 3: Lint local

```gherkin
Dado que developer ejecuta `make db-lint`
Entonces corre el mismo linter local sobre migrations/
Y muestra mismos errores que CI
```

### Escenario 4: Migrations idempotentes verificadas

```gherkin
Dado que migration N existe
Cuando linter procesa
Entonces verifica que statements destructivos usan `IF EXISTS` / `IF NOT EXISTS`
Y migrations down hacen lo inverso a up (parsed best-effort)
```

### Escenario 5: Comentario header requerido

```gherkin
Dado que migration no tiene header comentado:
  -- migration: add_user_rut_column
  -- author: alice@x.com
  -- issue: #1234
  -- description: agrega columna RUT con validation
Cuando linter procesa
Entonces warning (no error) "missing header metadata"
```

## Análisis breve

- **Qué pide:** squawk o atlas integrado a CI + Makefile + override mechanism + header convention
- **Esfuerzo:** S
- **Riesgos:** falsos positivos frustran developers → override mechanism + tunable rules
