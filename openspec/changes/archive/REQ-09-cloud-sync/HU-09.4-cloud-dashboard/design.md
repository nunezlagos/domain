# Design: HU-09.4-cloud-dashboard

## Decisión arquitectónica

### Routes

```go
// Dashboard routes (protegidas con session middleware)
mux.Handle("GET /dashboard", sessionMiddleware(dashboardHandler))
mux.Handle("GET /dashboard/stats", sessionMiddleware(statsHandler))
mux.Handle("GET /browser", sessionMiddleware(browserHandler))
mux.Handle("GET /projects", sessionMiddleware(projectsHandler))
mux.Handle("GET /admin", sessionMiddleware(adminOnlyMiddleware(adminHandler)))

// Auth routes
mux.Handle("GET /login", loginPageHandler)
mux.Handle("POST /login", loginHandler)
mux.Handle("POST /logout", logoutHandler)
```

### templ components

```go
// Navbar
templ Navbar() {
    <nav class="container-fluid">
        <ul><li><strong>Domain Cloud</strong></li></ul>
        <ul>
            <li><a href="/dashboard">Dashboard</a></li>
            <li><a href="/browser">Browser</a></li>
            <li><a href="/projects">Projects</a></li>
            <li><a href="/admin">Admin</a></li>
            <li><a href="/logout">Logout</a></li>
        </ul>
    </nav>
}

// StatsCard
templ StatsCard(title string, value string, icon string) {
    <article class="stats-card">
        <header>
            <span class="icon">{ icon }</span>
            <h3>{ title }</h3>
        </header>
        <p class="value">{ value }</p>
    </article>
}

// DataTable (genérico, con slots HTMX)
templ DataTable(headers []string, rows [][]string, pagination Pagination) {
    <table>
        <thead>
            <tr>{ for _, h := range headers }<th>{ h }</th>{ end }</tr>
        </thead>
        <tbody>
            { for _, row := range rows }
            <tr>{ for _, cell := range row }<td>{ cell }</td>{ end }</tr>
            { end }
        </tbody>
    </table>
    if pagination.HasMore {
        <button hx-get={ pagination.NextURL } hx-target="#table-container" hx-swap="outerHTML">
            Load more
        </button>
    }
}
```

### Dashboard page layout

```
┌─────────────────────────────────────────────────┐
│  Navbar: Domain Cloud | Dashboard Browser ...  │
├─────────────────────────────────────────────────┤
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐          │
│  │ 142  │ │ 1.2k │ │  12  │ │ 5min │          │
│  │Syncs │ │Entries│ │Enroll │ │Last  │          │
│  └──────┘ └──────┘ └──────┘ └──────┘          │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │ Activity Chart (last 24h)              │   │
│  │ ██▄▄▄▄██▄▄▄▄██▄▄▄▄██  ▄▄▄▄           │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │ Recent Syncs (table, HTMX paginated)    │   │
│  └─────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

### Session auth

```go
// Simple session store (in-memory, suficiente para single-admin)
type SessionStore struct {
    mu       sync.RWMutex
    sessions map[string]*Session
}

type Session struct {
    ID        string
    User      string
    Role      string
    CreatedAt time.Time
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
    password := r.FormValue("password")
    if password != os.Getenv("ENGRAM_DASHBOARD_PASSWORD") {
        http.Error(w, "invalid credentials", 401)
        return
    }
    session := sessionStore.Create("admin", "admin")
    http.SetCookie(w, &http.Cookie{
        Name: "session_id", Value: session.ID,
        Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode,
    })
    http.Redirect(w, r, "/dashboard", http.StatusFound)
}
```

### HTMX patterns

| Pattern | Implementación |
|---------|---------------|
| Paginación | Botón "Load more" con `hx-get="/browser?page=2" hx-target="#entries-table" hx-swap="outerHTML"` |
| Filtros | Select con `hx-get="/browser?project=X" hx-target="#entries-table" hx-trigger="change"` |
| Live refresh | Meta tag `<meta http-equiv="refresh" content="30">` en /dashboard |
| Confirm actions | `hx-confirm="Are you sure?"` en acciones destructivas |

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| React/Vue/Svelte | Overkill; HTMX + templ es suficiente para dashboard interno; menos dependencias |
| Tailwind CSS | Pico CSS es classless, menos markup, más simple para un dashboard interno |
| Server-side templates con html/template | templ da type safety y compile-time checks; html/template es runtime reflection |

## TDD plan

1. **Red:** Dashboard page renderiza stats cards → falla
2. **Green:** Implement dashboardHandler + templ components → pasa
3. **Red:** Login redirect a dashboard → falla
4. **Green:** Implement login/logout handlers → pasa
5. **Red:** Admin page retorna 403 sin rol admin → falla
6. **Green:** Implement adminOnlyMiddleware → pasa
7. **Sabotaje:** Romper session middleware (permitir any request) → test redirect falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Sin sesión persistente (in-memory) | Aceptable para dashboard single-admin; restart pierde sesiones activas |
| CSRF | SameSite Strict en cookie; POST login/logout sin CSRF token es aceptable para uso interno |
| CDN outage (Pico CSS, htmx) | Bundlear assets estáticos en el binary via embed; fallback a CDN |
