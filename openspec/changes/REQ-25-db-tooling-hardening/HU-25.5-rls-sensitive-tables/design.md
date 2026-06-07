# Design: HU-25.5-rls-sensitive-tables

## Policy template

```sql
ALTER TABLE secrets ENABLE ROW LEVEL SECURITY;

CREATE POLICY secrets_org_isolation ON secrets
  USING (organization_id = current_setting('app.current_org', true)::uuid);

-- INSERT/UPDATE need WITH CHECK too
CREATE POLICY secrets_org_write ON secrets
  FOR ALL
  USING (organization_id = current_setting('app.current_org', true)::uuid)
  WITH CHECK (organization_id = current_setting('app.current_org', true)::uuid);
```

User-scoped tables:
```sql
CREATE POLICY otp_codes_user_isolation ON otp_codes
  USING (user_id = current_setting('app.current_user', true)::uuid);
```

## SET LOCAL helper

```go
// internal/store/tx.go
func (s *Store) WithOrgTx(ctx context.Context, orgID, userID uuid.UUID, fn func(tx pgx.Tx) error) error {
  return s.pool.BeginTxFunc(ctx, pgx.TxOptions{}, func(tx pgx.Tx) error {
    if _, err := tx.Exec(ctx, "SET LOCAL app.current_org = $1", orgID.String()); err != nil {
      return err
    }
    if _, err := tx.Exec(ctx, "SET LOCAL app.current_user = $1", userID.String()); err != nil {
      return err
    }
    return fn(tx)
  })
}
```

## Roles

```sql
CREATE ROLE app_user NOBYPASSRLS NOLOGIN;
CREATE ROLE app_admin BYPASSRLS NOLOGIN;
CREATE ROLE app_migrator NOLOGIN;
CREATE ROLE app_readonly NOBYPASSRLS NOLOGIN;
```

(HU-25.6 detalla más grants per role.)

## Linter test (Go)

```go
// internal/store/linter_test.go
// scans internal/ for db.Query without WithOrgTx wrapper
// fails CI if found
```

## TDD plan

1. SELECT con org A → solo rows A
2. SELECT con org B → solo rows B
3. Sin SET LOCAL → 0 rows
4. app_admin sin SET LOCAL → todas
5. INSERT WITH CHECK valida org match
6. Cross-org INSERT (id de otra org) → reject
7. Performance bench
8. Linter detecta query sin helper
