# REQ-38 — Services Deploy Stack (rama services: 5 servicios dockerizados con Caddy)

> **Origen**: sesión 2026-06-13. Decisión arquitectónica: separar dashboard como servicio
> independiente (`domain-frontend` con nginx) del binario backend (`domain-backend`), con
> Caddy como reverse proxy único expuesto en :80 sobre IP del VPS (sin TLS, sin dominio).
> Postgres y MinIO quedan en red interna Docker, no expuestos públicamente. Toda la
> orquestación vive en la rama `services` del repo.

## Contexto

Estado actual de la rama `services` después de S01-S00 (commits 8eba383, 9d176c0, f96f984,
f9433b0, 336e714):

```
domain-services/
├── postgres/                ← compose listo, expone :5432 público (a refactorear)
├── minio/                   ← compose listo, expone :9000/:9001 público (a refactorear)
├── caddy/                   ← Caddyfile + compose creados (esqueleto)
├── domain-backend/          ← código copiado de main + Dockerfile heredado
├── domain-frontend/         ← solo .gitkeep
├── scripts/                 ← backup.sh, gen-certs.sh, healthcheck-alert.sh
├── systemd/                 ← 5 units (servicios + timers)
├── Makefile                 ← orquesta postgres + minio
├── install.sh               ← preflight + auto-sudo + levanta PG/MinIO
├── .env.example
└── README.md
```

Necesita pasar a:

```
VPS (un solo puerto público: 80)
  └── Caddy :80
        ├── /api/* → domain-backend:8000 (interno)
        ├── /mcp*  → domain-backend:8000
        └── /*     → domain-frontend:80
              ↓
        domain_internal network (PG + MinIO ocultos del público)
```

## Restricciones de diseño

1. **HTTP plano por IP**: no hay dominio. Caddy escucha `:80`, sin TLS, sin Let's
   Encrypt, sin sslip.io, sin Cloudflare Tunnel. Decisión cerrada del operador.
2. **Cada servicio en su carpeta**: postgres/, minio/, caddy/, domain-backend/,
   domain-frontend/ con `docker-compose.yml` independiente. Sin compose root.
3. **Red interna compartida**: network Docker `domain_internal` external, donde
   conviven los 5 servicios. Caddy es el único con `ports:` público.
4. **PG y MinIO no expuestos**: eliminar `ports:` público de postgres/ y minio/
   (los devs/admin acceden via `docker exec` o SSH tunnel ad-hoc).
5. **Imágenes versionadas en GHCR**: `ghcr.io/nunezlagos/domain-backend:vX.Y.Z`
   y `ghcr.io/nunezlagos/domain-frontend:vX.Y.Z`. Pin de versión en `.env`.
6. **Frontend con placeholder funcional**: el container de frontend debe levantar
   y servir un `index.html` mínimo aunque todavía no haya código Angular real,
   para validar el stack end-to-end.
7. **Trabajable en paralelo**: cada issue debe tocar archivos disjuntos para
   permitir branches simultáneas sin colisión.

## Issues

| Issue | Slug | Esfuerzo | Archivos tocados | Wave |
|-------|------|----------|------------------|------|
| 38.1 | `backend-dockerfile-refine` | S | `domain-backend/Dockerfile`, `domain-backend/.dockerignore` | 1 |
| 38.2 | `backend-compose-vps` | S | `domain-backend/docker-compose.yml` (rewrite) | 1 |
| 38.3 | `frontend-placeholder-scaffold` | M | `domain-frontend/{Dockerfile,docker-compose.yml,nginx.conf,web/index.html}` | 1 |
| 38.4 | `caddy-routing-refine` | S | `caddy/Caddyfile`, `caddy/docker-compose.yml` | 1 |
| 38.5 | `env-example-extended` | S | `.env.example` | 1 |
| 38.6 | `ci-build-backend` | M | `.github/workflows/build-backend.yml` | 2 |
| 38.7 | `ci-build-frontend` | S | `.github/workflows/build-frontend.yml` | 2 |
| 38.8 | `makefile-orchestration` | M | `Makefile` | 2 |
| 38.9 | `systemd-multi-service` | S | `systemd/domain-services.service` | 3 |
| 38.10 | `installer-vps-refactor` | L | `install.sh` | 4 |
| 38.11 | `readme-update` | M | `README.md` | 5 |

## Matriz de paralelismo

```
Wave 1 (cero colisión, 5 paralelos):
  38.1  38.2  38.3  38.4  38.5

Wave 2 (cuando Wave 1 termina, 3 paralelos):
  38.6  ← depende 38.1
  38.7  ← depende 38.3
  38.8  ← depende 38.2, 38.3, 38.4

Wave 3:
  38.9  ← depende 38.8

Wave 4:
  38.10 ← depende 38.6, 38.7, 38.8, 38.9

Wave 5:
  38.11 ← depende 38.8, 38.10
```

Cada wave puede trabajarse en branches separadas y mergearse a `services` sin
conflictos porque los archivos tocados son disjuntos dentro de la wave.

## Criterios de éxito globales

- `make up` levanta los 5 servicios en el VPS sin error.
- `curl http://<vps-ip>/healthz` devuelve 200 (Caddy → backend).
- `curl http://<vps-ip>/` devuelve HTML del frontend placeholder.
- `docker ps` muestra puerto público solo en `domain-caddy` (`:80`).
- `nc -zv <vps-ip> 5432` falla (PG no expuesto).
- `nc -zv <vps-ip> 9000` falla (MinIO no expuesto).
- `./install.sh` corre en VPS Ubuntu limpio y deja todo arriba en <5 min.
- Push de `git tag backend-v1.0.0` dispara CI que publica imagen en GHCR.
- Cada wave es mergeable a `services` sin conflictos.

## Prioridad: **alta** (bloqueante para deploy real del VPS)

Sin REQ-38, no se puede correr `domain` en el VPS de forma servible. Es
pre-requisito para REQ-31 (mcp-http-vps-mode) y REQ-32 (dashboard-readiness).
