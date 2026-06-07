# HU-09.4-cloud-dashboard

**Origen:** `REQ-09-cloud-sync`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria cloud
**Quiero** un dashboard web accesible desde el navegador
**Para** visualizar el estado de mi sincronización, estadísticas y proyectos

**Como** administrador
**Quiero** poder autenticarme en el dashboard con login/logout
**Para** acceder a vistas administrativas del cloud

## Criterios de aceptación

```gherkin
Scenario: Dashboard muestra estado general
  Given estoy autenticado en el dashboard
  When navego a GET /dashboard
  Then veo tarjetas con: total enrollments, syncs totales, última sync, proyectos activos

Scenario: /dashboard/stats muestra gráficas de actividad
  Given hay datos de sync en las últimas 24h
  When navego a GET /dashboard/stats
  Then veo stats de pushes/pulls por hora
  And veo top proyectos por actividad

Scenario: /browser permite navegar entries
  Given hay entries sincronizados
  When navego a GET /browser?project=my-app
  Then veo lista paginada de entries con filtros por proyecto, entidad, operación

Scenario: /projects lista proyectos
  Given hay múltiples proyectos con datos cloud
  When navego a GET /projects
  Then veo lista de proyectos con: nombre, entry count, última sync, status

Scenario: /admin requiere rol admin
  Given soy un usuario sin rol admin
  When navego a GET /admin
  Then veo 403 Forbidden

Scenario: /admin permite gestionar enrollments
  Given soy admin autenticado
  When navego a GET /admin
  Then veo lista de instancias enroladas con opción desactivar

Scenario: Login redirige a dashboard
  Given no estoy autenticado
  When navego a GET /dashboard
  Then soy redirigido a GET /login
  When ingreso credenciales válidas
  Then soy redirigido a /dashboard

Scenario: Logout invalida sesión
  Given estoy autenticado
  When hago POST /logout
  Then la sesión se invalida
  And soy redirigido a /login

Scenario: Pico CSS y htmx están presentes
  Given cargo cualquier página del dashboard
  Then el HTML incluye los links a Pico CSS y htmx
  And las interacciones (paginación, filtros) usan htmx

Scenario: templ components renderizan correctamente
  Given el servidor renderiza el dashboard
  When se genera el HTML
  Then los componentes templ (navbar, sidebar, cards) se renderizan sin errores
```

## Análisis breve

- **Qué pide realmente:** Dashboard web con HTMX + Pico CSS + templ components, 5 rutas protegidas, login/logout, vistas admin
- **Módulos sospechados:** `internal/cloud/dashboard/` — `handlers.go`, `templates/` con `.templ` files
- **Riesgos / dependencias:** templ codegen requiere `go generate`; HTMX requiere JS en cliente
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
