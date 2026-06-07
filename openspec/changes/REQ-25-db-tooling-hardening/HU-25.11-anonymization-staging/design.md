# Design: HU-25.11-anonymization-staging

## Architecture

```
internal/tools/anonymize/
  cmd.go              # CLI wiring
  catalog.go          # tabla → []ColumnTransform
  transforms.go       # email, name, rut, content, etc.
  walker.go           # pgx stream rows → transform → write SQL
  faker.go            # gofakeit wrappers deterministic
  detector_test.go    # grep PII real en output
```

## Catalog (table → transforms)

```go
var catalog = map[string][]ColTransform{
  "users": {
    {Col: "email",       Fn: AnonymizeEmail},      // user_<hash(id)>@example.test
    {Col: "name",        Fn: AnonymizeName},
    {Col: "rut",         Fn: AnonymizeRUT},        // deterministic fake RUT valid DV
    {Col: "phone",       Fn: NullOut},
  },
  "observations": {
    {Col: "content",     Fn: AnonymizeText},
  },
  "secrets": {
    {Col: "encrypted_value", Fn: NullOut},
  },
  "stripe_events_processed": {
    {Col: "stripe_event_id", Fn: HashTokenize},
  },
  // ...
}
```

## Catalog completeness test

```go
func TestCatalogCovers PIIColumns(t *testing.T) {
  // load schema, find columns with names like "email", "rut", "password*", "token*", "name", "phone", "address"
  // assert catalog has transform for each
}
```

## CLI

```bash
domain-mcp anonymize-dump \
  --source "$PROD_DB_URL" \
  --output /tmp/staging-dump.sql.gz \
  --seed 42 \
  --tables users,observations,secrets,...  # default: all
```

## TDD plan

1. Export con fixture data → grep emails reales 0
2. RUT real (válido) NO aparece; RUTs en output válidos pero diferentes
3. FK preservation: restore + ANALYZE + EXPLAIN works
4. Reproducible mismo seed
5. Catalog completeness test
6. RBAC enforce
