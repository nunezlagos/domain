# issue-38.5-env-example-extended

**Origen:** `REQ-38-services-deploy-stack`
**Prioridad tentativa:** media
**Tipo:** infrastructure / config
**Wave:** 1 (sin dependencias)

## Historia de usuario

**Como** operador que recibe la rama services por primera vez
**Quiero** un `.env.example` actualizado con las variables nuevas que
necesitan los 5 servicios (versionado de imágenes, puerto HTTP del backend,
secrets de DB y MinIO)
**Para** poder copiarlo a `.env`, rellenar passwords, y tener el stack
funcionando sin descubrir por error qué vars faltan.

## Criterios de aceptación

### Escenario 1: Variables de versionado de imágenes

```gherkin
Dado que abro .env.example
Cuando busco vars de versionado
Entonces existe DOMAIN_BACKEND_VERSION=latest
Y existe DOMAIN_FRONTEND_VERSION=latest
Y ambas tienen comentario "Pin a vX.Y.Z para producción"
```

### Escenario 2: Variables del backend

```gherkin
Dado que el backend necesita config
Cuando busco DOMAIN_*
Entonces existe DOMAIN_HTTP_PORT=8000
Y existe DOMAIN_DATABASE_URL (comentado, se compone de POSTGRES_* + app_user)
Y existe DOMAIN_S3_ENDPOINT (comentado, http://minio:9000)
Y NO se duplican passwords (siguen viniendo de POSTGRES_PASSWORD, APP_USER_PASSWORD, MINIO_ROOT_*)
```

### Escenario 3: Variables existentes preservadas

```gherkin
Dado que .env.example ya tenía vars de postgres + minio + backup + ntfy
Cuando reviso el archivo
Entonces TODAS las vars existentes siguen presentes con sus defaults
Y POSTGRES_DB, POSTGRES_USER, POSTGRES_PASSWORD están
Y APP_USER_PASSWORD, APP_ADMIN_PASSWORD están
Y MINIO_ROOT_USER, MINIO_ROOT_PASSWORD están
Y BACKUP_GPG_PASSPHRASE, BACKUP_DAILY_RETAIN están
Y NTFY_TOPIC, NTFY_SERVER están
```

### Escenario 4: Placeholders CHANGE_ME para secretos

```gherkin
Dado que el archivo se commitea al repo público
Cuando inspecciono valores
Entonces TODOS los secretos tienen valor CHANGE_ME (sin valor real)
Y existe comentario al inicio: "Copiar a .env. NUNCA committear .env."
Y existe comando recomendado: openssl rand -base64 48 | tr -d '/+=' | head -c 32
```

### Escenario 5: Comentarios mínimos pero accionables

```gherkin
Dado que el archivo busca ser autoexplicativo sin verbosidad
Cuando cuento comentarios
Entonces hay <= 1 línea de comentario por sección
Y NO hay headers ASCII art ni separadores ============
Y cualquier var no obvia tiene 1 línea explicando QUÉ es
```

## Notas

- El `.env.example` actual ya tiene buena base (audit limpia, sin secretos
  reales). Esta HU SUMA vars, no reescribe lo bueno.
- Variables que NO van en .env.example pero el backend espera (vienen del
  binario): DOMAIN_API_KEY del usuario admin inicial, DOMAIN_JWT_SECRET, etc.
  Se generan automáticamente en seed/install, no las ingresa el operador.
