# issue-18.1-postgres-backups

**Origen:** `REQ-18-backup-dr`
**Prioridad tentativa:** alta
**Tipo:** infrastructure

## Historia de usuario

**Como** operador
**Quiero** backups automáticos full + WAL continuo de Postgres con retención configurable y PITR
**Para** poder recuperar la DB a cualquier punto dentro de la ventana de retención ante incidente

## Criterios de aceptación

### Escenario 1: Backup full diario

```gherkin
Dado que pgBackRest está configurado contra el cluster Postgres
Y `DOMAIN_BACKUP_FULL_CRON="0 2 * * *"`
Cuando se ejecuta el cron
Entonces se genera un backup full en el repository S3
Y el backup queda registrado en `pgbackrest info`
Y el backup tiene checksum validado
```

### Escenario 2: WAL archiving continuo

```gherkin
Dado que `archive_mode=on` y `archive_command='pgbackrest --stanza=domain archive-push %p'`
Cuando se generan WAL segments en Postgres
Entonces los WAL se suben al repository S3 con latencia <30s
Y `pgbackrest info` reporta WAL archive size creciente
```

### Escenario 3: Point-In-Time Recovery

```gherkin
Dado que existen backups full + WAL en S3 cubriendo los últimos 30 días
Cuando ejecuto `pgbackrest --type=time --target='2026-06-05 14:30:00' --stanza=domain restore`
Entonces la DB se restaura al estado de ese instante
Y se logea audit_log del restore
```

### Escenario 4: Retención automática

```gherkin
Dado que `DOMAIN_BACKUP_RETENTION_DAYS=30`
Cuando hay backups full más viejos que 30 días
Entonces el cron diario los purga junto con sus WAL dependientes
Y queda al menos un backup full completo siempre
```

### Escenario 5: Notificación en fallo

```gherkin
Dado que el backup falla por permisos S3
Cuando el cron termina con exit != 0
Entonces se envía notificación al canal configurado (REQ-20)
Y se incrementa la métrica `domain_backup_failed_total`
```

## Análisis breve

- **Qué pide:** pgBackRest contra S3 + cron + retention + notificaciones de fallo
- **Esfuerzo:** M
- **Riesgos:** repositorio S3 puede llenarse; bandwidth costoso si DB grande
