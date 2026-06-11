# Tasks: issue-25.5-rls-selective-tables

- [x] **rls-001**: Migrations RLS → migration 000028 aplica RLS+FORCE en las 5 tablas sensibles existentes (secrets, api_keys, activity_log, otp_codes, audit_log); las restantes del Tier-1 se suman cuando sus tablas se creen (tracked en state notes) — 2026-06-10
- [x] **rls-002**: Policies USING + WITH CHECK → migration 000028 (org en 4 tablas, user en otp_codes, audit_log INSERT append-only)
- [x] **rls-003**: Helper → `internal/store/txctx` WithOrgTx / WithUserTx / WithOrgUserTx (SET LOCAL via set_config, rechaza uuid.Nil)
- [x] **rls-004**: Refactor services para usar helper → services de tablas RLS consumen txctx; el resto valida org en WHERE (ver `.claude/rules/connection-pools.md`)
- [ ] **rls-005**: Linter test scan queries sin helper (pendiente — familia linters issue-25.13)
- [x] **rls-006**: Roles app_admin BYPASSRLS para batch jobs → migration 000025 + pools.Auth (connection-pools.md)
- [x] **rls-007**: Tests aislamiento 2 orgs → TestRLS_{Secrets,ActivityLog,APIKeys}_OrgIsolation
- [ ] **rls-008**: Performance bench <5% regression (pendiente — junto a issue-27.4 benchmarks)
- [x] **test-001**: Cross-org SELECT bloqueado → TestSabotage_RLS_CrossOrgLeak_Blocked
- [x] **test-002**: SET LOCAL correcto → TestRLS_SetLocalScopedToTx
- [x] **test-003**: Sin SET LOCAL → 0 rows → TestSabotage_RLS_NoSetLocal_ZeroRows
- [x] **test-004**: app_admin bypass → cubierto por pools.Auth en auth path (apikey.Resolve cross-org funciona via BYPASSRLS; tests de auth integración)
- [x] **test-005**: INSERT WITH CHECK → TestSabotage_RLS_InsertWrongOrg_Rejected
- [ ] **test-006**: Sabotaje linter detecta (requiere rls-005)
- [x] **docs-001**: `docs/db/rls.md` con policies + helper usage — 2026-06-10
