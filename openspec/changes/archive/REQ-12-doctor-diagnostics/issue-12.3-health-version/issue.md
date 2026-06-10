# issue-12.3-health-version

**Origen:** `REQ-12-doctor-diagnostics`
**Prioridad:** baja
**Tipo:** feature

## Historia de usuario

**Como** operador de infraestructura
**Quiero** un endpoint /health que retorne el estado del servicio
**Para** integrar memoria en sistemas de monitoreo (systemd, launchd, Prometheus)

**Como** usuario
**Quiero** saber la versión instalada de memoria con `engram version`
**Para** verificar actualizaciones y reportar bugs con el número correcto

## Criterios de aceptación

```gherkin
Scenario: GET /health retorna 200 con estado
  Given el servidor de memoria está corriendo
  When GET /health
  Then retorna 200
  And body incluye: {"status":"ok","version":"x.y.z","uptime":"...","db":"connected"}

Scenario: /health incluye timestamp y uptime
  Given el servidor corriendo hace 5 minutos
  When GET /health
  Then body incluye uptime ~= "5m"
  And incluye timestamp actual ISO8601

Scenario: /health retorna 503 si DB está desconectada
  Given la base de datos no responde
  When GET /health
  Then retorna 503
  And body incluye: {"status":"degraded","db":"disconnected"}

Scenario: /health es rápido (< 100ms)
  Given el servidor está saludable
  When GET /health
  Then responde en menos de 100ms

Scenario: /health no requiere autenticación
  Given un request sin token
  When GET /health
  Then retorna 200 con estado

Scenario: `engram version` mustra versión
  Given memoria está instalado
  When se ejecuta `engram version`
  Then output incluye: version, commit, build_date, go_version

Scenario: `engram version --json` output estructurado
  Given se ejecuta `engram version --json`
  When termina
  Then output es JSON válido con: version, commit, date, go_version, os, arch

Scenario: Version info se inyecta en build via ldflags
  Given se compila memoria
  When se verifica el binary
  Then version, commit, date se setean via -ldflags

Scenario: Service health es compatible con systemd
  Given systemd quiere monitorear memoria
  When ejecuta GET /health
  Then formato de respuesta es compatible con systemd healthcheck
  And exit code 0 si status=ok

Scenario: Service health es compatible con launchd
  Given launchd quiere monitorear memoria en macOS
  When ejecuta GET /health
  Then la respuesta es parseable por launchd
```

## Análisis breve

- **Qué pide realmente:** Endpoint /health con uptime, DB status, version info; comando `engram version` con ldflags injection; compatibilidad con systemd/launchd
- **Módulos sospechados:** `internal/api/health.go`, `internal/cli/version.go`, `internal/version/` package
- **Riesgos / dependencias:** Version injection requiere build system config; uptime tracking
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
- **Evidencia:** —
- **Acción derivada:** —
