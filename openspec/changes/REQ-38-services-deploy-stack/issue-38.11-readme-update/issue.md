# issue-38.11-readme-update

**Origen:** `REQ-38-services-deploy-stack`
**Prioridad tentativa:** media
**Tipo:** documentation
**Wave:** 5 (depende de 38.8, 38.10)

## Historia de usuario

**Como** alguien que llega al repo por primera vez (operador, contributor)
**Quiero** un README de la rama services actualizado con la arquitectura
nueva (5 servicios + caddy + flujo de versionado de imágenes GHCR)
**Para** entender qué se deploya, cómo se opera día-a-día y cómo se publican
updates sin tener que leer issues ni código.

## Criterios de aceptación

### Escenario 1: Diagrama de arquitectura actualizado

```gherkin
Dado que abro README.md
Cuando veo el diagrama
Entonces muestra: INTERNET → Caddy:80 → {backend:8000 | frontend:80} → {postgres, minio}
Y indica que PG/MinIO son internas (no expuestas)
Y indica que es HTTP plano por IP, sin TLS
```

### Escenario 2: Sección Instalación actualizada

```gherkin
Dado que reviso "Instalación"
Cuando leo los comandos
Entonces el flujo es:
  git clone -b services <repo-url> /tmp/domain-services
  cd /tmp/domain-services
  ./install.sh
Y menciona que install pide contraseña sudo UNA vez
Y menciona que pulls imágenes desde GHCR (necesita internet en VPS)
```

### Escenario 3: Sección Layout describe las 5 carpetas

```gherkin
Dado que reviso "Layout"
Cuando inspecciono la estructura
Entonces lista las 5 carpetas de servicios: postgres, minio, domain-backend, domain-frontend, caddy
Y describe en 1 línea qué hace cada uno
Y menciona scripts/, systemd/, Makefile, install.sh, .env.example
```

### Escenario 4: Operación día-a-día con make

```gherkin
Dado que reviso "Operación"
Cuando leo los comandos
Entonces incluye:
  make up                    # ambos / todos
  make up SVC=postgres       # uno
  make ps
  make logs SVC=backend
  make restart SVC=backend
  make pull                  # tira imágenes nuevas
  make backup
  make clean                 # DESTRUCTIVO
```

### Escenario 5: Sección Update flow explica versionado

```gherkin
Dado que reviso "Update"
Cuando leo cómo deployar nueva versión
Entonces explica:
  1. git tag backend-vX.Y.Z + push  → CI publica imagen
  2. En VPS: edit .env (DOMAIN_BACKEND_VERSION=vX.Y.Z)
  3. make pull && make restart SVC=backend
  4. ~5s de downtime, sin tocar otros services
Y mismo flujo para frontend (frontend-vX.Y.Z)
```

### Escenario 6: URLs finales documentadas

```gherkin
Dado que reviso "Acceso"
Cuando leo qué URLs usar
Entonces lista:
  - Dashboard: http://<vps-ip>/
  - API:       http://<vps-ip>/api/v1/...
  - MCP:       http://<vps-ip>/mcp
Y menciona que PG y MinIO NO se acceden directo (solo via backend o ssh exec)
```

### Escenario 7: Backups y healthcheck mencionados

```gherkin
Dado que reviso "Backups" y "Healthcheck"
Cuando leo los párrafos
Entonces explica que backup corre diario 02:00 UTC (systemd timer)
Y retención = 2 backups (configurable en .env)
Y healthcheck cada 5min con notificación ntfy.sh
Y comando manual: make backup
Y restore: gpg -d ... | gunzip | docker exec -i domain-postgres psql ...
```

### Escenario 8: README sigue siendo CORTO

```gherkin
Dado que el principio de la rama services es "resumido"
Cuando cuento líneas
Entonces README.md tiene <= 100 líneas
Y NO tiene secciones FAQ extensas ni troubleshooting de 10 escenarios
Y NO duplica info que vive en docs/ del backend (sigue siendo "deploy infra")
```

## Notas

- El README actual ya está resumido (~60 líneas). Esta HU lo EXTIENDE para
  reflejar los 5 servicios + flujo de versionado, manteniendo brevedad.
- Sin diagramas ASCII enormes; tablas y listas son ok.
- NO mencionar dominio ni TLS (decisión cerrada del operador: solo IP).
