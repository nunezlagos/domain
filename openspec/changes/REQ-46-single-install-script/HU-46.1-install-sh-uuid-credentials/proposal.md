# Proposal: HU-46.1-install-sh-uuid-credentials

## Intención

Unificar el bootstrap del stack en un solo `services/install.sh` que:
- Funciona en VPS vacío o con servicios preexistentes
- Genera credenciales UUID v4 seguras automáticamente
- Preserva credenciales existentes en reinstall (idempotente)
- Imprime todas las credenciales al final
- Reemplaza `install-vps.sh`, `scripts/deploy-vps.sh`, `.env.vps`

## Scope

**Crea:**
- `services/install.sh` — único entrypoint

**Borra:**
- `services/install-vps.sh` — reemplazado por install.sh
- `scripts/deploy-vps.sh` — ya no aplica (install corre en el VPS directamente)
- `.env.vps` — credenciales viven en `/opt/services/.env` ahora

**Mantiene:**
- `services/setup-vm.sh` — VM/libvirt setup, distinto al VPS install
- `services/test-vps-*.sh` — tests, no installers
- `.env.example` — template

## Enfoque técnico

### Generación de credenciales
- UUID v4 vía `/dev/urandom` (sin dependencia externa):
  ```bash
  gen_uuid() {
    od -An -N16 -tx1 /dev/urandom | tr -d ' \n' | head -c 32 | \
      sed -E 's/^(.{8})(.{4})(.{4})(.{4})(.{12}).*$/\1-\2-\3-\4-\5/'
  }
  ```
- 122 bits de entropía, formato humanamente reconocible

### Preservación en reinstall
- Si `/opt/services/.env` existe, parsear valores existentes con `grep + cut`
- Si una variable está vacía/inexistente, generar nueva (fill-in)
- Si `.env` no existe, generar todo desde cero

### Flujo del script (8 pasos)
1. Validar Ubuntu + systemd + arch
2. Verificar/instalar Docker
3. Clone o pull del repo
4. Generar/preservar `.env` con UUIDs
5. Generar certs autofirmados (postgres + minio)
6. Build + up de docker compose
7. Configurar systemd timers (backup, healthcheck)
8. Print de credenciales a stdout

### Print final
Bloque ASCII con:
- URL del dashboard
- URL del API/MCP
- Email del admin
- Todas las contraseñas
- Path del .env
- Nota sobre rotación

## Riesgos

| Riesgo | Mitigación |
|---|---|
| UUIDs chocan con entropy insuficiente | 122 bits, prácticamente imposible |
| Install falla a mitad con credenciales parcialmente generadas | Re-correr install preserva las generadas |
| Operador pierde credenciales si no las guarda | Print es muy visible; también quedan en .env |
| Reinstall con `.env` corrupto | Script valida que .env sea legible antes de proceder |
| `.env` con permisos incorrectos | `chmod 600` siempre |

## Testing

- [ ] VPS vacío → install.sh → containers healthy + creds impresas
- [ ] Re-correr install.sh → credenciales NO cambian
- [ ] Modificar manualmente una credencial → reinstall → preserva la modificada
- [ ] Borrar una credencial del .env → reinstall → regenera SOLO esa
- [ ] Sabotaje: hacer fallar el docker compose → script debe reportar error claro