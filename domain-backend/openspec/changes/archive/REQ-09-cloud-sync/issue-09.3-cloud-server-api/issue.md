# issue-09.3-cloud-server-api

**Origen:** `REQ-09-cloud-sync`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** administrador del servidor cloud
**Quiero** ejecutar `engram cloud serve` para iniciar el servidor de sincronización con Postgres
**Para** que los clientes puedan push/pull sus memorias

**Como** operador
**Quiero** configurar ENGRAM_DATABASE_URL, ENGRAM_JWT_SECRET y ENGRAM_CLOUD_ALLOWED_PROJECTS
**Para** asegurar y limitar el acceso al servidor

## Criterios de aceptación

```gherkin
Scenario: Cloud serve inicia servidor HTTP
  Given ENGRAM_DATABASE_URL y ENGRAM_JWT_SECRET están configurados
  When se ejecuta `engram cloud serve`
  Then el servidor escucha en puerto configurado (default 8080)
  And responde a GET /health

Scenario: Push endpoint acepta chunks de observaciones
  Given un cliente autenticado con JWT válido
  When POST /api/sync/push con body de chunks de observaciones
  Then el servidor persiste los chunks en Postgres
  And retorna acknowledgment con server_timestamp

Scenario: Pull endpoint retorna cambios desde último sync
  Given un cliente con last_sync_timestamp
  When GET /api/sync/pull?since=2026-06-01T00:00:00Z
  Then retorna observaciones modificadas desde ese timestamp
  And incluye server_timestamp actual

Scenario: Mutations endpoint sincroniza operaciones MUT
  Given un cliente con operaciones MUT (Merge, Update, Tag)
  When POST /api/sync/mutations con lista de operaciones
  Then el servidor aplica las mutaciones en orden
  And retorna resultados por operación

Scenario: JWT inválido rechaza request
  Given un request con JWT expirado o inválido
  When se llama cualquier endpoint protegido
  Then retorna 401 Unauthorized
  And no procesa la operación

Scenario: Allowed projects filter limita acceso
  Given ENGRAM_CLOUD_ALLOWED_PROJECTS=project-a,project-b
  When un cliente intenta push de project-c
  Then retorna 403 Forbidden "project not allowed"

Scenario: Postgres schema se migra al iniciar
  Given Postgres está configurado
  When se ejecuta `engram cloud serve`
  Then las tablas cloud_sync_entries, enrollments, audit_log se crean automáticamente

Scenario: Health endpoint retorna status de Postgres
  Given servidor cloud corriendo
  When GET /health
  Then retorna {"status":"ok","database":"connected","version":"x.y.z"}
```

## Análisis breve

- **Qué pide realmente:** Servidor HTTP con Postgres backend, endpoints sync push/pull/mutations, JWT auth, ENGRAM_CLOUD_ALLOWED_PROJECTS filter, auto-migración de schema
- **Módulos sospechados:** `internal/cloud/server/` - `serve.go`, `handlers.go`, `auth.go`, `migrations.go`
- **Riesgos / dependencias:** Dependencia externa `lib/pq` o `pgx`, migraciones de Postgres, JWT signing
- **Esfuerzo tentativo:** L

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
