# Design: issue-38.9-systemd-multi-service

## Decisión arquitectónica

- **Delegar al Makefile**: el unit invoca `make up`/`make down`/`make restart`
  en lugar de duplicar la lógica de docker compose. Single source of truth.
- **`Type=oneshot` + `RemainAfterExit=yes`**: el comando `make` retorna
  rápido pero los containers siguen up; systemd considera el service activo.
- **Sin auto-restart de fallos**: si `make up` falla, healthcheck-alert
  notifica al operador via ntfy. Auto-restart oculta problemas.
- **EnvironmentFile preservado**: aunque Makefile usa `--env-file .env`,
  systemd también lee `.env` para resolver vars en el unit (si las hubiera).

## Alternativas descartadas

- **`Type=simple` con docker compose en foreground**: el compose --wait
  termina y el service queda dead. No funciona con simple.
- **Mantener `ExecStart=docker compose up`**: duplica lógica entre Makefile
  y systemd. Si Makefile cambia, systemd queda desincronizado.
- **`Restart=on-failure`**: peligroso. Si el problema es config malo (.env
  con CHANGE_ME), reintento infinito sin avisar.
- **`Restart=always`**: idem.
- **Múltiples units (uno por servicio)**: complejidad innecesaria.
  `docker-services.service` orquesta el conjunto via Makefile.

## Unit YAML final

```ini
[Unit]
Description=domain-services — orquestación de 5 servicios via docker compose
Documentation=https://github.com/nunezlagos/domain/tree/services
Requires=docker.service
After=docker.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/services
EnvironmentFile=/opt/services/.env
ExecStart=/usr/bin/make -C /opt/services up
ExecStop=/usr/bin/make -C /opt/services down
ExecReload=/usr/bin/make -C /opt/services restart SVC=all
TimeoutStartSec=300
TimeoutStopSec=120

[Install]
WantedBy=multi-user.target
```

## Por qué `-C /opt/services`

`make -C <dir>` cambia al directorio antes de ejecutar el target. Es
equivalente a `cd /opt/services && make ...` pero más limpio para systemd.

## Timeouts

- `TimeoutStartSec=300` (5 min): suficiente para `make up` con pull si
  hace falta. En boot normal sin pull, ~60s.
- `TimeoutStopSec=120` (2 min): docker compose down de 5 servicios típico
  10-30s.

## Backup y healthcheck timers (sin cambios)

```
domain-services-backup.timer       → OnCalendar=*-*-* 02:00:00
domain-services-backup.service     → ExecStart=/opt/services/scripts/backup.sh

domain-services-healthcheck.timer  → OnUnitActiveSec=5min
domain-services-healthcheck.service → ExecStart=/opt/services/scripts/healthcheck-alert.sh
```

Ambos siguen funcionando porque dependen de containers individuales
(`docker exec domain-postgres pg_dump`, `docker inspect domain-X`), no del
unit principal.
