# Design: issue-37.1-self-enroll-shared-token

## Schema (migration 000098)

```sql
CREATE TABLE org_enrollment_tokens (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id   UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  token_hash        BYTEA NOT NULL,         -- bcrypt cost 12
  token_prefix      VARCHAR(20) NOT NULL,   -- "et_" + 16 chars, indexable
  role_on_enroll    VARCHAR(30) NOT NULL DEFAULT 'member',
  created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at        TIMESTAMPTZ,
  CHECK (role_on_enroll IN ('owner','admin','maintainer','member','viewer'))
);

-- Lookup por prefix (cheap)
CREATE INDEX org_enrollment_tokens_prefix_idx
  ON org_enrollment_tokens (token_prefix)
  WHERE revoked_at IS NULL;

-- UNIQUE active token por org: máximo 1 fila activa por org
CREATE UNIQUE INDEX org_enrollment_tokens_org_active_uniq
  ON org_enrollment_tokens (organization_id)
  WHERE revoked_at IS NULL;
```

Notas:
- Sin RLS (estos endpoints son admin-only de la org del path; el handler
  hace la check explícita).
- Sin TLS/encryption at-rest extra: el hash bcrypt ya protege el contenido
  igual que las api_keys.
- `created_by_user_id ON DELETE SET NULL`: si el admin que rotó se borra,
  no queremos romper el token.

## Servicio nuevo: internal/service/enrollment

```go
type Service struct {
    Pool  *pgxpool.Pool
    Audit audit.Recorder
}

// Token plaintext + metadata para el caller.
type RotateResult struct {
    Plaintext     string
    Prefix        string
    RoleOnEnroll  string
    CreatedAt     time.Time
    OrganizationID uuid.UUID
}

// Rotate revoca el activo y crea uno nuevo. Atomic en una tx.
// Si role == "" usa "member" como default.
func (s *Service) Rotate(ctx, orgID, actorID uuid.UUID, role string) (*RotateResult, error)

// GetMetadata devuelve info del token activo (si existe), SIN plaintext.
type Metadata struct {
    Exists       bool
    Prefix       string
    RoleOnEnroll string
    CreatedAt    time.Time
}
func (s *Service) GetMetadata(ctx, orgID uuid.UUID) (*Metadata, error)

// Revoke marca el activo como revoked_at=NOW. No crea nuevo.
func (s *Service) Revoke(ctx, orgID, actorID uuid.UUID) error

// Enroll valida el plaintext, crea user + api_key, devuelve plaintext key.
type EnrollResult struct {
    UserID         uuid.UUID
    OrganizationID uuid.UUID
    OrgName        string
    OrgSlug        string
    APIKey         string  // plaintext UNA vez
    APIKeyID       uuid.UUID
    KeyPrefix      string
    Role           string
    Email          string
    Name           string
}
func (s *Service) Enroll(ctx, plaintext, email, name string) (*EnrollResult, error)
```

### Enroll: algoritmo de validación

1. Validar formato del plaintext (`et_` + 16+ chars). Si malformado → `ErrInvalidToken`.
2. Extraer prefix (primeros 19 chars: `et_` + 16 random).
3. SELECT all rows WHERE token_prefix = $1 AND revoked_at IS NULL.
4. **bcrypt.CompareHashAndPassword** contra cada candidato (típicamente 1).
   Si ninguno match → `ErrInvalidToken`. Si match → tomar la fila.
5. Validar email regex + role.
6. Begin tx, INSERT users + INSERT api_keys con apikey.Generate("live").
7. Audit log + Commit.
8. Devolver EnrollResult.

**Constant-time matters**: aunque el prefix no exista (0 rows), corremos
un bcrypt dummy para que el timing sea indistinguible. Misma defensa que
otros endpoints anti-enumeration en el proyecto.

### Rotate: atomicity

```go
tx.Begin
  UPDATE org_enrollment_tokens SET revoked_at = NOW()
    WHERE organization_id = $1 AND revoked_at IS NULL
  
  INSERT org_enrollment_tokens (organization_id, token_hash, token_prefix,
                                role_on_enroll, created_by_user_id)
    VALUES ($1, $2, $3, $4, $5)
  
  audit.Record("enrollment_token.rotated", ...)
tx.Commit
```

UNIQUE constraint sobre `(organization_id) WHERE revoked_at IS NULL` garantiza
que jamás hay 2 tokens activos simultáneamente para la misma org.

## Handlers nuevos

```go
// internal/api/handler/enrollment.go

func (a *API) enrollSelf(w, r) {
  // POST /api/v1/auth/enroll
  // Sin auth Bearer. Lee header X-Enrollment-Token.
  // body { email, name }
  // Llama Service.Enroll, responde 201.
}

func (a *API) rotateEnrollmentToken(w, r) {
  // POST /api/v1/organizations/{id}/enrollment-token/rotate
  // Auth: principal owner/admin de esa org.
  // body opcional { role_on_enroll }
  // Llama Service.Rotate, responde 201 con plaintext UNA vez.
}

func (a *API) getEnrollmentTokenMetadata(w, r) {
  // GET /api/v1/organizations/{id}/enrollment-token
  // Auth: principal owner/admin.
  // Llama Service.GetMetadata, responde 200.
}

func (a *API) deleteEnrollmentToken(w, r) {
  // DELETE /api/v1/organizations/{id}/enrollment-token
  // Auth: principal owner/admin.
  // Llama Service.Revoke, responde 204.
}
```

## Allowlist

`POST /api/v1/auth/enroll` se agrega a `handler.AuthAllowlist()` para no
requerir Bearer. El gating real lo hace el `X-Enrollment-Token` header.

## Bootstrap extension

`bootstrap.Bootstrap()` ya crea org+user+api_key en una tx. Extendemos para
también crear el primer enrollment token con role_on_enroll="member" y
agregar `enrollment_token` + `enrollment_role` al `BootstrapResult`.

Cambio en la API: el response del POST /auth/bootstrap **incluye 1 nuevo
campo**. Es additive, no rompe clientes existentes (que simplemente lo
ignoran). El installer wizard podría imprimirlo en la consola para que el
admin lo guarde.

## Wiring

- `cmd/domain/main.go`: instanciar `enrollment.Service`, inyectar en
  `handler.API.Enrollment *enrollment.Service`.
- `internal/api/handler/api.go`: agregar 4 mux.HandleFunc + 1 entry en
  AuthAllowlist.
- `internal/auth/bootstrap/service.go`: extender Bootstrap para crear
  enrollment_token (puede usar `enrollment.Service.Rotate` o hacerlo
  inline en la misma tx).

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Token en .env del server, no en DB | No rotable sin restart. No multi-org. |
| B | Single-use tokens generados por admin uno por user | Es exactamente issue-36.1, ya implementado. REQ-37 quiere shared. |
| C | Cada token incluye org_id en el plaintext, no en DB | Tamper-evident requiere firma; complica. DB es más simple. |
| D | TTL automático (expira en 7d) | YAGNI por ahora. Agregable después con un campo expires_at + check. |
| E | Endpoint público de auto-creación de org (cualquiera crea org sin invite) | Out of scope. Solo enrollment a orgs existentes. |

## Por qué este diseño gana

- **Simple**: 1 tabla, 4 endpoints, 1 servicio.
- **Multi-tenant correcto**: cada org tiene su token, no global.
- **Atomic**: cada enroll es 1 tx (INSERT user + INSERT api_key).
- **Rotable sin restart**: token vive en DB.
- **Coexiste**: no rompe invitations, bootstrap ni AddMemberWithAPIKey.
- **Forward-compatible**: cuando llegue SMTP/2FA, el flow se desactiva
  globalmente con DELETE en todos los tokens y se vuelve a invitations.

## Riesgos

- **R1**: token compartido se filtra (Slack history, GitHub gist) → bots
  enrolan masivamente. **Mitigación**: rotación rápida desde la UI;
  rate-limit a `/auth/enroll` (ej. 10 enrolls/hora por IP) — TODO en iter 2.
- **R2**: brute force del plaintext del token. **Mitigación**: 32 bytes
  random base64url = ~190 bits entropía. Bcrypt cost 12 = ~250ms por
  intento. Impracticable.
- **R3**: timing attack para detectar prefix válido. **Mitigación**:
  bcrypt dummy si no hay candidatos en DB. Detallado arriba.
