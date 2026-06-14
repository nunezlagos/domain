# Tasks: issue-01.2-config-system

## Backend

- [x] Crear `internal/config/config.go` con struct Config y tags `env`
- [x] Implementar `Load() (*Config, error)` usando `caarlos0/env`
- [x] Implementar `Validate() error` con reglas de negocio
- [x] Implementar `String()` que oculte valores sensibles
- [x] Agregar `caarlos0/env` a `go.mod`
- [x] Llamar `config.Load()` en `main.go` con manejo de error (log.Fatal)
- [x] Pasar `*config.Config` a los componentes que lo necesiten

## Tests

- [x] Test unitario: carga completa
- [x] Test unitario: defaults
- [x] Test unitario: error en DATABASE_URL faltante
- [x] Test unitario: error en ENCRYPTION_KEY faltante/corta
- [x] Test unitario: error en PORT inválido
- [x] Test unitario: error en LOG_LEVEL inválido
- [x] Test unitario: errores compuestos
- [x] Test unitario: String() no expone la key
- [x] Sabotaje: eliminar validación → confirmar que falla → restaurar
- [x] Sabotaje: cambiar default de PORT a 0 → confirmar que falla → restaurar

## Cierre

- [x] Verificación manual con env vars inválidas
- [x] Suite verde
