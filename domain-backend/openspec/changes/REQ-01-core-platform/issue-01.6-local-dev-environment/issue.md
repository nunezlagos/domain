# issue-01.6-local-dev-environment

**Origen:** `REQ-01-core-platform`
**Prioridad tentativa:** alta
**Tipo:** infrastructure

## Historia de usuario

**Como** desarrollador de la plataforma Domain
**Quiero** levantar con un solo comando (`make dev-up`) un entorno local con Postgres+pgvector, MinIO (S3 compatible), Adminer, MinIO Console y Mailpit
**Para** poder desarrollar y testear todas las HUs sin depender de servicios remotos ni de la integración real de S3/SMTP/Postgres en producción

## Criterios de aceptación

### Escenario 1: Levantar el stack completo con un comando

```gherkin
Dado que tengo Docker y Docker Compose v2 instalados
Y estoy en la raíz del repo
Cuando ejecuto `make dev-up`
Entonces docker compose levanta los servicios: postgres, minio, minio-init, adminer, mailpit
Y todos los servicios reportan healthcheck status "healthy" en menos de 60 segundos
Y el comando retorna exit code 0
```

### Escenario 2: Postgres expone pgvector y pgcrypto listas

```gherkin
Dado que el stack está corriendo
Cuando me conecto con `psql "$DOMAIN_DATABASE_URL"`
Y ejecuto `SELECT extname FROM pg_extension;`
Entonces el resultado incluye `vector`
Y el resultado incluye `pgcrypto`
Y la versión de Postgres reportada por `SELECT version();` es 16.x
```

### Escenario 3: MinIO arranca con bucket inicial creado

```gherkin
Dado que el stack está corriendo
Cuando consulto la API de MinIO con `mc ls local/`
Entonces existe el bucket `domain-assets`
Y la policy del bucket es `private`
Y las credenciales son las definidas en `.env` (DOMAIN_S3_ACCESS_KEY / DOMAIN_S3_SECRET_KEY)
```

### Escenario 4: Configuración por variables de entorno

```gherkin
Dado que existe `.env.example` en la raíz del repo
Y existe `.env` (copiado de `.env.example` por el desarrollador)
Cuando inspecciono `.env.example`
Entonces contiene `DOMAIN_DATABASE_URL` apuntando a `postgres://domain:domain@localhost:5432/domain?sslmode=disable`
Y contiene `DOMAIN_S3_ENDPOINT=http://localhost:9000`
Y contiene `DOMAIN_S3_BUCKET=domain-assets`
Y contiene `DOMAIN_S3_REGION=us-east-1`
Y contiene `DOMAIN_S3_ACCESS_KEY` y `DOMAIN_S3_SECRET_KEY` con valores de dev
Y contiene `DOMAIN_SMTP_HOST=localhost` y `DOMAIN_SMTP_PORT=1025`
Y `.env` está en `.gitignore`
```

### Escenario 5: Puertos expuestos solo en loopback

```gherkin
Dado que el stack está corriendo
Cuando reviso `docker compose ps`
Entonces los puertos publicados están bindeados a `127.0.0.1` y no a `0.0.0.0`:
  | servicio     | host:container          |
  | postgres     | 127.0.0.1:5432:5432    |
  | minio        | 127.0.0.1:9000:9000    |
  | minio-console| 127.0.0.1:9001:9001    |
  | adminer      | 127.0.0.1:8080:8080    |
  | mailpit-smtp | 127.0.0.1:1025:1025    |
  | mailpit-ui   | 127.0.0.1:8025:8025    |
```

### Escenario 6: Servicios accesibles desde la UI

```gherkin
Dado que el stack está corriendo
Cuando abro en el browser `http://localhost:8080`
Entonces veo la UI de Adminer y puedo loguearme contra postgres con las credenciales de `.env`

Cuando abro `http://localhost:9001`
Entonces veo la consola de MinIO y puedo loguearme con las credenciales de `.env`

Cuando abro `http://localhost:8025`
Entonces veo la UI de Mailpit
```

### Escenario 7: Persistencia entre reinicios

```gherkin
Dado que cargué datos en Postgres y un archivo en MinIO
Cuando ejecuto `make dev-down` (sin -v)
Y ejecuto `make dev-up` nuevamente
Entonces los datos de Postgres siguen presentes
Y el archivo de MinIO sigue presente
Porque los volúmenes nombrados (`domain_pg_data`, `domain_minio_data`) persisten
```

### Escenario 8: Reset completo del entorno

```gherkin
Dado que el stack está corriendo con datos
Cuando ejecuto `make dev-reset`
Entonces docker compose ejecuta `down -v --remove-orphans`
Y los volúmenes nombrados son eliminados
Y al volver a levantar con `make dev-up`, la base está vacía y MinIO solo tiene el bucket inicial vacío
```

### Escenario 9: Versiones fijadas (no `:latest`)

```gherkin
Dado que reviso `docker-compose.yml`
Cuando inspecciono las imágenes
Entonces ninguna usa tag `latest`
Y todas usan tag específico:
  | servicio  | imagen                                |
  | postgres  | pgvector/pgvector:pg16                |
  | minio     | minio/minio:RELEASE.2026-01-15T...    |
  | adminer   | adminer:4.8.1-standalone              |
  | mailpit   | axllent/mailpit:v1.20                 |
```

### Escenario 10: Migraciones aplicadas opcionalmente al levantar

```gherkin
Dado que el stack está corriendo
Cuando ejecuto `make dev-migrate`
Entonces se ejecuta `migrate -path migrations -database "$DOMAIN_DATABASE_URL" up`
Y las migraciones de issue-01.1 se aplican exitosamente
```

## Análisis breve

- **Qué pide realmente:** Entorno reproducible y aislado para desarrollo y CI local, con paridad de servicios (Postgres+pgvector, S3, SMTP) respecto a producción. Single-command bootstrap con buenas prácticas: versiones fijas, healthchecks, volúmenes nombrados, bind a loopback, secrets via `.env`.
- **Módulos sospechados:** `docker-compose.yml`, `Makefile`, `.env.example`, `.gitignore`, `scripts/postgres/init/`, `scripts/minio/init.sh`.
- **Riesgos / dependencias:** Requiere Docker Compose v2 instalado. Imagen `pgvector/pgvector` debe existir para el tag deseado. Puertos 5432/9000/9001/8080/1025/8025 deben estar libres en el host.
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Revisar codebase (grep) — ningún `docker-compose.yml` previo
- [ ] Revisar memorias engram (`domain_mem_search`)
- [ ] Confirmar imagen `pgvector/pgvector:pg16` disponible en Docker Hub
- [ ] Confirmar puertos libres en el host del desarrollador

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
