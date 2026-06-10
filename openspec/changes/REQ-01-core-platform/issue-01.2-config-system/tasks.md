# Tasks: issue-01.2-config-system

## Backend

- [ ] Crear `internal/config/config.go` con struct Config y tags `env`
- [ ] Implementar `Load() (*Config, error)` usando `caarlos0/env`
- [ ] Implementar `Validate() error` con reglas de negocio
- [ ] Implementar `String()` que oculte valores sensibles
- [ ] Agregar `caarlos0/env` a `go.mod`
- [ ] Llamar `config.Load()` en `main.go` con manejo de error (log.Fatal)
- [ ] Pasar `*config.Config` a los componentes que lo necesiten

## Tests

- [ ] Test unitario: carga completa
- [ ] Test unitario: defaults
- [ ] Test unitario: error en DATABASE_URL faltante
- [ ] Test unitario: error en ENCRYPTION_KEY faltante/corta
- [ ] Test unitario: error en PORT inválido
- [ ] Test unitario: error en LOG_LEVEL inválido
- [ ] Test unitario: errores compuestos
- [ ] Test unitario: String() no expone la key
- [ ] Sabotaje: eliminar validación → confirmar que falla → restaurar
- [ ] Sabotaje: cambiar default de PORT a 0 → confirmar que falla → restaurar

## Cierre

- [ ] Verificación manual con env vars inválidas
- [ ] Suite verde
