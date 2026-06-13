# issue-31.3-vps-deploy-caddy-https

**Origen:** `REQ-31-mcp-http-vps-mode`
**Prioridad tentativa:** alta
**Tipo:** feature (ops)

## Historia de usuario

**Como** operador que mueve domain a un VPS Contabo
**Quiero** un script + documentación para deployar el server con HTTPS automático (Let's Encrypt), reverse-proxy, health checks y backup
**Para** que el server esté expuesto en `https://api.tudominio.com` con TLS válido, mientras Postgres y MinIO quedan en localhost (NO accesibles desde internet)

## Criterios de aceptación

### Escenario 1: Deploy a VPS Contabo, Caddy sirve HTTPS

```gherkin
Dado un VPS Contabo fresh (Ubuntu 24.04) con un dominio `api.tudominio.com` apuntando en DNS
Cuando corro `bash deploy/contabo/setup.sh <tu-dominio.com>` en el VPS
Entonces el script:
  - Instala Docker + Docker Compose si falta
  - Crea directorio /opt/domain con el repo clonado
  - Genera Caddyfile con reverse proxy a localhost:8000
  - Inicia Caddy (que obtiene cert de Let's Encrypt vía ACME)
  - Inicia el stack (domain server + Postgres + MinIO) via docker compose
  - Espera healthcheck /health verde
Y al final imprime: "deploy done. https://<tu-dominio.com>/health returns 200"
Y exit code 0
```

### Escenario 2: TLS válido desde internet

```gherkin
Dado que el deploy terminó OK
Cuando desde mi laptop corro `curl -v https://api.tudominio.com/health`
Entonces el handshake TLS usa cert de Let's Encrypt (issuer: Let's Encrypt Authority X3 o R10)
Y el response es 200 OK con body JSON de health
Y `curl --resolve api.tudominio.com:443:<vps-ip> -vI ...` muestra la cadena de certs completa
```

### Escenario 3: Postgres NO accesible desde internet

```gherkin
Dado que el deploy está corriendo
Cuando desde mi laptop corro `nmap -p 5432 api.tudominio.com` o `psql -h api.tudominio.com -U postgres`
Entonces el puerto 5432 retorna "connection refused" o filtered (NO abierto)
Y `nmap -p 9000,9001 api.tudominio.com` (MinIO) también retorna filtered/refused
Y solo el 443 está abierto (Caddy)
```

### Escenario 4: Health checks + restart automático

```gherkin
Dado que el server domain crashea (kill -9 al proceso)
Cuando systemd/docker detecta el crash
Entonces el container se reinicia automáticamente en <30s (RestartPolicy: unless-stopped)
Y el health check de Caddy (`/health`) empieza a retornar 200 de nuevo
Y el log de Caddy muestra el upstream recovered
```

### Escenario 5: Backup nightly configurable

```gherkin
Dado que `BACKUP_ENABLED=true` está en /opt/domain/.env
Cuando el cron diario `0 3 * * *` se dispara
Entonces corre `pg_dump $DOMAIN_DATABASE_URL | gzip > /backups/db-<date>.sql.gz`
Y guarda últimos 7 backups (rotate)
Y exit code 0
Y hay un health check que verifica que el último backup es <26h viejo
```

### Escenario 6: Sabotaje — Caddy config apunta al puerto equivocado

```gherkin
Dado que el Caddyfile tiene `reverse_proxy localhost:9999` (puerto equivocado, sabotaje)
Cuando el deploy termina
Entonces el health check `https://api.tudominio.com/health` retorna 502 Bad Gateway (Caddy no encuentra upstream)
Y el script de deploy DEBE detectar este caso y reportar error claro
Cuando restauro el puerto correcto (8000)
Entonces el health check verde
```

### Escenario 7: Edge case — DNS no propagado aún

```gherkin
Dado que el dominio `api.tudominio.com` NO está apuntando al VPS (DNS no propagado)
Cuando corro el script de deploy
Entonces Caddy NO puede obtener cert de Let's Encrypt (ACME DNS challenge falla)
Y el script loggea: "DNS not propagated. Wait and retry, or check with `dig api.tudominio.com`"
Y exit code != 0
Y NO inicia el stack (espera al DNS)
```

## Notas

- Caddy es elegido sobre nginx porque auto-gestiona Let's Encrypt
  sin config extra (vs certbot + nginx que requiere cron + reload).
- MinIO NO se expone directamente. Si el dashboard lo necesita,
  se accede via presigned URLs generadas por el server (firmadas
  con la secret key local, válidas por N minutos).
- El script de deploy es IDEMPOTENTE: se puede correr 2 veces
  sin romper nada.
- Los secrets (DOMAIN_MASTER_KEY, S3 keys, DB password) se generan
  en el primer deploy y se guardan en `/opt/domain/.env` (chmod 600).
  Rotación documentada en `deploy/contabo/rotate-secrets.md`.
