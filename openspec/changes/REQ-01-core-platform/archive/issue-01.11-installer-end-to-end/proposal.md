# HU-01.11 — Proposal: install end-to-end con wizard

## Por qué

El binario `domain` actual (post-HU-01.10) requiere 5 pasos manuales
para tener Domain corriendo en una máquina nueva:

1. Clone del repo
2. `go build`
3. Copiar `.env.example` → `.env` a mano
4. Asegurar que Docker está corriendo (o configurar DSN cloud)
5. `domain install --mode local`

Eso es fricción. El contrato del usuario (basado en el feedback) es:

> "ejecutar el instalador y que decida todo adentro"

## Qué cambia

### Antes (post-HU-01.10)
```bash
git clone ... && cd domain
go build -o bin/domain ./cmd/domain
cp .env.example .env  # manual
docker compose up -d  # separado del install
./bin/domain install --mode local --non-interactive
```

### Después (post-HU-01.11)
```bash
curl -fsSL https://raw.githubusercontent.com/.../main/install.sh | bash
domain   # wizard decide todo
```

## Approach

1. **`install.sh` one-liner** (estilo `ptools`):
   - Detecta `git` + `go 1.22+`
   - Clona a `~/.local/share/domain` (idempotente: `git pull` si existe)
   - Compila con `go build` → `~/.local/bin/domain` o `$HOME/go/bin/domain`
   - Advierte si `$INSTALL_DIR` no está en PATH
   - Imprime: "Listo. Ejecuta: domain"

2. **`domain` sin args = wizard**:
   - Redirige a `runInstallWizard(false)` que pregunta 4 cosas
   - Después corre los 5 steps con `InstallProgress`
   - El user puede cancelar con Ctrl-C en cualquier prompt

3. **`domain install` sin `--mode` = mismo wizard**:
   - Equivalente a `domain` sin args
   - Mantiene `domain install --mode X` para CI/scripting

4. **Auto-`ensureLocalEnv`** en `install --mode local`:
   - Si `.env` falta Y `.env.example` existe, copia
   - Si Docker no está corriendo, falla claro
   - Es un step más del wizard con su propio progress

## Risks

| Risk | Mitigation |
|------|------------|
| `install.sh` rompe el PATH del user | NO modifica `.bashrc`/`.zshrc`. Solo warning |
| User tiene Go viejo | Chequeo de versión en `install.sh` con mensaje claro |
| Docker cuelga en `up` | `WaitHealthy` con 90s timeout (ya existe) |
| Re-correr install pisa configs | Backups automáticos timestamped |
| User quiere saltarse el wizard | `--non-interactive` flag mantiene el flow CI |

## Open questions

- **Q1:** ¿El binario debería auto-actualizarse al boot (como `ptools`)?
  - **Decisión:** NO. El binario es el core, no debe hacer `git pull` sin
    consentimiento. `install.sh` cubre el update.
- **Q2:** ¿Windows support?
  - **Decisión:** NO en este commit. Script falla claro si no es bash.
- **Q3:** ¿Agregar `domain update` con `git pull` + rebuild?
  - **Decisión:** NO. Es scope creep. El user re-corre `install.sh`.

## Success metrics

- Un user nuevo puede tener Domain corriendo en **<2 minutos** desde el clone
- `domain install` re-corrido N veces no rompe nada (idempotente)
- Cero pasos manuales fuera del wizard (después del install.sh)
