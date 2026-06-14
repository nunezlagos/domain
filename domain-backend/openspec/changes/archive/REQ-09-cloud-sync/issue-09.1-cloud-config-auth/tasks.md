# Tasks: issue-09.1-cloud-config-auth

## Backend

- [ ] **B1: Crear paquete `internal/cloud/`**
      - `config.go` — CloudConfig struct, load/save
      - `auth.go` — GetCloudToken, IsInsecureNoAuth

- [ ] **B2: Implementar config directory discovery**
      - `configDir()` función con ENGRAM_CONFIG_DIR > ~/.config/engram

- [ ] **B3: Implementar LoadCloudConfig() y SaveCloudConfig()**
      - Load: os.ReadFile + json.Unmarshal
      - Save: os.WriteFile con 0600 + json.MarshalIndent
      - Si archivo no existe, retorna config default (empty)

- [ ] **B4: Implementar getters con env var override**
      - `GetCloudServer()`: env > config.Server > ""
      - `GetCloudToken()`: env > config.Token > ""
      - `IsInsecureNoAuth()`: env > config.InsecureNoAuth > false

- [ ] **B5: Implementar `engram cloud config` CLI**
      - Flags: --server, --token, --insecure-no-auth
      - Sin flags: mustra config actual con token sanitizado
      - Con flags: actualiza y persiste

- [ ] **B6: Implementar token sanitization**
      - `SanitizeToken(token) string`

## Tests

- [ ] **T1: Save y Load cloud.json**
- [ ] **T2: ENV var overrides file token**
- [ ] **T3: InsecureNoAuth flag**
- [ ] **T4: Token sanitizado en output**
- [ ] **T5: Config sin flags mustra current config**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cloud/... -v`
- [ ] Commit: `feat: cloud config and auth with env var support`
