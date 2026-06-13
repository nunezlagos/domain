# Tasks: issue-29.1-install-cwd-guard

## Backend

- [ ] **T1**: Crear helper `install.IsProjectRoot(path string) (bool, []string, error)`
  en `internal/cli/install/guards.go` (nuevo archivo). Retorna
  `(true, nil, nil)` si `path/.env.example` y `path/docker-compose.yml`
  ambos existen; `(false, ["faltantes"], nil)` si faltan;
  `(false, nil, err)` si el path no es accesible. Función pura testeable
  con `os.Stat` (sin side effects).

- [ ] **T2**: Agregar flag `--src <path>` al struct `installFlags` en
  `cmd/domain/install_cli.go:327`. Parsear en `parseInstallFlags`.
  Validar formato: path absoluto o relativo que se resuelve al inicio
  de `runInstall` con `filepath.Abs`.

- [ ] **T3**: Insertar guard al INICIO de `runInstall`
  (`cmd/domain/install_cli.go:56`), ANTES de `loadEnvCascade` o
  cualquier side effect. Lógica:
  1. Resolver `projectRoot := flags.src` (si vacío, `os.Getwd()`).
  2. `ok, missing, err := install.IsProjectRoot(projectRoot)`.
  3. Si `err != nil` → log.Fatal-like con `progress.EndStep(StepFailed, ...)`
     + return 1.
  4. Si `!ok` → mensaje accionable con la lista de `missing` y exit 1.
  5. Si pasa → `os.Chdir(projectRoot)` para que el resto del flujo
     use paths relativos al project root efectivo. (Esto resuelve el
     problema de que el resto del código usa paths relativos al cwd.)

- [ ] **T4**: Actualizar mensajes de error de `ensureLocalEnvFile`
  (línea 456) para que NO se disparen si el guard pasó (debería ser
  imposible, pero defense-in-depth: mensaje menciona "guard should have
  caught this" si pasa).

## Tests

- [ ] **T-unit-1**: `TestIsProjectRoot_OK` — tempdir con ambos archivos
  → retorna `(true, nil, nil)`.
- [ ] **T-unit-2**: `TestIsProjectRoot_Missing` — tempdir con solo
  `.env.example` → retorna `(false, ["docker-compose.yml"], nil)`.
- [ ] **T-unit-3**: `TestIsProjectRoot_Empty` — tempdir vacío →
  retorna `(false, [".env.example", "docker-compose.yml"], nil)`.
- [ ] **T-unit-4**: `TestIsProjectRoot_NotExist**` — path inexistente →
  retorna error (no es "missing", es error real).
- [ ] **T-e2e-1**: `TestRunInstall_AbortsOutsideRepo` — `os.Chdir` a
  tempdir vacío + `runInstall([]string{"--non-interactive"})` → exit 1,
  stderr contiene "no estás en el root del repo domain",
  NO se creó `.env`, NO se creó `.bak.*`.
- [ ] **T-e2e-2**: `TestRunInstall_OKInsideRepo**` — cwd del repo real
  (donde estamos) + `runInstall([]string{"--non-interactive", "--no-service"})`
  → el guard pasa (no aborta por guard). No asserteamos más allá porque
  el install completo requiere infra.
- [ ] **T-e2e-3**: `TestRunInstall_SrcOverride**` — cwd=tempdir random
  + `--src=<repo real>` → guard pasa, resto del flujo trabaja contra
  ese path.
- [ ] **T-sabotaje**: Romper el guard (comentar la verificación de
  `IsProjectRoot` en `runInstall`) → correr `T-e2e-1` → DEBE fallar
  (exit 0 inesperado o mensaje ausente) → restaurar guard → test verde.
  Documentar el sabotaje en el commit body.
