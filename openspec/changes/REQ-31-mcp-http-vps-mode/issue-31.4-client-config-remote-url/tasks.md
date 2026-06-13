# Tasks: issue-31.4-client-config-remote-url

## Backend

- [ ] **T1**: Agregar `remoteURL` y `apiKey` a `installFlags` en
  `cmd/domain/install_cli.go:327`. Parsear en `parseInstallFlags`.

- [ ] **T2**: Validación de `remoteURL` al inicio de `runInstall`:
  - `u, err := url.Parse(flags.remoteURL)`.
  - Si err → abortar.
  - Si `u.Scheme == "https"` → OK.
  - Si `u.Scheme == "http" && (u.Hostname() == "localhost" ||
    u.Hostname() == "127.0.0.1")` → OK con warning.
  - Else → abortar con "remote-url must use https:// (got <scheme>)".
  - Normalizar: `flags.baseURL = u.Scheme + "://" + u.Host`
    (quitar path).

- [ ] **T3**: Helper `writeRemoteEnvFile(path, remoteURL string)
  error` en `internal/cli/install/remote_env.go`:
  - Lee el archivo, parsea líneas KEY=VAL.
  - Filtra: remueve `DOMAIN_DATABASE_URL`,
    `DOMAIN_DATABASE_AUTH_URL`, `DOMAIN_HTTP_PORT`.
  - Appende `DOMAIN_REMOTE_URL=<normalized>`,
    `DOMAIN_BASE_URL=<normalized>`.
  - Escribe atómico (write to temp + rename), chmod 600.

- [ ] **T4**: Branch condicional en `runInstall`:
  ```go
  if flags.remoteURL != "" {
      // skip steps 4-8, 11
      // step 7: usa writeRemoteEnvFile
      // step 9: si flags.apiKey != "", persist directo
  } else {
      // comportamiento actual
  }
  ```

- [ ] **T5**: `persistRemoteCredentials(apiKey, baseURL string)
  error`: helper que arma `onboard.Credentials{APIKey, BaseURL,
  IssuedAt: now}` y llama a `persistCredentials` existente.

- [ ] **T6**: Validación online best-effort: post persist, hacer
  `http.Get(baseURL + "/api/v1/auth/first-run")` con Bearer. Si
  200 → log "key validated". Si 401 → log warning "key present
  but server rejected it". Si timeout/error de red → log warning
  "could not reach server (will validate on first use)".

- [ ] **T7**: Comando standalone `domain setup remote [--url URL]
  [--api-key KEY] [--yes]` en `cmd/domain/setup.go`:
  - Mismo flow que los steps 7+9 de install pero sin el resto.
  - Útil para "ya tengo domain-mcp, solo quiero cambiar el server".

- [ ] **T8**: Actualizar `printInstallHelp` con los flags nuevos.

- [ ] **T9**: En modo `--remote-url`, el progress loggea claramente
  "REMOTE MODE — local infra skipped".

## Tests

- [ ] **T-unit-1**: `TestWriteRemoteEnvFile_RemovesDSN**` — env
  con `DOMAIN_DATABASE_URL=postgres://...` + `OTHER=value` →
  writeRemoteEnvFile con remote URL → el resultado NO tiene DSN
  Y mantiene `OTHER` Y tiene `DOMAIN_REMOTE_URL`.
- [ ] **T-unit-2**: `TestWriteRemoteEnvFile_PreservesUnrelated**` —
  env con `DOMAIN_OTEL_ENDPOINT=...` y otros → preserva todo
  excepto DSN.
- [ ] **T-unit-3**: `TestValidateRemoteURL_HTTPSOK**` —
  `https://api.foo.com` → OK.
- [ ] **T-unit-4**: `TestValidateRemoteURL_HTTPLocalhostOK**` —
  `http://localhost:8000` → OK con warning.
- [ ] **T-unit-5**: `TestValidateRemoteURL_HTTPExternalFails**` —
  `http://api.foo.com` → error.
- [ ] **T-unit-6**: `TestValidateRemoteURL_NormalizesPath**` —
  `https://api.foo.com/v1` → normalizado a
  `https://api.foo.com`.
- [ ] **T-e2e-1**: `TestRunInstall_RemoteMode_SkipsLocal**` — con
  `--remote-url` + `--api-key` + `--non-interactive` → mockear
  el HTTP client → asserta que step 4 (docker) y step 5
  (migrate) NO se ejecutaron (calls=0 a sus entry points).
- [ ] **T-e2e-2**: `TestRunInstall_RemoteMode_WritesEnv**` —
  `~/.config/domain/env` post-install tiene
  `DOMAIN_REMOTE_URL` y NO tiene `DOMAIN_DATABASE_URL`.
- [ ] **T-e2e-3**: `TestRunInstall_RemoteMode_Idempotent**` —
  correr 2 veces → la 2da detecta "already configured" y skip
  el UPSERT (no modifica el archivo si ya coincide).
- [ ] **T-sabotaje**: Comentar la lógica de UPSERT que REMUEVE DSN
  → test e2e-2 DEBE FALLAR (`DOMAIN_DATABASE_URL` sigue
  presente) → restaurar lógica → test verde. Documentar en
  commit body.
