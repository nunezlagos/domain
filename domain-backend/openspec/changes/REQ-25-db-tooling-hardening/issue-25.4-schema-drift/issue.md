# issue-25.4-schema-drift

**Origen:** `REQ-25-db-tooling-hardening`
**Prioridad tentativa:** media
**Tipo:** infrastructure

## Historia de usuario

**Como** operador
**Quiero** detectar drift entre el schema real en producción y el schema esperado por las migraciones
**Para** identificar cambios manuales (DBA ad-hoc) o migrations que fallaron parcialmente

## Criterios de aceptación

### Escenario 1: Cron diario drift check

```gherkin
Dado que existe cron `schema-drift-check` cada 24h
Cuando se ejecuta
Entonces hace:
  - Aplica TODAS las migraciones a una DB efímera (template clean)
  - Hace pg_dump --schema-only de la DB efímera (expected)
  - Hace pg_dump --schema-only de la DB de producción (actual)
  - Diff normalizado (orden columnas, whitespace)
Y si hay diff → publica métrica + notifica admin con diff body
Y si no hay diff → métrica ok
```

### Escenario 2: Diff humano-legible

```gherkin
Dado que hay drift detectado
Cuando se notifica
Entonces el diff se formatea con --color y truncado a 100 líneas relevantes
Y se sube full diff a S3 con signed URL para inspección
```

### Escenario 3: Sources de drift comunes

```gherkin
Dado que un DBA ejecutó manualmente `ALTER TABLE ... ADD INDEX ...` sin migration
Cuando drift check corre
Entonces se detecta y reporta:
  - índice X presente en actual, ausente en expected
  - propone: "crear migration para formalizar O remover índice"
```

### Escenario 4: Migration parcialmente aplicada

```gherkin
Dado que migration 99 quedó en estado dirty (golang-migrate `--force`)
Cuando drift check
Entonces se detecta inconsistencia
Y se reporta + paginar SRE
```

### Escenario 5: Endpoint admin

```gherkin
Dado que GET /admin/db/schema-drift devuelve último resultado
Cuando se consulta
Entonces JSON con `{status, last_check_at, diff_summary, full_diff_url}`
```

## Análisis breve

- **Qué pide:** cron + 2 pg_dumps + diff + report + endpoint admin
- **Esfuerzo:** S
- **Riesgos:** pg_dump diff puede ser ruidoso por orden; normalización clave
