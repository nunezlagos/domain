# issue-18.3-restore-runbook

**Origen:** `REQ-18-backup-dr`
**Prioridad tentativa:** alta
**Tipo:** runbook

## Historia de usuario

**Como** operador on-call
**Quiero** un runbook ejecutable con steps reproducibles para restaurar Postgres + S3 ante incidente
**Para** cumplir RTO ≤ 1h sin depender del conocimiento tribal

## Criterios de aceptación

### Escenario 1: Runbook publicado y ejecutable

```gherkin
Dado que existe `docs/runbooks/restore.md` versionado
Cuando un nuevo operador lo lee
Entonces puede ejecutar los steps sin contexto adicional
Y existe checklist de pre-flight (acceso S3, KMS, kubectl, credenciales)
Y existe verificación post-restore (queries de smoke, cuenta de filas, login app)
```

### Escenario 2: Drill mensual automatizado

```gherkin
Dado que existe un cron job `restore-drill` en staging
Cuando se ejecuta el primer domingo de cada mes
Entonces toma el último backup de prod (read-only)
Y lo restaura en cluster staging efímero
Y ejecuta queries de smoke
Y publica reporte en `docs/runbooks/drills/YYYY-MM.md`
Y notifica al canal ops
```

### Escenario 3: RTO/RPO declarados y medidos

```gherkin
Dado que el drill terminó
Cuando inspecciono el reporte
Entonces contiene:
  | métrica       | valor             |
  | RPO real      | <=5 min           |
  | RTO real      | <=1 hora          |
  | data loss     | 0 rows            |
  | smoke queries | todas en verde    |
```

### Escenario 4: Rollback aborted-restore

```gherkin
Dado que un restore in-progress falla a mitad
Cuando ejecuto rollback documentado
Entonces el cluster vuelve al estado pre-restore sin pérdida
Y se logea incidente
```

## Análisis breve

- **Qué pide:** runbook markdown + script drill + métricas + reporte
- **Esfuerzo:** S
- **Riesgos:** drift entre prod y staging hace drill inválido
