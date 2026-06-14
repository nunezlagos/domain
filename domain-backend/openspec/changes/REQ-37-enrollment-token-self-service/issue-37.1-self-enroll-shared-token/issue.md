# issue-37.1-self-enroll-shared-token

**Origen:** `REQ-37-enrollment-token-self-service`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** miembro nuevo de un equipo que usa domain en cloud
**Quiero** auto-enrolarme en la org con un token que me pasaron por Slack
**Para** obtener mi API key personal sin esperar que un admin la cree a mano

## Criterios de aceptación

### Escenario 1: enrollment con token válido

```gherkin
Dado que la org A tiene un enrollment_token activo "et_abc123..."
  con role_on_enroll="member"
Cuando alguien hace POST /api/v1/auth/enroll
  header: X-Enrollment-Token: et_abc123...
  body:   { "email": "alice@example.com", "name": "Alice" }
Entonces response 201 con body:
  {
    "data": {
      "user": { "id":..., "email":"alice@example.com", "role":"member", ... },
      "api_key": "domk_live_<32 chars>",
      "api_key_id": "<uuid>",
      "key_prefix": "domk_live_xxxxxxxx",
      "organization": { "id":..., "name":..., "slug":... }
    }
  }
Y se crea audit_log con action="user.self_enrolled"
Y el enrollment_token sigue activo (multi-use)
```

### Escenario 2: token revocado → 401

```gherkin
Dado que el enrollment_token "et_old" fue rotado y queda revoked_at != NULL
Cuando alguien hace POST /auth/enroll con header X-Enrollment-Token: et_old
Entonces response 401 code "invalid_token"
Y NO se crea user ni api_key
Y se loggea WARN en server con prefix del token (NUNCA full plaintext)
```

### Escenario 3: token inexistente → 401 (anti-enumeration)

```gherkin
Dado que pasamos un token random que no existe en DB
Cuando hago POST /auth/enroll
Entonces response 401 code "invalid_token"
Y el response es indistinguible (timing + body) de "token revocado"
```

### Escenario 4: email duplicado en la org → 409

```gherkin
Dado que ya existe alice@example.com en la org del token
Cuando alguien hace POST /auth/enroll con email=alice@example.com
Entonces response 409 code "email_taken"
Y NO se crea user ni api_key
```

### Escenario 5: rotación por admin

```gherkin
Dado que soy admin/owner de la org A
Cuando hago POST /api/v1/organizations/<orgA_id>/enrollment-token/rotate
  body opcional: { "role_on_enroll": "admin" }
Entonces response 201 con body:
  {
    "data": {
      "enrollment_token": "et_NEW...",   // plaintext UNA sola vez
      "role_on_enroll": "admin",
      "key_prefix": "et_xxxxxxxx",
      "created_at": "..."
    }
  }
Y el token anterior queda revoked_at=NOW()
Y se crea audit_log con action="enrollment_token.rotated"
```

### Escenario 6: GET muestra metadata (NO el plaintext)

```gherkin
Dado que soy admin/owner de la org A con un token activo
Cuando hago GET /api/v1/organizations/<orgA_id>/enrollment-token
Entonces response 200 con body:
  {
    "data": {
      "exists": true,
      "role_on_enroll": "member",
      "key_prefix": "et_xxxxxxxx",
      "created_at": "...",
      "rotated_at": null
    }
  }
Y NO contiene el plaintext del token (impossible re-obtenerlo después del rotate)
```

### Escenario 7: DELETE revoca sin crear nuevo

```gherkin
Dado que la org A tiene token activo
Cuando un admin/owner hace DELETE /api/v1/organizations/<orgA_id>/enrollment-token
Entonces response 204
Y el token queda revoked_at=NOW()
Y enrollment queda cerrado hasta nueva rotación
Y se crea audit_log con action="enrollment_token.revoked"
```

### Escenario 8: bootstrap emite primer token

```gherkin
Dado que la DB está vacía (first-run)
Cuando hago POST /api/v1/auth/bootstrap con body válido
Entonces response 201 con body que INCLUYE:
  {
    "data": {
      "user": {...},
      "organization": {...},
      "api_key": "domk_live_...",
      "enrollment_token": "et_...",      // NUEVO en issue-37.1
      "enrollment_role": "member"
    }
  }
Y la tabla org_enrollment_tokens tiene 1 fila con la org recién creada
```

### Escenario 9: no admin intenta rotar → 403

```gherkin
Dado que estoy autenticado como member (role=member) de org A
Cuando hago POST /organizations/<orgA_id>/enrollment-token/rotate
Entonces response 403 code "forbidden" con mensaje "owners/admins only"
```

### Escenario 10: sabotaje — token plaintext loggeado

```gherkin
Dado que el código tiene un bug (sabotaje) que loggea el plaintext del
  token al validarlo en el handler enroll
Cuando se hace POST /auth/enroll con token válido
Entonces los logs incluyen el plaintext "et_abc..."
Y el linter de issue-17.3 (security key prohibidas en logs) DEBE FAILAR
Cuando se restaura el log a usar solo key_prefix
Entonces test verde
```

## Notas

- **Storage**: tabla `org_enrollment_tokens` con bcrypt hash del token; el
  plaintext aparece UNA sola vez al rotar.
- **Formato del token**: `et_<base64url 32 bytes>` ≈ 46 chars. Prefijo
  `et_` distingue del API key (`domk_`).
- **Lookup**: por `token_prefix` (primeros 16 chars indexados) + verify
  bcrypt full. Mismo pattern que api_keys.
- **Atomicidad de enroll**: tx con INSERT users + INSERT api_keys. Si una
  falla, rollback completo.
- **Anti-enumeration**: tokens inválidos y tokens revocados devuelven el
  mismo 401 con el mismo timing (bcrypt compare se hace siempre, aunque
  no haya match).
- **Migration**: numerada 000098 (sigue después de 000097_audit_log_org).
- **Endpoint enroll en AuthAllowlist**: sin Bearer required.
- **Audit log obligatorio** en los 4 endpoints (enroll, rotate, get, delete).
