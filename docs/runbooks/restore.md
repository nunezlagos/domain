# Restore Runbook — Domain

**Owner**: SRE / DBA on-call.
**SLA**: RTO 1h, RPO 1h (gracias a incremental backups horarios).
**Última prueba**: pendiente.

## Cuándo aplicar este runbook

- Pérdida total de la BD (corrupción, hardware, ransomware)
- Restauración point-in-time tras incidente de datos
- Smoke test de DR mensual (drill)

## Pre-requisitos

- Credenciales S3 del bucket de backups (`DOMAIN_BACKUP_S3_KEY`, `DOMAIN_BACKUP_S3_SECRET`)
- Cipher pass para descifrar (`DOMAIN_BACKUP_CIPHER_PASS`) — distinta de las app keys
- Nuevo host Postgres 16 + pgvector + pgBackRest instalados
- Accesos a Grafana + AlertManager para verificación post-restore

## Procedimiento

### 1. Detener servicio Domain (si el cluster sigue parcial)

```bash
kubectl scale -n domain deployment/domain-app --replicas=0
```

### 2. Provisionar nuevo Postgres (si pérdida total)

```bash
# Helm postgres-operator o RDS instance nuevo
helm upgrade --install postgres-domain bitnami/postgresql \
  --set image.repository=pgvector/pgvector \
  --set image.tag=pg16 \
  --set primary.persistence.size=200Gi
```

### 3. Configurar pgBackRest en el nuevo host

```bash
cat > /etc/pgbackrest/pgbackrest.conf <<EOF
$(cat deploy/backups/pgbackrest.conf)
EOF
```

Exportar env vars:

```bash
export DOMAIN_BACKUP_S3_KEY="..."
export DOMAIN_BACKUP_S3_SECRET="..."
export DOMAIN_BACKUP_CIPHER_PASS="..."
```

### 4. Restaurar desde el último full + diffs + WAL

```bash
# Stop postgres si está corriendo
systemctl stop postgresql@16-main

# Limpia data dir (DESTRUCTIVO — confirmar)
rm -rf /var/lib/postgresql/16/main/*

# Restore con WAL replay hasta el último point posible
pgbackrest --stanza=domain --delta restore

# O point-in-time específico:
pgbackrest --stanza=domain --delta --type=time \
  --target="2026-06-07 14:30:00+00" restore
```

### 5. Iniciar Postgres y verificar

```bash
systemctl start postgresql@16-main

# Verificar versión del schema
psql -U domain -d domain -c "SELECT version FROM schema_migrations;"

# Conteo sanity check
psql -U domain -d domain <<SQL
SELECT 'organizations' AS table, COUNT(*) FROM organizations
UNION ALL SELECT 'users', COUNT(*) FROM users
UNION ALL SELECT 'observations', COUNT(*) FROM observations
UNION ALL SELECT 'agent_runs', COUNT(*) FROM agent_runs;
SQL
```

### 6. Re-encriptar secrets si la pass cambió

Domain tiene secrets cifrados (HU-02.3). Si la master key se rotó tras
el incidente, ejecutar:

```bash
domain secrets re-encrypt --old-key=$OLD_KEY --new-key=$NEW_KEY
```

### 7. Restart Domain apuntando al nuevo Postgres

```bash
# Actualizar el Secret K8s con el nuevo DSN
kubectl create secret generic domain-db \
  --from-literal=DOMAIN_DATABASE_URL="postgres://app_user:...@new-host:5432/domain" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl scale -n domain deployment/domain-app --replicas=2
```

### 8. Verificar salud y throughput

- `GET /health/ready` debe responder 200
- Métricas Prometheus: `domain_http_requests_total` retoma valores
- Tests smoke: crear observation, search, agent run
- Comparar contra snapshot previo al incidente: counts row, sample queries

## Drill mensual

Primer lunes del mes 10:00 UTC, SRE on-call ejecuta:

```bash
# En staging, restore al último backup + verifica
make drill-restore
```

Documentar en `docs/runbooks/drill-log.md` con fecha + duración + issues.

## Anti-patterns

- ❌ NUNCA `pgbackrest --stanza=domain restore` sin `--delta` (rewrite full)
- ❌ NUNCA reinit cluster perdiendo WAL no archivado
- ❌ NUNCA test restore directo en prod (siempre staging primero)
- ❌ NUNCA storage de pass cifrado en clear text ni en `.env` committeado

## Métricas a observar post-restore

| Métrica | Esperado |
|---------|----------|
| `domain_http_requests_total{status="2xx"}` | crece linealmente con tráfico |
| `domain_db_pool_in_use` | < 80% del cap |
| `domain_agent_runs_total{status="completed"}` | retoma ritmo |
| `domain_db_replication_lag_seconds` | < 5s (si hay replicas) |
