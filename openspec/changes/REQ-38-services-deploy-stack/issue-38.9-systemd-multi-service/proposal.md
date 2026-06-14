# Proposal: issue-38.9-systemd-multi-service

## Intención

Actualizar `systemd/domain-services.service` para que delegue al `Makefile`
(target `up`/`down`/`restart`) en lugar de invocar `docker compose` directo,
asegurando que el boot del VPS levante los 5 servicios correctamente.

## Scope

**Incluye:**
- Edición de `systemd/domain-services.service`:
  - `ExecStart=/usr/bin/make -C /opt/services up`
  - `ExecStop=/usr/bin/make -C /opt/services down`
  - `ExecReload=/usr/bin/make -C /opt/services restart`
  - `Type=oneshot`
  - `RemainAfterExit=yes`
  - `TimeoutStartSec=300` (suficiente para pull + up de 5 servicios)
  - `TimeoutStopSec=120`
  - `After=docker.service network-online.target`
  - `Requires=docker.service`
  - `Wants=network-online.target`
  - `WorkingDirectory=/opt/services`
  - `EnvironmentFile=/opt/services/.env`

**No incluye:**
- Cambios a backup.service / backup.timer (siguen igual).
- Cambios a healthcheck.service / healthcheck.timer (siguen igual).
- Auto-actualización de imágenes via timer (no se desea — el update es manual).

## Enfoque técnico

1. **Delegación al Makefile**: la lógica de orquestación vive en un solo lugar
   (Makefile). El unit systemd solo coordina cuándo arrancar/detener.
2. **`Type=oneshot` + `RemainAfterExit=yes`**: porque `make up` termina rápido
   (no es un daemon), pero queremos que systemd considere el service "active"
   mientras los containers corran.
3. **Sin auto-restart**: si `make up` falla, NO reintentamos (puede ser por
   imagen no encontrada, network problem, etc.). El healthcheck-alert.timer
   notifica al operador via ntfy.
4. **EnvironmentFile**: aunque el Makefile usa `--env-file .env`, systemd
   también necesita leer `.env` para resolver vars en el unit (si las hubiera).

## Riesgos

- **make no está en PATH durante boot**: en Ubuntu standard sí está.
  Mitigación: ruta absoluta `/usr/bin/make` (ya en el spec).
- **WorkingDirectory inexistente al boot**: si `/opt/services` no existe (ej.
  install.sh nunca corrió), unit falla. Mitigación: el install.sh garantiza
  que existe antes de habilitar el unit.
- **`make up` requiere docker daemon corriendo**: el `After=docker.service`
  + `Requires=docker.service` garantiza que docker esté arriba primero.
- **Timeout corto si pull es lento**: 300s para pull + up de 5 servicios en
  red lenta puede no alcanzar. Mitigación: el install.sh hace pull explícito
  ANTES, así que el `make up` en boot solo levanta containers (rápido).

## Testing

- `systemctl daemon-reload` después de editar el unit → exit 0
- `systemctl enable domain-services.service` → exit 0
- `systemctl start domain-services` → exit 0, los 5 containers arriba
- `systemctl status domain-services` muestra "active (exited)" con
  `RemainAfterExit`
- `systemctl stop domain-services` detiene los 5 containers, volumes persisten
- `journalctl -u domain-services --since "1 min ago"` muestra make output
- Simular reboot: `reboot` → al volver, los 5 containers están up sin
  intervención
- `systemctl restart domain-services` ejecuta make down + make up
