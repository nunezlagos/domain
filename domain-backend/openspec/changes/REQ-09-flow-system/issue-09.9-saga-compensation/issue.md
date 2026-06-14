# issue-09.9-saga-compensation

**Origen:** `REQ-09-flow-system`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer de flows que invocan APIs externas no transaccionales
**Quiero** declarar acciones de compensación (rollback) por step
**Para** revertir efectos parciales si un step posterior falla y el flow se aborta

## Modelo (Saga pattern)

- Cada step puede declarar `compensate: <skill_slug or inline action>`
- Si flow termina en `failed` después de N steps completos, motor ejecuta `compensate` de cada step **en orden inverso**
- Compensaciones también tienen retry policy + budget
- Compensaciones fallidas se quedan en `flow_compensation_failures` para intervención manual

## Criterios de aceptación

### Escenario 1: Compensate on failure

```gherkin
Dado que flow tiene steps:
  - A: create_user → compensate: delete_user
  - B: send_welcome_email → compensate: (none)
  - C: charge_card → compensate: refund_card
Y A y B completaron, C falla
Cuando el flow se marca failed
Entonces motor ejecuta compensaciones en reverso: B(none), A(delete_user)
Y status final = "failed_compensated"
Y audit log de cada compensación
```

### Escenario 2: Compensate fail

```gherkin
Dado que compensate de step A falla
Cuando se reintenta según retry policy y sigue fallando
Entonces se inserta en `flow_compensation_failures` con detalles
Y status final = "failed_compensation_failed"
Y se notifica admin (REQ-20)
```

### Escenario 3: Manual skip de compensación

```gherkin
Dado que existe `flow_compensation_failures` entry
Cuando admin POST /api/v1/runs/:id/compensation/:step_id/skip con razón
Entonces se marca skipped + audit
Y el flow_run.status final no cambia (queda failed_compensation_failed)
```

### Escenario 4: Compensaciones paralelas opcional

```gherkin
Dado que flow tiene flag `compensate_in_parallel: true`
Cuando se aborta
Entonces compensaciones se ejecutan en paralelo (no respeta orden)
Y si alguna falla, las demás continúan
```

## Análisis breve

- **Qué pide:** compensate config por step + executor reverso + failures table + skip endpoint
- **Esfuerzo:** M
- **Riesgos:** compensación no idempotente; dependencias entre compensaciones; loops si compensación falla y dispara otra
