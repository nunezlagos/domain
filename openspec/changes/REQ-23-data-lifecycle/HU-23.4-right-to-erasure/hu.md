# HU-23.4-right-to-erasure

**Origen:** `REQ-23-data-lifecycle`
**Prioridad tentativa:** alta
**Tipo:** feature + compliance

## Historia de usuario

**Como** usuario europeo / chileno bajo Ley 19.628
**Quiero** ejercer mi derecho al olvido (GDPR Art. 17) sobre mis datos
**Para** que Domain elimine o anonimice todo lo que pueda identificarme

## Criterios de aceptación

### Escenario 1: Self-service erase

```gherkin
Dado que estoy autenticado como user X
Cuando POST /api/v1/me/erase con body `{"confirmation":"DELETE_ME"}`
Entonces se valida que confirmation = "DELETE_ME"
Y se ejecuta erase transaccional:
  - users row: email → "erased+<uuid>@example.invalid", rut/phone/name → NULL,
    is_erased = TRUE, erased_at = NOW
  - observations.created_by = userID → created_by = NULL (deja content por
    valor histórico)
  - sessions user_id = userID → user_id = NULL
  - prompts created_by = userID → created_by = NULL
  - knowledge_docs created_by = userID → created_by = NULL
  - agent_runs user_id = userID → user_id = NULL
  - api_keys: revocadas (revoked_at = NOW)
  - audit_log: queda intacto (legal hold) pero actor_id puede mapearse a NULL
    si query lo pide
Y se logea audit "user.erased" antes del erase
Y se retorna 200 con count de rows afectadas
```

### Escenario 2: Idempotencia

```gherkin
Dado que ya fui erased
Cuando POST /api/v1/me/erase otra vez
Entonces 409 "already_erased"
Y NO se ejecuta nada
```

### Escenario 3: Confirmation incorrecta

```gherkin
Dado que envío `{"confirmation":"yes"}`
Cuando POST /api/v1/me/erase
Entonces 422 "confirmation_required" con message:
  "Send {confirmation: 'DELETE_ME'} to confirm irreversible erase"
```

### Escenario 4: Audit trail preservado

```gherkin
Dado que erase completa
Cuando consulto audit_log
Entonces sigue existiendo el registro "user.erased" con timestamp + actor_id
Y los registros previos del user están intactos (legal hold compliance)
Y solo la PII del user en cada row queda anonimizada (NULL refs en otras tablas)
```

### Escenario 5: Admin erase de otro user

```gherkin
Dado que soy platform_admin
Cuando POST /api/v1/admin/users/{id}/erase con body `{"confirmation":"DELETE_ME","reason":"GDPR request 2026-06-07"}`
Entonces se ejecuta el mismo flow que self-service
Y se logea audit con actor_id = admin + entity_id = target user
```

### Escenario 6: Owner de org

```gherkin
Dado que soy owner de una org con otros members
Cuando intento POST /api/v1/me/erase
Entonces 409 "transfer_ownership_first" con message:
  "Transfer ownership of org X before erasing your account"
Y NO se ejecuta el erase
```

## Análisis breve

- **Qué pide:** Endpoint self-service + endpoint admin + lógica transaccional
  anonimizando PII conservando integridad referencial e historial audit.
- **Módulos:** internal/service/lifecycle/erasure.go, internal/api/handler/me.go
- **Riesgos:** owner sin transfer rompería org; audit log debe sobrevivir por
  compliance; rollback imposible una vez ejecutado.
- **Esfuerzo:** M
