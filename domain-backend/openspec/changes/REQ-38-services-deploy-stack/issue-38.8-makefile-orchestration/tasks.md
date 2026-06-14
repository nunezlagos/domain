# Tasks: issue-38.8-makefile-orchestration

## Variables y constantes

- [ ] **mk-001**: Sumar `NETWORK := domain_internal` al header.
- [ ] **mk-002**: Definir COMPOSE_PG, COMPOSE_MINIO, COMPOSE_BACKEND,
      COMPOSE_FRONTEND, COMPOSE_CADDY con sus paths + ENV_FILE.

## Targets nuevos

- [ ] **tg-001**: `ensure-network`: crea network si no existe (idempotente).
- [ ] **tg-002**: `pull`: docker compose pull para backend y frontend.

## Targets extendidos

- [ ] **tg-003**: `up`: depende de ensure-network. Case por SVC: postgres,
      minio, backend, frontend, caddy, all (orden secuencial).
- [ ] **tg-004**: `down`: detiene los 5 en orden inverso (caddy primero).
- [ ] **tg-005**: `restart`: case por SVC, `all` = down + up.
- [ ] **tg-006**: `ps`: itera por los 5 composes.
- [ ] **tg-007**: `logs`: requiere SVC, case por los 5 servicios.

## Targets preservados

- [ ] **tg-008**: `certs`: sin cambios (sigue ejecutando gen-certs.sh).
- [ ] **tg-009**: `certs-force`: igual.
- [ ] **tg-010**: `backup`: igual.
- [ ] **tg-011**: `healthcheck`: igual.
- [ ] **tg-012**: `psql`: confirmar usa COMPOSE_PG correctamente.
- [ ] **tg-013**: `mc`: confirmar usa `--network domain_internal` (la nueva
      red, antes era `minio_default`).
- [ ] **tg-014**: `clean`: confirmar bajar TODOS los composes con `-v`
      (PG y MinIO volumes; caddy_data y caddy_config opcional borrar).
- [ ] **tg-015**: `help`: sigue funcionando con grep de doc comments.

## Validación

- [ ] **test-001**: `make ensure-network` desde clean state → crea OK.
- [ ] **test-002**: `make ensure-network` 2ª vez → no-op.
- [ ] **test-003**: `make up SVC=postgres` levanta solo PG, --wait pasa.
- [ ] **test-004**: `make up SVC=invalid` falla con mensaje listando opciones.
- [ ] **test-005**: `make up` (sin SVC = all) levanta los 5 en orden,
      total ≤ 90s.
- [ ] **test-006**: `make ps` muestra 5 containers, todos "Up" + healthcheck.
- [ ] **test-007**: `docker network inspect domain_internal | jq
      '.[0].Containers | keys | length'` igual a 5.
- [ ] **test-008**: `make logs SVC=caddy` tail funcional.
- [ ] **test-009**: `make logs` sin SVC falla con mensaje.
- [ ] **test-010**: `make pull` ejecuta pull solo de backend y frontend.
- [ ] **test-011**: `make restart SVC=backend` recrea solo backend container.
- [ ] **test-012**: `make down` detiene los 5; `docker network ls | grep
      domain_internal` persiste.
- [ ] **test-013**: `make clean` con "borrar todo" elimina volúmenes de
      PG y MinIO.

## Edge cases

- [ ] **edge-001**: Si PG falla `--wait`: `make up` exits != 0, los demás
      no arrancan, `make ps` muestra solo postgres en estado weird.
- [ ] **edge-002**: Network ya existe con scope distinto:
      `docker network create domain_internal` falla con error claro.
      Resolución manual: `docker network rm domain_internal && make ensure-network`.

## Notas para reviewers

- SOLO se toca `Makefile` (raíz).
- Helpers (gen-certs.sh, backup.sh, healthcheck-alert.sh) NO se tocan.
- Targets de dev local (que existen en Makefile del repo `domain` rama main,
  como `dev-up`) NO entran acá. Esto es deploy VPS, no dev.
- Documentar en `## help` comments cada target para que `make help` los
  liste con descripción.
