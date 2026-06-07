# HU-27.3-hot-reload-config

**Origen:** `REQ-27-vertical-performance`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** SRE
**Quiero** cambiar log level, pool sizes, timeouts, feature flags sin restart
**Para** investigar issues prod sin causar window de unavailability

## Criterios de aceptación

### Escenario 1: Tabla runtime_config

```gherkin
Dado que existe tabla `runtime_configs` con (key, value JSONB, updated_at, updated_by)
Cuando admin POST /admin/runtime/configs/log_level con `{value:"debug"}`
Entonces persist + NOTIFY cache_invalidate_runtime_configs
Y todos los pods detectan + aplican
Y log info "runtime config changed: log_level=debug"
```

### Escenario 2: SIGHUP fallback

```gherkin
Dado que pod recibe SIGHUP
Cuando se procesa
Entonces re-lee runtime_configs desde DB
Y aplica cambios sin restart
Y log "config reload via SIGHUP"
```

### Escenario 3: Configs hot-reloadables (lista cerrada)

```gherkin
Dado que existen configs marcados `is_hot_reloadable: true`:
  | key                                | tipo  |
  | log_level                          | enum  |
  | http_request_timeout_seconds       | int   |
  | llm_default_timeout_seconds        | int   |
  | feature_flags.X                    | bool  |
  | otel_sample_ratio                  | float |
  | metrics_enabled                    | bool  |
Y configs `is_hot_reloadable: false`:
  | key                                | tipo  |
  | database_url                       | string|
  | s3_endpoint                        | string|
  | google_oauth_client_id             | string|
Cuando admin intenta cambiar non-reloadable
Entonces 409 "requires restart"
```

### Escenario 4: Validation

```gherkin
Dado que admin POST con `{value:"invalid"}` para log_level
Cuando se valida
Entonces 422 con error "log_level must be one of debug,info,warn,error"
Y NO se aplica
```

### Escenario 5: Audit log

```gherkin
Dado que se cambia config
Cuando se aplica
Entonces audit_log "runtime_config.changed" con key, old_value, new_value, who
```

### Escenario 6: GET current

```gherkin
Dado que GET /admin/runtime/configs
Cuando consulto
Entonces devuelve todos los configs actuales con `value`, `default_value`, `is_hot_reloadable`, `last_changed_at`, `last_changed_by`
```

## Análisis breve

- **Qué pide:** tabla runtime_configs + API admin + NOTIFY propagate + validators + audit
- **Esfuerzo:** S
- **Riesgos:** cambios no aplicables sin restart confunden → flag is_hot_reloadable explícito
