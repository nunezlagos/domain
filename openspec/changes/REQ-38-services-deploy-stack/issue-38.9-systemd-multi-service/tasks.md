# Tasks: issue-38.9-systemd-multi-service

## Edición del unit

- [ ] **sys-001**: Cambiar `ExecStart=/usr/bin/docker compose up -d` a
      `ExecStart=/usr/bin/make -C /opt/services up`.
- [ ] **sys-002**: Cambiar `ExecStop=/usr/bin/docker compose down` a
      `ExecStop=/usr/bin/make -C /opt/services down`.
- [ ] **sys-003**: Cambiar `ExecReload=/usr/bin/docker compose restart` a
      `ExecReload=/usr/bin/make -C /opt/services restart SVC=all`.
- [ ] **sys-004**: Confirmar `Type=oneshot` + `RemainAfterExit=yes`.
- [ ] **sys-005**: Confirmar `WorkingDirectory=/opt/services`.
- [ ] **sys-006**: Confirmar `EnvironmentFile=/opt/services/.env`.
- [ ] **sys-007**: Confirmar `After=docker.service network-online.target`
      y `Requires=docker.service`.
- [ ] **sys-008**: `TimeoutStartSec=300`, `TimeoutStopSec=120`.

## Backup/healthcheck units (sin cambios)

- [ ] **sys-009**: Confirmar que `domain-services-backup.service` no se toca.
- [ ] **sys-010**: Confirmar que `domain-services-backup.timer` no se toca.
- [ ] **sys-011**: Confirmar que `domain-services-healthcheck.service` no se toca.
- [ ] **sys-012**: Confirmar que `domain-services-healthcheck.timer` no se toca.

## Validación

- [ ] **test-001**: Copiar el unit nuevo a `/etc/systemd/system/`, ejecutar
      `systemctl daemon-reload` → exit 0.
- [ ] **test-002**: `systemctl enable domain-services.service` → exit 0.
- [ ] **test-003**: `systemctl start domain-services` → exit 0.
- [ ] **test-004**: `systemctl status domain-services` muestra "active (exited)".
- [ ] **test-005**: `docker ps --filter name=^domain-` muestra los 5 containers
      Up.
- [ ] **test-006**: `journalctl -u domain-services --since "1 min ago"` muestra
      output de make (ensure-network, up de los 5 servicios).
- [ ] **test-007**: `systemctl stop domain-services` → exit 0, los 5
      containers detienen.
- [ ] **test-008**: `systemctl restart domain-services` ejecuta make down + up.
- [ ] **test-009**: Reboot VPS (`sudo reboot`) → al volver, los 5 containers
      están up automáticamente.

## Edge cases

- [ ] **edge-001**: Si `.env` tiene CHANGE_ME: `make up` falla, unit reporta
      "failed", `journalctl` muestra el error claro.
- [ ] **edge-002**: Si Docker daemon está caído: unit espera `docker.service`
      por Requires; si docker NO arranca, unit no se ejecuta.
- [ ] **edge-003**: Si `/opt/services` no existe: unit falla rápido con error
      "WorkingDirectory not found".
- [ ] **edge-004**: Si `make` no está en PATH: usar ruta absoluta
      `/usr/bin/make` evita el problema (ya está en spec).

## Notas para reviewers

- SOLO se edita `systemd/domain-services.service`.
- Los otros 4 units (backup + healthcheck × {service, timer}) NO se tocan.
- install.sh ya los copia a `/etc/systemd/system/` (HU-38.10).
- Test crítico: reboot del VPS y validar que los 5 containers están up.
