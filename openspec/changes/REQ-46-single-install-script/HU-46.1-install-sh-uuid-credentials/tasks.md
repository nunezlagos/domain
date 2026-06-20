# Tasks: HU-46.1-install-sh-uuid-credentials

## Implementación

- [ ] Crear `services/install.sh` con 8 pasos del proposal
- [ ] Implementar `gen_uuid()` basado en `/dev/urandom` (sin uuidgen/python)
- [ ] Implementar `env_get()` para parsear valores existentes
- [ ] Implementar preservación selectiva (regenera solo lo que falta)
- [ ] Implementar print final con bloque ASCII

## Cleanup

- [ ] Borrar `services/install-vps.sh` (reemplazado por install.sh)
- [ ] Borrar `scripts/deploy-vps.sh` (install corre en VPS, no en dev)
- [ ] Borrar `.env.vps` (credenciales viven en /opt/services/.env ahora)
- [ ] Actualizar `.gitignore` (sigue ignorando .env.vps por seguridad)

## Verificación en VPS

- [ ] Test 1: VPS con .env existente → reinstall → credenciales NO cambian
- [ ] Test 2: VPS con .env inexistente → install → credenciales nuevas impresas
- [ ] Test 3: Modificar credencial manualmente → reinstall → preserva el cambio
- [ ] Test 4: Containers quedan healthy post-install

## Cierre

- [ ] Commit dedicado: `feat(services): single install.sh with uuid credentials`
- [ ] Sin Co-Authored-By
- [ ] Push a GitHub
- [ ] Validar en VPS real