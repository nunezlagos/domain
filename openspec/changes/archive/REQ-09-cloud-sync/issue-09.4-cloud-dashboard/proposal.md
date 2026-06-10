# Proposal: issue-09.4-cloud-dashboard

## Intención

Proveer un dashboard web accesible desde el navegador para monitorear y administrar el cloud de memoria. Usa HTMX para interacciones dinámicas sin JS custom, Pico CSS para estilos minimalistas, y templ para components type-safe.

## Scope

**Incluye:**
- 5 rutas: /dashboard, /dashboard/stats, /browser, /projects, /admin
- Login/logout con sesiones
- templ components reutilizables (navbar, sidebar, cards, tables, pagination)
- Pico CSS (via CDN o bundle estático)
- HTMX (via CDN) para interacciones AJAX
- Protección de rutas con middleware de sesión

**No incluye:**
- Autosync background (issue-09.5)
- Audit log admin features (issue-09.6)
- Diseño responsive avanzado (Pico CSS ya es responsive)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Template engine | `templ` — components Go type-safe, codegen, sin reflection |
| CSS | Pico CSS (classless, minimal, responsive) |
| Interacciones | HTMX — hx-get, hx-post, hx-trigger, hx-target |
| Session auth | Session cookie + CSRF token; login valida contra ENGRAM_DASHBOARD_PASSWORD |
| Router | Compatible con chi o http.ServeMux; rutas agrupadas bajo /dashboard/ |

