# Tasks: issue-09.4-cloud-dashboard

## Backend

- [ ] **B1: Crear paquete `internal/cloud/dashboard/`**
      - `handlers.go` — dashboard, stats, browser, projects, admin handlers
      - `auth.go` — login/logout handlers, session middleware
      - `components/` — templ components

- [ ] **B2: Configurar templ codegen**
      - `//go:generate templ generate` en el paquete
      - Script en Makefile o task runner

- [ ] **B3: Implementar session store y middleware**
      - SessionStore in-memory con map + mutex
      - Session cookie (httpOnly, SameSite Strict)
      - Middleware que checkea cookie y pone user en context

- [ ] **B4: Implementar login/logout handlers**
      - GET /login → render login form (Pico CSS styled)
      - POST /login → validate ENGRAM_DASHBOARD_PASSWORD → create session → redirect
      - POST /logout → delete session → redirect to /login

- [ ] **B5: Implementar dashboardHandler**
      - Stats: total enrollments, total syncs, last sync timestamp, active projects
      - Render StatsCard templ components

- [ ] **B6: Implementar statsHandler**
      - Query sync activity por hora (últimas 24h)
      - Top projects por actividad
      - Render chart (HTML table o bar chart con CSS)

- [ ] **B7: Implementar browserHandler**
      - Lista paginada de cloud_sync_entries
      - Filtros: project, entity, operation
      - HTMX pagination (Load more)

- [ ] **B8: Implementar projectsHandler**
      - Lista de proyectos con entry count, last sync, status
      - Agrupar por project de cloud_sync_entries

- [ ] **B9: Implementar adminHandler + adminOnlyMiddleware**
      - Lista de enrollments con status
      - Botón para desactivar enrollment (POST /admin/deactivate/{id})
      - Middleware verifica role == "admin" en session

- [ ] **B10: Crear templ components reutilizables**
      - `Navbar.templ` — navegación con active state
      - `StatsCard.templ` — tarjeta con icono, título, valor
      - `DataTable.templ` — tabla genérica con headers y rows
      - `Pagination.templ` — botón "Load more" HTMX
      - `Sidebar.templ` — navegación lateral (opcional)
      - `Layout.templ` — wrapper con navbar + main + footer

- [ ] **B11: Integrar assets Pico CSS y htmx**
      - Download y embed via `//go:embed`
      - O usar CDN con fallback a embedded

## Tests

- [ ] **T1: Dashboard handler retorna HTML con stats**
- [ ] **T2: Login con password correcto crea sesión**
- [ ] **T3: Login con password incorrecto retorna 401**
- [ ] **T4: Sin sesión redirect a /login**
- [ ] **T5: Logout invalida sesión**
- [ ] **T6: Admin handler sin rol admin retorna 403**
- [ ] **T7: Browser handler retorna entries paginados**
- [ ] **T8: templ components renderizan sin error**
- [ ] **T9: Sabotaje — session middleware no checkea cookie → test T4 falla → restaurar**

## Cierre

- [ ] `templ generate` sin errores
- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cloud/dashboard/... -v`
- [ ] Commit: `feat: cloud dashboard with HTMX, Pico CSS, and templ components`
