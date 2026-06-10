# issue-25.13-schema-conventions-linter

**Origen:** `REQ-25-db-tooling-hardening`
**Prioridad tentativa:** alta
**Tipo:** tooling

## Historia de usuario

**Como** mantenedor del schema
**Quiero** un linter que valide las conventions de `.claude/rules/db.md` automáticamente en CI sobre cada migration nueva
**Para** evitar drift de conventions que solo se atrapan en code review (y a veces no)

## Conventions validadas

(Derivadas de `.claude/rules/db.md` — single source of truth)

| convention | check |
|------------|-------|
| Naming snake_case plural | regex `^[a-z][a-z0-9_]*$` + plural detector |
| Columna `id UUID DEFAULT gen_random_uuid()` o `BIGSERIAL` PK | parsea CREATE TABLE |
| `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` | regex |
| `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` + trigger | regex + trigger search |
| FKs `<singular>_id` | regex |
| Tipo prohibido: `JSON` (usar `JSONB`) | grep |
| Tipo prohibido: `TIMESTAMP` sin tz | grep |
| Tipo prohibido: `FLOAT`/`REAL` para money | grep heuristic con name `*_usd`, `*_amount`, `price*` |
| Tipo prohibido: `CHAR(1)` boolean | grep |
| Trigger `set_updated_at_<table>` cuando hay `updated_at` | parse trigger |
| Header completo en migration | check 6 fields |
| FK ON DELETE strategy presente | regex `ON DELETE (CASCADE|SET NULL|RESTRICT)` |
| Índice GIN sobre tsvector | si tabla tiene `_tsv` column, requerir índice GIN |
| Particionado declarado donde aplica (tablas log) | warning si tabla en lista de "should partition" no está particionada |

## Criterios de aceptación

### Escenario 1: Migration con tabla mal nombrada

```gherkin
Dado que migration `000999_create_user.up.sql` (singular incorrect)
Cuando CI ejecuta `db-conventions-lint`
Entonces error "table name 'user' should be plural; suggested 'users'"
Y CI fail
```

### Escenario 2: Falta created_at

```gherkin
Dado que migration crea tabla sin `created_at`
Cuando linter procesa
Entonces error "table 'foo' missing required column 'created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()'"
```

### Escenario 3: Tipo JSON sin B

```gherkin
Dado que migration tiene `data JSON`
Cuando linter procesa
Entonces error "use JSONB instead of JSON for column 'foo.data'"
```

### Escenario 4: FK sin sufijo _id

```gherkin
Dado que migration tiene `org UUID REFERENCES organizations(id)`
Cuando linter procesa
Entonces error "FK column 'org' should be 'organization_id'"
```

### Escenario 5: Header incompleto

```gherkin
Dado que migration sin field `issue:` en header
Cuando linter procesa
Entonces error "missing required header field 'issue'"
```

### Escenario 6: Override explícito

```gherkin
Dado que un statement legítimamente viola una regla
Cuando comment `-- domain-lint-ignore-next: prefer-jsonb` precede
Entonces se ignora con audit log CI "convention override: prefer-jsonb at line N"
Y el resto del file SÍ se valida
```

### Escenario 7: Modo --fix

```gherkin
Dado que developer ejecuta `make db-conventions-fix`
Cuando linter detecta issues auto-fixables (e.g. JSON→JSONB, naming)
Entonces aplica fix en el .sql
Y muestra diff
Y dev revisa y commitea
```

### Escenario 8: Aplica solo a nuevas migrations

```gherkin
Dado que migration 000001 fue creada antes de esta HU y viola convention
Cuando linter corre
Entonces se ignora migrations anteriores a marker `--lint-baseline-from N`
Y solo valida 000(N+1)+ adelante
```

## Análisis breve

- **Qué pide:** parser SQL ligero + reglas en Go + integration con issue-25.3 squawk en mismo CI step + modo fix
- **Esfuerzo:** M
- **Riesgos:** falsos positivos frustran developers → modo override + fix; squawk no cubre conventions semánticas
