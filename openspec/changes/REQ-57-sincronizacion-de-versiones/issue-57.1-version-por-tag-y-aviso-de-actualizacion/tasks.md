# Tasks: versión por tag y aviso de actualización

Seis fases entregables por separado. Las 0-1 dan el 80% del valor y son casi gratis.

## Fase 0 — Versión (habilitante, sin ella nada de lo demás es posible)

- [x] Test: un binario compilado con `-X` reporta su versión y su commit (REQ-1)
- [x] Test: un build local sin tag se identifica como desarrollo y NO finge ser release (REQ-1)
- [x] `install-user/Makefile`: agregar `-X` al `LDFLAGS`, que hoy es solo `-s -w`
- [x] `release-installer.yml`: pasar el tag como versión en el build de los 5 targets
- [x] Exponer versión del server + versión mínima de cliente en `domain_session_bootstrap` (REQ-2)
  — `min_client_version` va vacío ("sin piso declarado") hasta que la fase 4 lo defina

## Fase 1 — Aviso (el 80% del valor)

- [x] Test: comparación de versiones como **función pura** — mayor, igual, menor, y formatos raros
- [x] Test: cliente viejo → el bloque de inicio trae el aviso y el comando (REQ-3)
- [x] Test: cliente al día → sin línea de aviso (REQ-3)
- [x] Test: server caído → la sesión arranca igual, sin aviso y sin romper (REQ-3)
- [x] `hooks/domain-session-start.sh`: comparar y agregar la línea al bloque
  — la comparación NO se duplica en el hook: invoca `domain-install --version-check`

## Fase 2 — Distribución sin Git ni Go

- [x] El instalador baja el binario de Releases (anónimo, sin credenciales) (REQ-4)
  — `bootstrap.sh` baja de `/releases/latest/download` y verifica contra `SHA256SUMS.txt`
- [x] Conservar clone + compile como fallback para arquitecturas sin binario publicado
  — verificado EJECUTANDO el script con un `curl` que falla, no leyendo el texto
- [x] Test: la descarga no usa credenciales ni `git`

## Fase 3 — Actualización en un paso

- [ ] Comando de actualización: baja, verifica, reemplaza y reinstala hooks (REQ-4)
- [ ] Test: una actualización fallida deja el binario anterior funcionando (REQ-4)
- [ ] Test: ninguna ruta del hook descarga o reemplaza sin pedido explícito (REQ-4)

## Fase 4 — Piso de compatibilidad

- [ ] El server declara la versión mínima de cliente soportada (REQ-6)
- [ ] Test: por debajo del mínimo el aviso es explícito, y la sesión igual arranca (REQ-6)

## Fase 5 — Auto-deploy del VPS

- [ ] Script de verificación: último tag `v*` del remoto vs. SHA desplegado (REQ-5)
- [ ] Test: sin tag nuevo no se ejecuta deploy ni se reinicia nada (REQ-5)
- [ ] Test: commits en `main` sin tag NO despliegan (REQ-5)
- [ ] Guard de concurrencia: dos ciclos no se pisan (REQ-5)
- [ ] Instalación del cron en el VPS, documentada
- [ ] Verificar que el rollback de `scripts/deploy.sh` se dispara ante un tag que falla (REQ-5)

## Verify (auditoría — última task)

- [ ] Ninguna función nueva > 50 líneas (`go run ./cmd/size-lint`)
- [ ] El aviso NUNCA bloquea el arranque de sesión, en ninguna ruta
- [ ] La descarga del binario no requiere credenciales
- [ ] `go test ./... -count=1` verde; los tests de hooks con `-count=1` (leen archivos)
- [ ] Sabotajes: quitar el `-X` del build; hacer que el aviso bloquee; solapar dos ciclos del cron
- [ ] Un deploy real por tag de punta a punta, verificado contra el VPS

## Documentación

- [ ] CHANGELOG Unreleased
- [ ] `state.yaml` a `implemented`
- [ ] Documentar el flujo de release: qué pasa al pushear un tag
