# Proposal: issue-25.8-resource-limits-timeouts

## Intención

Establecer timeouts strict por role, connection limits, work_mem calibrado, TLS verify-full obligatorio en prod, pg_hba.conf hardened.

## Scope

**Incluye:**
- ALTER ROLE para timeouts (statement, lock, idle_in_tx)
- ALTER ROLE CONNECTION LIMIT por role
- postgresql.conf calibration (shared_buffers, work_mem, etc.)
- ssl=on + ssl_min_protocol_version
- pg_hba.conf: solo hostssl + scram-sha-256
- Documentar jobs largos que necesitan override (deben usar app_migrator o role específico)
- App side: pgx URL con sslmode=verify-full + ssl_root_cert

**No incluye:**
- mTLS (futuro si demanda)
- Encryption-at-rest (delegado a cloud DB)

## Enfoque técnico

1. Migration ALTER ROLE para timeouts/limits
2. ConfigMap postgresql.conf con sizing por env (dev menor, prod calibrado)
3. CA cert montado vía Secret K8s + pgx sslrootcert env

## Riesgos

- Jobs batch legítimos largos: rol separado `app_batch` con statement_timeout=10min
- Cert expiration: monitoring de cert expiry alert 30d antes

## Testing

- 31s query con app_user → aborta
- Lock wait 11s → aborta
- idle_in_tx 60s → conn closed
- 201va conn app_user → too many
- sslmode=disable → reject
- Cert hostname mismatch → reject
