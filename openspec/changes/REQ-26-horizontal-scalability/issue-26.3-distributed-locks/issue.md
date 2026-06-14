# issue-26.3-distributed-locks

**Origen:** `REQ-26-horizontal-scalability`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** desarrollador
**Quiero** helper reutilizable de distributed locks (Postgres advisory) para coordinar operaciones cross-pod
**Para** no inventar el mecanismo cada vez

## Casos de uso

- "Solo 1 pod procesa schedule X a la vez"
- "Solo 1 pod ejecuta seed catalog"
- "Solo 1 pod consume signal Y"
- Operations donde concurrent execution causa duplicates/race

## Criterios de aceptación

### Escenario 1: Try-lock no-bloqueante

```gherkin
Dado que dos pods invocan `dlock.TryAcquire(ctx, "key-X")`
Cuando se procesa
Entonces el primero retorna `(lock, true, nil)`
Y el segundo retorna `(nil, false, nil)` sin esperar
Y la primera tx termina o conn cierra → lock auto-libera
```

### Escenario 2: Lock con TTL via session conn

```gherkin
Dado que la conn dedicada para lock cae
Cuando otra request intenta acquire
Entonces lo obtiene (Postgres libera al cerrar session)
Y NO hay state inconsistente
```

### Escenario 3: Lock waiting opcional

```gherkin
Dado que pod necesita esperar hasta acquire
Cuando llama `dlock.Acquire(ctx, "key", maxWait=10s)`
Entonces espera con polling cada 200ms hasta acquire o timeout
Y si timeout → error con info
```

### Escenario 4: Métricas

```gherkin
Dado que cada acquire/release publica métricas
Cuando query
Entonces:
  - `domain_dlock_acquired_total{key}`
  - `domain_dlock_failed_total{key,reason}`
  - `domain_dlock_held_duration_seconds{key}` histogram
```

## Análisis breve

- **Qué pide:** helper TryAcquire + Acquire con polling + métricas + docs
- **Esfuerzo:** S
- **Riesgos:** mismo issue PgBouncer transaction-pool (issue-26.2 design lo cubre con session-pool dedicado)
