# Tasks: issue-29.1-install-cwd-guard

> **Pre:** ninguno (chore local). Fix para evitar side effects fuera del repo.

## Backend
- [x] **T1**: `internal/cli/install/guards.go:IsProjectRoot(path)` retorna
  `(ok, missing, err)`. Markers: `.env.example` + `docker-compose.yml`.
- [x] **T2**: Flag `--src <path>` en `cmd/domain/install_cli.go:410` +
  parser en `parseInstallFlags` (líneas 479-484).
- [x] **T3**: Guard al inicio de `runInstall` (`install_cli.go:74`):
  `checkProjectRootGuard(flags.src)` antes de cualquier side effect.
  Si --src se pasó, hace `os.Chdir` al path absoluto resuelto.
- [x] **T4**: Mensajes de error ya cubren el caso (defense-in-depth
  en `ensureLocalEnvFile` + Abort en guard). No requiere cambio adicional.

## Tests
- [x] **T-unit-1**: `TestIsProjectRoot_OK` — ambos markers → (true, nil, nil).
  (guards_test.go:12)
- [x] **T-unit-2**: `TestIsProjectRoot_MissingOne` — solo .env.example →
  (false, ["docker-compose.yml"], nil). (guards_test.go:24)
- [x] **T-unit-3**: `TestIsProjectRoot_Empty` — dir vacío → (false,
  [".env.example", "docker-compose.yml"], nil). (guards_test.go:35)
- [x] **T-unit-4**: `TestIsProjectRoot_NotExist` — path inexistente →
  error real. (guards_test.go:45)
- [x] **T-e2e-1**: `TestCheckProjectRootGuard_FailsOutsideRepo` — tempdir
  vacío + `--src=<tempdir>` → ok=false + stderr contiene "no estás en
  el root del repo domain" + nombres de markers. (install_cwd_guard_test.go:49)
- [x] **T-e2e-2**: `TestCheckProjectRootGuard_OKInRepo` — cwd del repo
  real → ok=true sin requerir --src. (install_cwd_guard_test.go:17)
- [x] **T-e2e-3**: `TestCheckProjectRootGuard_SrcOverrideOK` — cwd=tempdir
  random + `--src=<repo válido>` → ok=true + cwd efectivo = --src.
  (install_cwd_guard_test.go:74)
- [x] **T-e2e-4**: `TestCheckProjectRootGuard_SrcNotExists` — --src apunta
  a path inexistente → ok=false + mensaje de error. (install_cwd_guard_test.go:99)
- [x] **T-e2e-5**: `TestCheckProjectRootGuard_OnlyOneMarker` — solo
  .env.example presente (sin docker-compose.yml) → ok=false + mensaje
  menciona docker-compose.yml. (install_cwd_guard_test.go:127)
- [x] **T-sabotaje**: documentado en los tests E2E: cualquier intento
  de comentar `if _, ok := checkProjectRootGuard(flags.src); !ok`
  en `runInstall` rompe los 3 tests E2E con `ok=false` y mensaje
  accionable ausente. La defensa es estructural: el guard es el
  PRIMER step, antes de backups/loadEnv/docker.

## Verificación final
- [x] **VF-1**: código commiteado (parte del refactor de repo 6270a78),
  tests escritos siguiendo patrón existing (`guards_test.go`).
- [x] **VF-2**: state.yaml → implemented (este commit).
- [x] **VF-3**: REQ-29: 29.1 → implemented.
