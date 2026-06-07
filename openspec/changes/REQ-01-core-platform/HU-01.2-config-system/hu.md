# HU-01.2-config-system

**Origen:** `REQ-01-core-platform`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** operador de la plataforma
**Quiero** configurar la aplicación mediante variables de entorno con validación al inicio
**Para** asegurarme de que el sistema arranca solo con parámetros válidos y no falla en runtime por configuraciones faltantes o inválidas

## Criterios de aceptación

### Escenario 1: Carga exitosa con todas las variables

```gherkin
Dado que las siguientes variables de entorno están definidas:
  | DOMAIN_DATABASE_URL  | postgres://user:pass@localhost:5432/domain?sslmode=disable |
  | DOMAIN_PORT          | 8080                                                      |
  | DOMAIN_LOG_LEVEL     | debug                                                     |
  | DOMAIN_ENCRYPTION_KEY | a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6                       |
Cuando inicio la aplicación
Entonces la configuración se carga sin errores
Y `Config.Port` es `8080`
Y `Config.LogLevel` es `debug`
Y `Config.DatabaseURL` es la URL provista
Y `Config.EncryptionKey` es la clave provista
```

### Escenario 2: Valores por defecto para opcionales

```gherkin
Dado que solo `DOMAIN_DATABASE_URL` y `DOMAIN_ENCRYPTION_KEY` están definidas
Cuando inicio la aplicación
Entonces `Config.Port` es `3000` (default)
Y `Config.LogLevel` es `info` (default)
```

### Escenario 3: Error si falta variable requerida

```gherkin
Dado que `DOMAIN_DATABASE_URL` NO está definida
Y `DOMAIN_ENCRYPTION_KEY` NO está definida
Cuando intento cargar la configuración
Entonces se produce un error de validación
Y el mensaje indica que `DOMAIN_DATABASE_URL` es requerida
Y el mensaje indica que `DOMAIN_ENCRYPTION_KEY` es requerida
Y la aplicación no arranca
```

### Escenario 4: Validación de puerto inválido

```gherkin
Dado que `DOMAIN_PORT` es `99999`
Cuando intento cargar la configuración
Entonces se produce un error indicando que el puerto está fuera de rango (1-65535)
```

### Escenario 5: Validación de log level inválido

```gherkin
Dado que `DOMAIN_LOG_LEVEL` es `superdebug`
Cuando intento cargar la configuración
Entonces se produce un error indicando que el log level no es válido
Y los valores aceptados son: debug, info, warn, error
```

### Escenario 6: Encryption key demasiado corta

```gherkin
Dado que `DOMAIN_ENCRYPTION_KEY` tiene menos de 32 bytes
Cuando intento cargar la configuración
Entonces se produce un error indicando que la clave debe tener al menos 32 bytes
```

## Análisis breve

- **Qué pide realmente:** Sistema de configuración basado en env vars con struct tipado, defaults, validación al startup y errores claros.
- **Módulos sospechados:** `internal/config/`, `cmd/`
- **Riesgos / dependencias:** Ninguno significativo. Librería recomendada: `caarlos0/env` o `kelseyhightower/envconfig`. Validación con `go-playground/validator`.
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
