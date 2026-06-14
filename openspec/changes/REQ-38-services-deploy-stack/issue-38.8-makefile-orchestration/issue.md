# issue-38.8-makefile-orchestration

**Origen:** `REQ-38-services-deploy-stack`
**Prioridad tentativa:** alta
**Tipo:** infrastructure / build
**Wave:** 2 (depende de 38.2, 38.3, 38.4)

## Historia de usuario

**Como** operador del VPS
**Quiero** un `Makefile` que orqueste los 5 servicios (postgres, minio,
backend, frontend, caddy) con un único `make up` que crea la red interna
y los levanta en orden correcto
**Para** no tener que recordar 5 comandos `docker compose -f ...` por
servicio cada vez.

## Criterios de aceptación

### Escenario 1: ensure-network crea la red si no existe

```gherkin
Dado que `domain_internal` no existe en Docker
Cuando ejecuto `make ensure-network`
Entonces docker network create domain_internal se ejecuta
Y exit code 0
Y `docker network ls | grep domain_internal` muestra la red
```

### Escenario 2: ensure-network idempotente

```gherkin
Dado que `domain_internal` ya existe
Cuando ejecuto `make ensure-network` de nuevo
Entonces NO falla
Y NO recrea la red
Y exit code 0
```

### Escenario 3: make up levanta los 5 servicios en orden

```gherkin
Dado que estoy en /opt/services con .env válido
Cuando ejecuto `make up`
Entonces primero ejecuta `make ensure-network`
Y luego up de postgres (espera healthy)
Y luego up de minio (espera healthy)
Y luego up de domain-backend
Y luego up de domain-frontend
Y por último up de caddy
Y los 5 containers están en estado "running" después de <2 min
```

### Escenario 4: make up SVC=X levanta solo uno

```gherkin
Dado que paso SVC=postgres
Cuando ejecuto `make up SVC=postgres`
Entonces solo levanta postgres (con ensure-network primero)
Y SVC válidos: postgres|minio|backend|frontend|caddy|all
Y SVC inválido falla con mensaje claro listando opciones
```

### Escenario 5: make down detiene los 5

```gherkin
Dado que los 5 servicios están arriba
Cuando ejecuto `make down`
Entonces detiene caddy, frontend, backend, minio, postgres (orden inverso)
Y NO elimina volumes
Y NO elimina la network domain_internal
```

### Escenario 6: make ps muestra estado de los 5

```gherkin
Dado que ejecuto `make ps`
Cuando inspecciono el output
Entonces lista los 5 containers con su status
Y muestra puerto público solo de caddy (:80)
Y healthcheck status visible
```

### Escenario 7: make logs SVC=X tail de uno

```gherkin
Dado que paso SVC=caddy
Cuando ejecuto `make logs SVC=caddy`
Entonces tail -f de caddy logs
Y `make logs` sin SVC requiere especificar (no hace tail de todos por defecto)
```

### Escenario 8: make pull tira imágenes nuevas

```gherkin
Dado que .env apunta a DOMAIN_BACKEND_VERSION=v1.2.4 (nueva)
Cuando ejecuto `make pull`
Entonces docker compose pull para backend y frontend
Y NO pull de postgres/minio/caddy (imágenes pinneadas, no cambian)
```

### Escenario 9: make restart-service actualiza una sola pieza

```gherkin
Dado que actualicé DOMAIN_BACKEND_VERSION
Cuando ejecuto `make restart SVC=backend`
Entonces detiene container backend
Y pull nueva imagen
Y up backend
Y healthcheck pasa OK
Y los otros 4 servicios NO se reinician
```

### Escenario 10: make clean elimina volumes con confirmación

```gherkin
Dado que ejecuto `make clean`
Cuando se prompt confirmación
Entonces requiere escribir "borrar todo" para proceder
Y si confirma: detiene + elimina volumes de PG y MinIO
Y conserva network y caddy_data (innecesario perderlos)
```

## Notas

- El Makefile actual ya tiene la base (compose para PG y MinIO). Esta HU
  lo extiende a 5 servicios.
- Orden de up importa: PG/MinIO tienen healthcheck, backend debe esperar
  service_healthy de ambos. Pero como están en composes separados, el
  `depends_on` no aplica cross-compose. La solución es secuencial en make
  (espera con `docker compose up -d --wait` o loop manual).
- Usar `--env-file .env` en todos los compose commands para que cada uno
  lea las mismas vars sin duplicación.
