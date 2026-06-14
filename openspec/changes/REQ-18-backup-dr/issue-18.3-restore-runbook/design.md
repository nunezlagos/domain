# Design: issue-18.3-restore-runbook

## Decisión arquitectónica

**Formato runbook:** markdown ejecutable (bloques de código copiables).
**Drill automation:** Kubernetes CronJob mensual + cluster efímero StatefulSet.
**Reporte:** markdown auto-generado committeado al repo en cada drill.

## Estructura del runbook

```
docs/runbooks/restore.md
├── 1. Pre-flight checks (10 min)
│   ├── Acceso S3/KMS/kubectl
│   ├── Inventario backups disponibles
│   └── Decisión target time (PITR)
├── 2. Restore Postgres
│   ├── 2.1 Crear cluster destino
│   ├── 2.2 pgbackrest restore --type=time
│   ├── 2.3 promover read-write
│   └── 2.4 Apuntar app DNS
├── 3. Restore S3 (si aplica)
│   ├── Identificar versión a restaurar
│   └── aws s3 cp --version-id
├── 4. Post-flight
│   ├── Smoke queries
│   ├── Verificar login app
│   └── Notificar resolución
├── 5. Rollback aborted-restore
└── 6. Post-mortem template
```

## Drill cluster

```
deploy/k8s/drill/
  cronjob.yaml          # schedule 0 4 1 * *
  job-template.yaml     # spawns ephemeral pg cluster, runs script, reports
  rbac.yaml             # SA con acceso read S3 prod backups
```

## TDD plan

1. Drill manual end-to-end → reporte verde
2. Sabotaje WAL → drill falla con mensaje claro
3. Cleanup TTL: cluster efímero borrado después de 2h aun si script colgado
