# HU-13.5-bulk-batch-endpoints

**Origen:** `REQ-13-http-api`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** cliente API que necesita crear N entidades juntas
**Quiero** endpoints batch (POST /observations/batch) que acepten array y devuelvan resultado por item
**Para** evitar N round-trips

## Criterios de aceptación

### Escenario 1: Batch create observations

```gherkin
Dado que envío POST /api/v1/observations/batch con array de 500 items
Cuando el server procesa
Entonces se valida cada item individualmente
Y se crean en una sola transacción si `mode:"all_or_nothing"`
O en transacciones individuales si `mode:"best_effort"` (default)
Y se devuelve 207 Multi-Status con array de resultados:
  `[{index, status:201, id:"...", data:{...}}, {index, status:422, error:{...}}, ...]`
```

### Escenario 2: Límite de batch size

```gherkin
Dado que envío batch de 5001 items
Cuando el server valida
Entonces 413 "batch too large: max 5000 items"
```

### Escenario 3: All-or-nothing rollback

```gherkin
Dado que mode="all_or_nothing" y item 250 falla validación
Cuando se procesa
Entonces toda la transacción se rolea back
Y respuesta 422 indica index 250 + error sin crear nada
```

### Escenario 4: Idempotency aplica a batch

```gherkin
Dado que envío Idempotency-Key con batch
Cuando reenvío con misma key + mismo body
Entonces se devuelve cached response (HU-13.4)
```

### Escenario 5: Bulk delete

```gherkin
Dado que DELETE /api/v1/observations/batch con `{"ids":["...","..."]}`
Cuando se procesa
Entonces soft-delete masivo de los IDs accesibles
Y se devuelve 207 con array {id, status:204|404|403}
```

## Análisis breve

- **Qué pide:** endpoints /batch en entidades principales + Multi-Status response + mode all-or-nothing/best-effort + límite size
- **Esfuerzo:** M
- **Riesgos:** transactions largas; memory con 5k items; partial failure visibility
