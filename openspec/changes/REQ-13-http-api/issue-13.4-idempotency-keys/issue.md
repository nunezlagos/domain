# issue-13.4-idempotency-keys

**Origen:** `REQ-13-http-api`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** cliente API (SDK, retry layer)
**Quiero** enviar `Idempotency-Key` en POST y recibir misma respuesta si reintento
**Para** evitar duplicar recursos en redes inestables (siguiendo Stripe-like pattern)

## Criterios de aceptación

### Escenario 1: Primer request con key

```gherkin
Dado que envío POST /api/v1/observations con header `Idempotency-Key: abc-123`
Cuando el server procesa
Entonces se ejecuta normal
Y se persiste en `idempotency_records`:
  - key=abc-123
  - request_hash = SHA256(method+path+body)
  - response_status = 201
  - response_body = (serializado)
  - org_id, user_id
  - expires_at = now() + 24h
Y se responde 201 normal
```

### Escenario 2: Re-envío con misma key + mismo body

```gherkin
Dado que existe record para `abc-123` con response 201
Cuando envío misma request again
Entonces se devuelve cached: 201 con mismo body y header `Idempotent-Replayed: true`
Y NO se ejecuta lógica de nuevo
```

### Escenario 3: Misma key, body diferente

```gherkin
Dado que existe record `abc-123` con request_hash X
Cuando envío con misma key pero body diferente
Entonces 422 `{"error":"idempotency_conflict","message":"key reused with different request"}`
```

### Escenario 4: Key expirada

```gherkin
Dado que record `abc-123` tiene expires_at < now()
Cuando envío con esa key
Entonces se procesa como first request (overwrite del record viejo)
```

### Escenario 5: Concurrent same key

```gherkin
Dado que llegan 2 requests simultáneos con misma key
Cuando el server procesa
Entonces el primer worker hace SELECT FOR UPDATE + INSERT
Y el segundo espera (lock) y al desbloquearse encuentra record → devuelve cached
```

### Escenario 6: Solo POST/PATCH/DELETE

```gherkin
Dado que envío GET con Idempotency-Key
Cuando el server recibe
Entonces se ignora el header (GET ya es idempotent por HTTP semantics)
```

## Análisis breve

- **Qué pide:** middleware + tabla idempotency_records con SHA256 + locking + TTL
- **Esfuerzo:** S
- **Riesgos:** keys reusables maliciosamente; storage acumulado
