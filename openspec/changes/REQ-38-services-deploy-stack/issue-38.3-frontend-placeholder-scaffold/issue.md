# issue-38.3-frontend-placeholder-scaffold

**Origen:** `REQ-38-services-deploy-stack`
**Prioridad tentativa:** alta
**Tipo:** infrastructure / frontend
**Wave:** 1 (sin dependencias)

## Historia de usuario

**Como** operador del VPS
**Quiero** un container `domain-frontend` que sirva un `index.html` placeholder
("Domain dashboard — coming soon") mediante nginx, con su propio Dockerfile y
docker-compose.yml
**Para** poder validar el stack end-to-end (Caddy → frontend) antes de tener
código Angular real, y para que el día que Angular esté listo solo cambie el
contenido de `web/`.

## Criterios de aceptación

### Escenario 1: Dockerfile multi-stage minimal

```gherkin
Dado que existe domain-frontend/Dockerfile
Cuando hago build
Entonces usa nginx:1.27-alpine como runtime
Y copia domain-frontend/web/* a /usr/share/nginx/html
Y copia domain-frontend/nginx.conf a /etc/nginx/conf.d/default.conf
Y la imagen pesa <30 MB
Y EXPOSE 80
```

### Escenario 2: nginx.conf con SPA fallback

```gherkin
Dado que existe domain-frontend/nginx.conf
Cuando reviso la configuración
Entonces server listen 80
Y root /usr/share/nginx/html
Y location / { try_files $uri $uri/ /index.html; }
Y assets versionados tienen cache 1 año immutable
Y /index.html tiene Cache-Control "no-store, must-revalidate"
```

### Escenario 3: index.html placeholder funcional

```gherkin
Dado que existe domain-frontend/web/index.html
Cuando abro el archivo
Entonces contiene HTML válido
Y muestra el texto "Domain dashboard — coming soon"
Y tiene viewport meta para responsive
Y referencia "Powered by domain" como footer simple
Y NO carga JS/CSS externos (es 100% standalone)
```

### Escenario 4: docker-compose.yml referencia imagen GHCR

```gherkin
Dado que existe domain-frontend/docker-compose.yml
Cuando lo inspecciono
Entonces image: ghcr.io/nunezlagos/domain-frontend:${DOMAIN_FRONTEND_VERSION:-latest}
Y container_name: domain-frontend
Y restart: unless-stopped
Y expose: ["80"] (sin ports público)
Y network domain_internal external
Y healthcheck wget http://localhost/ -O /dev/null
```

### Escenario 5: Build local funciona

```gherkin
Dado que estoy en domain-frontend/
Cuando ejecuto `docker buildx build -t domain-frontend:dev --load .`
Entonces builda OK en <60s
Y `docker run --rm -p 8080:80 domain-frontend:dev` sirve el placeholder
Y curl http://localhost:8080/ devuelve "Domain dashboard — coming soon"
```

## Notas

- Este es **placeholder**, no Angular real. Cuando arranque REQ-FRONTEND
  (Angular scaffolding), reemplazará el contenido de `domain-frontend/web/`
  con el `dist/` build de Angular, manteniendo el Dockerfile/nginx.conf
  intactos.
- La estructura de archivos final de la HU:
  ```
  domain-frontend/
  ├── Dockerfile
  ├── docker-compose.yml
  ├── nginx.conf
  └── web/
      └── index.html
  ```
- El `.gitkeep` actual de `domain-frontend/` se elimina (ya no hace falta).
