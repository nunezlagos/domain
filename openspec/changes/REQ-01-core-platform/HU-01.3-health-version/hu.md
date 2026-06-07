# HU-01.3-health-version

**Origen:** `REQ-01-core-platform`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** operador de la plataforma
**Quiero** consultar un endpoint `GET /health` que devuelva estado, versión, uptime y conectividad a la base de datos
**Para** verificar rápidamente que el servicio está operativo y monitorizar su estado desde herramientas externas

## Criterios de aceptación

### Escenario 1: Health check exitoso con DB conectada

```gherkin
Dado que la aplicación está corriendo
Y la base de datos es accesible
Cuando hago una petición GET a `/health`
Entonces el código de respuesta es `200 OK`
Y el body JSON contiene:
  | status   | "ok"                    |
  | version  | "1.0.0"                 |
  | uptime   | un string en formato "1m2.5s" |
  | db_alive | true                    |
```

### Escenario 2: Health check con DB caída

```gherkin
Dado que la aplicación está corriendo
Pero la base de datos no está accesible
Cuando hago una petición GET a `/health`
Entonces el código de respuesta es `503 Service Unavailable`
Y el body JSON contiene:
  | status   | "degraded" |
  | db_alive | false      |
```

### Escenario 3: Versión embebida desde ldflags

```gherkin
Dado que el binario fue compilado con:
  go build -ldflags "-X domain/internal/version.Version=1.2.3"
Cuando hago una petición GET a `/health`
Entonces `version` en la respuesta es `"1.2.3"`

Dado que el binario fue compilado SIN ldflags
Cuando hago una petición GET a `/health`
Entonces `version` es `"dev"` (default)
```

### Escenario 4: `domain version` CLI

```gherkin
Dado que el binario `domain` está instalado
Cuando ejecuto `domain version`
Entonces el output contiene la versión compilada
Y el output contiene el commit SHA (si se compiló con ese ldflag)
Y el exit code es 0
```

## Análisis breve

- **Qué pide realmente:** Endpoint de health con status, versión, uptime, DB ping. Version info embebida vía ldflags. CLI command `domain version`.
- **Módulos sospechados:** `internal/api/`, `internal/version/`, `cmd/`
- **Riesgos / dependencias:** El health check depende del pool de conexiones de la DB. Si no hay DB configurada, debe responder degraded en lugar de crash.
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
