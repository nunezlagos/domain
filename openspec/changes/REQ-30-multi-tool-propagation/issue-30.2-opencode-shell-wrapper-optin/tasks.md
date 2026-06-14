# Tasks: issue-30.2-opencode-shell-wrapper-optin

## Backend

- [ ] **T1**: Crear archivo `internal/cli/setup/wrapper.go` con:
  - `const WrapperMarkerOpen = "# >>> domain-wrapper >>>"`
  - `const WrapperMarkerClose = "# <<< domain-wrapper <<<"`
  - `func GenerateWrapperSnippet() string` — retorna el snippet canónico
    (opencode() + domain() functions, ambos invocan auto-detect).
  - `func InstallShellWrapper(rcfile string) (installed bool, err
    error)` — appendea el snippet al `rcfile` solo si NO contiene el
    marker open. Retorna `(true, nil)` si instaló, `(false, nil)` si
    ya estaba.
  - `func UninstallShellWrapper(rcfile string) error` — remueve el
    bloque entre markers. Si no existe, noop.
  - `func HasWrapper(rcfile string) (bool, error)` — helper para
    detectar presencia.

- [ ] **T2**: Agregar paso "Configure shell wrapper" al `runInstall`
  en `cmd/domain/install_cli.go`. Posición: entre step 10 (Configure
  agents) y step 11 (Init). Ofrece agregar wrapper solo si el
  install no es `--non-interactive` o si se pasa un flag explícito
  `--with-wrapper`.

- [ ] **T3**: Shell detection: leer `$SHELL`. Si termina en `/zsh` →
  target `~/.zshrc`. Si `/bash` → `~/.bashrc`. Si `fish` → mensaje
  "fish no soportado, snippet para zsh:" + imprimir snippet. Default
  si no detectable: zsh.

- [ ] **T4**: Comando standalone `domain setup wrapper-snippet
  [--shell zsh|bash]` que imprime el snippet sin tocar archivos. Útil
  para el user que prefiere agregar manual.

- [ ] **T5**: Sintaxis-check post-install del `rcfile`: para zsh
  `exec.Command("zsh", "-n", rcfile)`, para bash `bash -n`. Si falla,
  revertir el append y reportar error claro.

- [ ] **T6**: Integrar con manifest global (REQ-30.4): cuando se
  instala el wrapper, agregar entry al manifest global con
  `{type: "rcfile_append", path: "~/.zshrc", marker: "domain-wrapper"}`.

## Tests

- [ ] **T-unit-1**: `TestGenerateWrapperSnippet_HasMarkers**` — el
  snippet retornado contiene ambos markers.
- [ ] **T-unit-2**: `TestInstallShellWrapper_Fresh**` — rcfile vacío
  → Install appendea el snippet, retorna `(true, nil)`, el rcfile
  contiene el marker.
- [ ] **T-unit-3**: `TestInstallShellWrapper_Idempotent**` — rcfile
  con wrapper ya instalado → Install retorna `(false, nil)`, el
  rcfile no cambia (mismo número de líneas, mismo contenido).
- [ ] **T-unit-4**: `TestUninstallShellWrapper**` — rcfile con wrapper
  → Uninstall → el rcfile queda SIN el bloque entre markers; el
  resto del contenido se preserva.
- [ ] **T-e2e-1**: `TestRunInstall_OffersWrapper**` — `runInstall`
  interactivo (mock stdin con respuesta "y") → wrapper se agrega al
  rcfile correcto.
- [ ] **T-e2e-2**: `TestRunInstall_RejectsWrapper**` — `runInstall`
  interactivo (mock stdin con respuesta "N") → wrapper NO se agrega.
- [ ] **T-e2e-3**: `TestWrapperSnippet_ExecutesAutoDetect**` — tempdir
  con proyecto sin config + rcfile con wrapper + `bash -c "source
  rcfile && opencode --version 2>/dev/null || true"` → el
  `.domain/install-manifest.json` del proyecto EXISTE post-invocación.
- [ ] **T-sabotaje**: Comentar la línea `command domain setup
  auto-detect "$PWD" --quiet` en el snippet generado (modificar el
  output de `GenerateWrapperSnippet` para sabotaje) → installarlo →
  test e2e-3 DEBE FALLAR (no hay manifest post-opencode) → restaurar
  línea → test verde. Documentar en commit body.
