# Design: issue-01.2-config-system

## Decisión arquitectónica

**Parser:** `caarlos0/env` v10 — tags nativos, soporte para prefijos, cero dependencias pesadas.
**Validación:** Manual post-parse en `Validate() error`.
**Estructura:** `internal/config/config.go`.

La configuración es un singleton cargado una vez en `main.go` y propagado vía dependency injection.

## Alternativas descartadas

- **Viper:** Sobredimensionado para este caso. Soporta YAML/JSON/remote que no necesitamos.
- **envconfig (kelseyhightower):** Buena, pero `caarlos0/env` tiene mejor soporte de slices, tags, y es más activo.
- **Cobra viper pre-run hook:** Agrega complejidad innecesaria para cargar config.

## Diagrama

```
main()
  │
  └─→ config.Load()
        │
        ├─→ env.Parse(&cfg)        ← lee DOMAIN_* vars
        │
        └─→ cfg.Validate()          ← reglas de negocio
              │
              ├─→ Port 1-65535
              ├─→ LogLevel ∈ {debug, info, warn, error}
              ├─→ DatabaseURL != ""
              └─→ EncryptionKey ≥ 32 bytes
        │
        └─→ *Config, error

  └─→ Si error → log.Fatal (no arranca)
  └─→ Si ok → injectar en handlers/services
```

## Config struct

```go
type Config struct {
    DatabaseURL    string `env:"DOMAIN_DATABASE_URL"`
    Port           int    `env:"DOMAIN_PORT" envDefault:"3000"`
    LogLevel       string `env:"DOMAIN_LOG_LEVEL" envDefault:"info"`
    EncryptionKey  string `env:"DOMAIN_ENCRYPTION_KEY"`
}

func (c *Config) Validate() error {
    var errs error
    if c.DatabaseURL == "" { errs = errors.Join(errs, ...) }
    if c.Port < 1 || c.Port > 65535 { errs = errors.Join(errs, ...) }
    if !slices.Contains(validLogLevels, c.LogLevel) { errs = errors.Join(errs, ...) }
    if len(c.EncryptionKey) < 32 { errs = errors.Join(errs, ...) }
    return errs
}
```

## TDD plan

1. Test carga completa con todas las env vars
2. Test defaults cuando no se definen opcionales
3. Test error cuando falta DATABASE_URL
4. Test error cuando falta ENCRYPTION_KEY
5. Test error cuando PORT es inválido
6. Test error cuando LOG_LEVEL es inválido
7. Test error cuando ENCRYPTION_KEY es muy corta
8. Test errores compuestos (todo inválido a la vez)
9. Test que String() oculta EncryptionKey
10. Test que Load() panice si hay error (desde main)

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Exponer EncryptionKey en logs | Baja | Alto | Método String() que reemplaza con "****" |
| Env vars de CI faltantes | Media | Alto | Mensaje de error claro con variable faltante |
| Puerto tomado en dev | Baja | Bajo | Default 3000 es seguro para desarrollo |
