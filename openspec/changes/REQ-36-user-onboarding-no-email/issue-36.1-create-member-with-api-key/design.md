# Design: issue-36.1-create-member-with-api-key

## Contexto

Hoy `bootstrap.Service` (issue-01.9) ya implementa exactamente el patrón
"crear user + api_key + devolver plaintext UNA vez". Pero solo aplica al
**first-run** (cuando no hay users en la DB).

REQ-36 extiende el patrón a CADA creación de user post-bootstrap, gated por
RBAC: solo admins/owners de la org.

## Decisión arquitectónica

**Estrategia:** nuevo método en `orgsvc.Service` que reutiliza el algoritmo
de generación de keys del bootstrap, dentro de una sola tx.

### 1. Servicio

```go
// orgsvc.Service.AddMemberWithAPIKey crea user + api_key atómicamente.
// Retorna el plaintext de la key UNA sola vez al caller.
//
// Defensa-en-profundidad: la verificación RBAC (owner/admin) la hace el
// handler antes de invocar. El service confía en el caller.
func (s *Service) AddMemberWithAPIKey(
    ctx context.Context,
    orgID, actorID uuid.UUID,
    email, name, role string,
) (*MemberWithKey, error)

type MemberWithKey struct {
    User       Member
    APIKey     string    // plaintext, UNA sola vez
    APIKeyID   uuid.UUID
    KeyPrefix  string
}
```

Pasos internos:
1. Validar email (regex), role (whitelist), name (puede estar vacío)
2. Begin tx
3. Verificar org existe + no deleted
4. INSERT users (con bcrypt dummy hash; user usa api_key como credencial)
5. Generar plaintext + bcrypt hash de la key (reutiliza algoritmo de bootstrap)
6. INSERT api_keys con expires_at=NULL, environment=live, name="default"
7. Audit log: member.created_with_key
8. Commit
9. Retornar plaintext (perdida si caller no la guarda)

### 2. Handler

`POST /api/v1/organizations/{id}/members`

```go
func (a *API) addMemberWithKey(w http.ResponseWriter, r *http.Request) {
    // 1. Auth: principal != nil
    // 2. Parse orgID del path
    // 3. RBAC: principal.OrganizationID == orgID
    // 4. RBAC: principal.Role in ("owner", "admin")
    // 5. Decode body { email, name, role }
    // 6. Validar campos
    // 7. Call orgsvc.AddMemberWithAPIKey
    // 8. Map errores a status codes
    // 9. 201 + Location + data
}
```

Mapping de errores:
- `ErrInvalidEmail` → 422 validation_failed
- `ErrInvalidRole` → 422 invalid_role
- `ErrEmailTaken` (constraint violation) → 409 email_taken
- `ErrOrgNotFound` → 404 not_found (anti-enumeration)
- otro → 500 create_member

### 3. Generación de la key

Movemos `bootstrap.generateAPIKey()` a un helper interno compartido:

```
internal/auth/apikey/keygen.go
  func GenerateLiveKey() (plaintext string, hash []byte, prefix string, err error)
```

`bootstrap.Service` y `orgsvc.Service.AddMemberWithAPIKey` consumen el mismo
helper. Esto evita drift entre los dos algoritmos.

### 4. RBAC check

El check `role in (owner, admin)` se hace en el handler. Si en el futuro
introducimos roles custom (issue-02.8 custom_roles), el check pasa al
`RoleService.HasPermission(principal, "organization.members.create")`.

Por ahora: chequeo simple basado en `principal.Role`.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Extender `POST /invitations` con flag `direct=true` que skipea email + auto-acepta | Mezcla 2 flujos en un endpoint. Más complejo. Mejor 2 endpoints claros. |
| B | Cambiar `bootstrap.Bootstrap` para que funcione siempre (no solo first-run) | Cambia semantica de un endpoint público que ya tiene clientes (el wizard de install). |
| C | Generar plaintext en el server pero NO retornarlo; mandarlo por SMTP | Justamente lo que queremos evitar (no hay SMTP). |
| D | Endpoint que retorna múltiples plaintext (batch members) | YAGNI. Si el admin necesita batch, hace múltiples requests. |

## Por qué B + helper compartido gana

- **Reutiliza** el algoritmo de keygen probado (bootstrap.generateAPIKey)
- **Atomic**: una sola tx con user + api_key
- **Seguro**: plaintext una sola vez en response, nunca persistido
- **RBAC claro**: el handler decide, el service confía
- **Compatible**: invitations flow sigue funcionando para quien lo use

## Detalle de implementación

- `internal/auth/apikey/keygen.go`: nuevo, exporta `GenerateLiveKey`
- `internal/auth/bootstrap/service.go`: deja de tener `generateAPIKey` privado, usa `apikey.GenerateLiveKey`
- `internal/service/org/service.go`: agrega `AddMemberWithAPIKey` + `MemberWithKey` struct + `ErrEmailTaken`
- `internal/api/handler/org.go`: agrega `addMemberWithKey` handler
- `internal/api/handler/api.go`: registra `POST /api/v1/organizations/{id}/members`
- Tests:
  - Unit: keygen formato (`internal/auth/apikey/keygen_test.go`)
  - Unit: validaciones del service (email, role) sin hit DB
  - Integration: e2e test contra DB (RLS + atomicidad) — deferido si no hay
    testcontainers en esta sesión

## Riesgos

- **R1:** alguien lee el plaintext del log si el handler hace `slog.Info(..., slog.String("api_key", plaintext))` por error.
  **Mitigación:** revisor humano + linter de keys prohibidas en logs (issue-17.3 ya enforced).
- **R2:** doble-clic del frontend crea el mismo user 2 veces.
  **Mitigación:** constraint unique en `(organization_id, email)` ya existe en la tabla `users`. El segundo POST devuelve 409.
- **R3:** atacante hace fuerza bruta de la API key creada.
  **Mitigación:** 32 chars random de un charset de 62 (~190 bits de entropía). Brute force impracticable.
