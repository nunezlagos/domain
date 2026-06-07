# Proposal: HU-01.2-config-system

## Intención

Implementar un sistema de configuración tipado que se cargue desde variables de entorno, con validación en startup. El sistema debe ser explícito: toda variable requerida debe estar presente antes de que la aplicación procese requests. Los valores opcionales tienen defaults sensibles.

## Scope

**Incluye:**
- Struct `Config` en `internal/config/config.go`
- Carga desde environment variables con prefijo `DOMAIN_`
- Validación de tipos (int, string, enum)
- Error con todos los campos inválidos (no solo el primero)
- Defaults: PORT=3000, LOG_LEVEL=info
- Requeridos: DATABASE_URL, ENCRYPTION_KEY (mín 32 bytes)
- Función `Load() (*Config, error)` llamada en `main.go`
- Panic rápido si `Load()` falla

**No incluye:**
- Archivos `.env` (se deja a discreción del operador)
- Config desde archivos YAML/JSON
- Hot-reload de config
- Feature flags

## Enfoque técnico

1. Usar `caarlos0/env` v10 para parseo directo a struct con tags `env`
2. Validación post-parse manual para reglas de negocio (rango de puerto, longitud de key, log level enum)
3. Separación clara: parseo de tipos (env) + validación de negocio (manual)
4. Log de la configuración cargada (sin valores sensibles) al startup

```go
type Config struct {
    DatabaseURL    string `env:"DOMAIN_DATABASE_URL" envDefault:""`
    Port           int    `env:"DOMAIN_PORT" envDefault:"3000"`
    LogLevel       string `env:"DOMAIN_LOG_LEVEL" envDefault:"info"`
    EncryptionKey  string `env:"DOMAIN_ENCRYPTION_KEY" envDefault:""`
}
```

## Riesgos

- **Exposición de secrets en logs:** El EncryptionKey no debe loguearse. Mitigación: método `String()` que oculta valores sensibles.
- **Case sensitivity:** Las env vars en Unix son mayúsculas por convención. DOMAIN_* es el estándar.
- **Overrides de prueba:** Variables de entorno del test pueden contaminar. Mitigación: usar `env.LoadWithOptions` o limpiar en setup/teardown.

## Testing

- Test unitario con `t.Setenv()` para cada escenario
- Test de error compuesto (múltiples campos inválidos a la vez)
- Test de defaults
- Test de validación de puerto
- Test de validación de log level
- Verificar que `Config.String()` no expone la encryption key
