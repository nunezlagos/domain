# Design: HU-09.1-cloud-config-auth

## Decisión arquitectónica

### CloudConfig struct y persistencia

```go
type CloudConfig struct {
    Server         string `json:"server,omitempty"`
    Token          string `json:"token,omitempty"`
    InsecureNoAuth bool   `json:"insecure_no_auth,omitempty"`
}
```

Resolve order:
1. `ENGRAM_CLOUD_TOKEN` env var → override token
2. `ENGRAM_CLOUD_INSECURE_NO_AUTH` env var → override insecure
3. `cloud.json` file → base config

### Config directory discovery

```go
func configDir() string {
    if d := os.Getenv("ENGRAM_CONFIG_DIR"); d != "" {
        return d
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".config", "engram")
}
```

### CLI command

```
engram cloud config [--server URL] [--token TOKEN] [--insecure-no-auth]
```

Sin flags → mustra config actual. Con flags → actualiza.

### Token sanitization

```go
func sanitizeToken(token string) string {
    if len(token) <= 8 {
        return "***"
    }
    return token[:4] + "..." + token[len(token)-4:]
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| YAML/TOML para cloud.json | JSON es más estándar, no requiere parser adicional |
| Keychain/secret service | Overkill para esta feature; puede agregarse después |
| Config en DB SQLite | Desacoplado del store; cloud.json puede existir sin DB |

## TDD plan

1. **Red:** Read config from cloud.json → falla
2. **Green:** Implement LoadCloudConfig → pasa
3. **Red:** ENV var overrides file → falla
4. **Green:** Implement resolve order → pasa
5. **Red:** Sanitize token in output → falla
6. **Green:** Implement sanitize → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Token en disco con permisos incorrectos | `os.WriteFile` con 0600; warn si permisos != 0600 |
| ENGRAM_CLOUD_INSECURE_NO_AUTH en producción | Log warning every request; no silenciar |
| Config directory no existe | Crear con `os.MkdirAll` al escribir |
