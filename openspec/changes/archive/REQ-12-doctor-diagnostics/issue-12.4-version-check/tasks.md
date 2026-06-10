# Tasks: issue-12.4-version-check

## Backend

- [ ] **B1: Implementar VersionChecker con HTTP client**
      - `internal/version/check.go`
      - GitHubRelease struct, CheckResult struct
      - doCheck() con GET a GitHub API
      - Filtrar pre-releases

- [ ] **B2: Implementar cache con CheckFrequency**
      - `internal/version/check.go`
      - checkCache{result, checkedAt}
      - Check(ctx, force) con lógica de caché
      - Default CheckFrequency = 1h

- [ ] **B3: Modificar `engram version` para incluir update status**
      - `internal/cli/version.go`
      - Llamar VersionChecker.Check()
      - printUpdateStatus() con mensajes claros
      - Flag --check (force online check)

- [ ] **B4: Extender `--json` con update info**
      - `internal/cli/version.go`
      - Incluir CheckResult en JSON output

- [ ] **B5: Manejo de errores graceful**
      - Offline → "⚠ version check failed (offline)"
      - Rate limited → "⚠ version check failed (rate limited)"
      - API error → "⚠ version check failed (status X)"
      - Exit code 0 siempre (no blocking)

## Tests

- [ ] **T1: CheckResult.UpdateAvailable = false cuando misma versión**
- [ ] **T2: CheckResult.UpdateAvailable = true cuando tag_name ≠ Version**
- [ ] **T3: Cache retorna same result sin HTTP request (mocker servidor)**
- [ ] **T4: Cache expira después de CheckFrequency**
- [ ] **T5: Force=true ignora caché**
- [ ] **T6: Pre-release no se considera latest**
- [ ] **T7: HTTP error → CheckResult.Error no vacío**
- [ ] **T8: Timeout → offline error**
- [ ] **T9: printUpdateStatus output correcto para up-to-date**
- [ ] **T10: printUpdateStatus output correcto para update available**
- [ ] **T11: printUpdateStatus output correcto para offline**
- [ ] **T12: Sabotaje — no chequear Prerelease flag → pre-release como latest → test cae → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/version/... -v`
- [ ] `go test ./internal/cli/... -v`
- [ ] Commit: `feat: version check against GitHub releases with cache`
