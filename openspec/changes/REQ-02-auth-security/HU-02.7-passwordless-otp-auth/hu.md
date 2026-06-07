# HU-02.7-passwordless-otp-auth

**Origen:** `REQ-02-auth-security`
**Persona:** security-officer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario final invitado a una organización
**Quiero** loguearme escribiendo mi RUT o email, recibir un código de 6 dígitos al correo y obtener mi API key
**Para** acceder al sistema sin password local ni dependencia de proveedores externos

## Modelo conceptual

- Cada usuario tiene **una sola** API key activa por cuenta
- El login no emite "sesiones" — emite/revela esa API key
- Al validar OTP, el user elige: **revelar la actual** o **regenerar** (rotar)
- Sin self-signup público: solo usuarios pre-existentes (creados por invitación admin de HU-21.2) pueden loguearse
- Login funciona por RUT chileno **o** email — ambos almacenados y únicos por usuario

## Criterios de aceptación

### Escenario 1: Solicitar OTP por email

```gherkin
Dado que existe user con `email="bob@x.com"` y `rut="12.345.678-5"` (creado por invitación)
Cuando POST /auth/request-otp con body `{"identifier":"bob@x.com"}`
Entonces se genera código numérico de 6 dígitos
Y se guarda hash bcrypt en tabla `otp_codes` con `expires_at = now() + 10min`
Y se envía email al user vía canal `email-smtp` (HU-20.2) con el código en plano
Y la respuesta es 200 `{"sent": true, "expires_in_seconds": 600}`
Y NO se incluye en la respuesta nada que confirme si el identifier existe (anti-enumeration)
```

### Escenario 2: Solicitar OTP por RUT

```gherkin
Dado que existe user con email y `rut="12.345.678-5"`
Cuando POST /auth/request-otp con `{"identifier":"12345678-5"}` (sin puntos)
O con `{"identifier":"12.345.678-5"}` (con puntos)
O con `{"identifier":"123456785"}` (sin separadores)
Entonces se normaliza a formato canónico `12345678-5`
Y se valida dígito verificador (módulo 11)
Y se busca user por `rut` normalizado
Y se envía OTP al email asociado al user
Y la respuesta NO revela el email parcial ni completo
```

### Escenario 3: RUT inválido

```gherkin
Dado que `identifier="11.111.111-1"` con dígito verificador incorrecto
Cuando POST /auth/request-otp
Entonces respuesta 200 con `{"sent": true, "expires_in_seconds": 600}` (igual que happy path, anti-enumeration)
Pero NO se envía ningún email
Y se logea internamente "rut.invalid_dv"
```

### Escenario 4: Identifier inexistente

```gherkin
Dado que NO existe user con ese email/rut
Cuando POST /auth/request-otp con `{"identifier":"ghost@x.com"}`
Entonces respuesta 200 idéntica a happy path
Pero NO se envía email
Y se logea internamente "otp.identifier_not_found"
```

### Escenario 5: Verificar OTP — modo "reveal" (usar API key actual)

```gherkin
Dado que user solicitó OTP y recibió código "482917"
Y el user ya tiene una API key activa
Cuando POST /auth/verify-otp con `{"identifier":"bob@x.com","code":"482917","action":"reveal"}`
Entonces se valida code contra hash bcrypt del último OTP activo del user
Y `attempts < 5` y `expires_at > now()` y `used_at IS NULL`
Y se marca `used_at = now()`
Y se devuelve 200 JSON simple:
  ```json
  {
    "api_key": "domk_live_a1b2c3...",
    "key_prefix": "domk_live_a1b2c3",
    "created_at": "2026-06-06T12:00:00Z",
    "regenerated": false,
    "user": { "id": "...", "email": "bob@x.com", "rut": "12345678-5" }
  }
  ```
```

### Escenario 6: Verificar OTP — modo "regenerate" (rotar)

```gherkin
Dado que user solicitó OTP y existe API key previa
Cuando POST /auth/verify-otp con `{"identifier":"...", "code":"482917", "action":"regenerate"}`
Entonces se valida OTP igual que escenario 5
Y se revoca la API key previa (`api_keys.revoked_at = now()`)
Y se genera nueva API key con `secrets/rand` (32 bytes base64)
Y se persiste hash bcrypt + key_prefix en `api_keys`
Y se devuelve 200 con `regenerated: true` y la nueva key
Y se logea audit_log "api_key.rotated_via_otp"
```

### Escenario 7: Primer login (user existe pero sin API key)

```gherkin
Dado que invitación fue aceptada y user existe sin API key activa
Cuando POST /auth/verify-otp con cualquier action
Entonces se genera API key fresh (independiente del action)
Y se devuelve con `regenerated: false` y campo `is_first: true`
```

### Escenario 8: OTP incorrecto

```gherkin
Dado que existe OTP activo
Cuando POST /auth/verify-otp con code incorrecto
Entonces `otp_codes.attempts++`
Y respuesta 401 `{"error":"invalid_code","attempts_remaining":4}`
Y si attempts >= 5: se marca `expired_at = now()` y respuesta 429 `{"error":"too_many_attempts"}`
```

### Escenario 9: OTP expirado

```gherkin
Dado que OTP tiene `expires_at < now()`
Cuando POST /auth/verify-otp
Entonces 410 `{"error":"otp_expired"}`
Y NO se incrementa attempts (ya está caducado)
```

### Escenario 10: OTP ya usado

```gherkin
Dado que OTP tiene `used_at IS NOT NULL`
Cuando POST /auth/verify-otp con el mismo code
Entonces 410 `{"error":"otp_already_used"}`
```

### Escenario 11: Rate limit en request-otp

```gherkin
Dado que un identifier solicitó OTP hace <60 segundos
Cuando POST /auth/request-otp con el mismo identifier
Entonces 429 `{"error":"rate_limited","retry_after_seconds":NN}`
Y NO se envía nuevo email
Y máx 5 solicitudes por identifier en 1h
Y máx 10 solicitudes por IP en 1h
```

### Escenario 12: User suspendido / org borrada

```gherkin
Dado que `users.deleted_at IS NOT NULL` o la org tiene `deleted_at`
Cuando POST /auth/request-otp
Entonces respuesta 200 fake (anti-enumeration) pero NO se envía email
Y NO se permite verify-otp aunque tuviera código previo válido
```

### Escenario 13: User pertenece a múltiples orgs

```gherkin
Dado que user es miembro de org A y B
Cuando se valida OTP
Entonces se devuelve la API key del user (única, no scoped por org)
Y el body incluye `organizations: [{id, slug, role}, ...]`
Y para operaciones específicas el cliente envía header `X-Organization-Id`
Y si no envía header, se usa la última org accedida (`users.last_organization_id`)
```

## Análisis breve

- **Qué pide:** flow passwordless 2 endpoints + tabla OTP + normalización RUT + integración API key + email canal SMTP
- **Módulos sospechados:** `internal/auth/otp/`, `internal/auth/rut/`, `internal/http/handlers/auth.go`, integración con `internal/notifications/channels/email/`
- **Riesgos:** enumeration de usuarios, brute-force OTP, replay del email, race en rotate
- **Esfuerzo tentativo:** M
