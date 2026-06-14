# Tasks: issue-37.1-self-enroll-shared-token

## Backend

- [ ] **T1** Migration `000098_create_org_enrollment_tokens.up.sql`:
  - Tabla con id, organization_id (FK CASCADE), token_hash (BYTEA), token_prefix (VARCHAR 20), role_on_enroll, created_by_user_id (FK SET NULL), created_at, revoked_at
  - Index `org_enrollment_tokens_prefix_idx` partial WHERE revoked_at IS NULL
  - UNIQUE index `org_enrollment_tokens_org_active_uniq` partial WHERE revoked_at IS NULL (máximo 1 activo por org)
  - CHECK constraint role IN (...)
  - Header migration conventions (.claude/rules/migrations.md)

- [ ] **T2** Migration `000098_create_org_enrollment_tokens.down.sql`:
  - DROP TABLE IF EXISTS org_enrollment_tokens CASCADE

- [ ] **T3** Helper de generación de token en `internal/auth/enrollment/token.go`:
  - `GeneratePlaintext() (plaintext, prefix string, err error)`
  - Formato `et_<base64url 32 bytes>` ≈ 46 chars
  - Prefix = primeros 19 chars
  - Test unit: formato, entropy, no collisions

- [ ] **T4** Service `internal/service/enrollment/service.go`:
  - `Service{Pool, Audit}`
  - `Rotate(ctx, orgID, actorID, role) (*RotateResult, error)` — atomic: UPDATE revoked + INSERT new
  - `GetMetadata(ctx, orgID) (*Metadata, error)`
  - `Revoke(ctx, orgID, actorID) error`
  - `Enroll(ctx, plaintext, email, name) (*EnrollResult, error)` — tx con INSERT user + INSERT api_key
  - Errores tipados: `ErrInvalidToken`, `ErrEmailTaken`, `ErrInvalidEmail`, `ErrInvalidRole`
  - Constant-time bcrypt: si no hay candidatos, correr bcrypt dummy igual

- [ ] **T5** Handlers `internal/api/handler/enrollment.go`:
  - `enrollSelf` — POST /api/v1/auth/enroll, sin auth, lee X-Enrollment-Token
  - `rotateEnrollmentToken` — POST /api/v1/organizations/{id}/enrollment-token/rotate, RBAC owner/admin
  - `getEnrollmentTokenMetadata` — GET, RBAC
  - `deleteEnrollmentToken` — DELETE, RBAC

- [ ] **T6** Wire en `internal/api/handler/api.go`:
  - 4 mux.HandleFunc nuevos
  - Agregar `/api/v1/auth/enroll` a `AuthAllowlist()`
  - Field `Enrollment *enrollment.Service` en API struct

- [ ] **T7** Wire en `cmd/domain/main.go`:
  - Instanciar `enrollment.Service{Pool: pools.App, Audit: recorder}`
  - Inyectar en API

- [ ] **T8** Extender `bootstrap.Service.Bootstrap()` para crear primer enrollment_token:
  - Agregar campos `EnrollmentToken string`, `EnrollmentRole string` a `BootstrapResult`
  - En la misma tx del bootstrap, INSERT en org_enrollment_tokens
  - Update handler bootstrap para devolver los nuevos campos en el response

- [ ] **T9** response-shape-lint snapshot regen:
  - `REGEN_SNAPSHOTS=1 go test ./cmd/response-shape-lint/...`

## Tests

- [ ] **T-unit-1** `enrollment/token_test.go`: GeneratePlaintext formato y entropy
- [ ] **T-unit-2** `enrollment/service_test.go`: validaciones email/role sin DB
- [ ] **T-unit-3** `handler/enrollment_test.go`: response shapes con service mockeado
- [ ] **T-integration-1** (testcontainers, opcional): rotate → enroll → enroll mismo email = 409 → DELETE → enroll = 401

## Documentación

- [ ] **T10** state.yaml → implemented
- [ ] **T11** README en rama services: documentar el flow para el operador del VPS
