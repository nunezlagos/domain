# issue-36.1-create-member-with-api-key

**Origen:** `REQ-36-user-onboarding-no-email`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** admin/owner de una org
**Quiero** crear un user adicional con su API key sin pasar por email/OTP
**Para** poder onboarding cuando no hay SMTP configurado (deploy cloud)

## Criterios de aceptación

### Escenario 1: admin crea member con role member

```gherkin
Dado que estoy autenticado como owner de org A
Cuando hago POST /api/v1/organizations/<orgA_id>/members con body:
  { "email": "alice@example.com", "name": "Alice", "role": "member" }
Entonces response 201 con body:
  {
    "data": {
      "user": { "id": "<uuid>", "email": "alice@example.com", "name": "Alice", "role": "member" },
      "api_key": "domk_live_<32 chars>",
      "api_key_id": "<uuid>",
      "key_prefix": "domk_live_xxxxxxxx"
    }
  }
Y Location: /api/v1/users/<user_id>
Y el plaintext aparece UNA sola vez (un segundo request al mismo endpoint
  con misma email retorna 409, sin re-emitir key)
Y un registro audit_log con action=member.created_with_key
```

### Escenario 2: no admin → 403

```gherkin
Dado que estoy autenticado como member (role=member) de org A
Cuando hago POST /api/v1/organizations/<orgA_id>/members con body válido
Entonces response 403 con code "forbidden"
Y el body de error indica "owners/admins only"
```

### Escenario 3: email duplicado en la org → 409

```gherkin
Dado que ya existe un user con email alice@example.com en org A
Cuando un admin hace POST /api/v1/organizations/<orgA_id>/members con
  body { "email": "alice@example.com", ... }
Entonces response 409 con code "email_taken"
Y NO se crea ningún user nuevo ni api_key (atomicidad)
```

### Escenario 4: email inválido → 422

```gherkin
Dado que paso "no-email-valido" como email
Cuando hago POST ... con body { "email": "no-email-valido", ... }
Entonces response 422 con code "validation_failed"
Y details menciona el campo "email"
```

### Escenario 5: role inválido → 422

```gherkin
Dado que paso role="superadmin" (no existe en domain)
Cuando hago POST ... 
Entonces response 422 con code "invalid_role"
Y details enumera los roles válidos (owner, admin, member, viewer)
```

### Escenario 6: org no existe o no soy member → 404

```gherkin
Dado que mi principal pertenece a org A
Cuando intento POST /api/v1/organizations/<org_X>/members
  donde X != A
Entonces response 404 (anti-enumeration: no distingue "no existe" vs
  "no autorizado")
```

### Escenario 7: atomicidad — fallo en INSERT api_keys

```gherkin
Dado que el INSERT users tiene éxito pero el INSERT api_keys falla por
  cualquier razón (constraint, lock timeout, etc.)
Cuando se ejecuta el handler
Entonces NINGÚN user se queda creado (rollback de la tx)
Y response 500 con code "create_member"
```

### Escenario 8: sabotaje — devolver el plaintext en GET posterior

```gherkin
Dado que el admin ya creó el user con su key
Y el código tiene un bug (sabotaje) que persiste el plaintext en algún campo
Cuando alguien hace GET /api/v1/api-keys/<id>
Entonces el response contiene "key_plaintext" (NO debería)
Y el test e2e que assserta "GET api-keys nunca retorna plaintext" DEBE FALLAR
Cuando restauro la lógica que solo lo devuelve en POST /members
Entonces el test verde
```

## Notas

- El plaintext de la key se hashea con bcrypt cost 12 (mismo que bootstrap)
- key_prefix público: "domk_live_" + primeros 8 chars del random
- El user creado NO tiene password_hash real — usa un dummy bcrypt como
  bootstrap (issue-01.9). El user usa la API key como credencial primaria
- Audit log: action="member.created_with_key", entity=organization,
  new_values={email, role, key_prefix} (NO el plaintext — security.md)
- Los webhooks outbound (issue-10.4) que disparan en `invite.created`
  pueden agregar un nuevo evento `member.created_with_key` en una iteración
  futura. Por ahora no lo emitimos.
- El endpoint requiere principal admin/owner de la org del path. Si el
  user es member de otra org, NO puede crear miembros (RBAC normal).
