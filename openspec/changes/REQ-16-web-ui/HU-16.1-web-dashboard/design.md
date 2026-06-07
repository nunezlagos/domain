# Design: HU-16.1-web-dashboard

## Decisión arquitectónica

**SPA con React + Vite + TypeScript:** Es el stack frontend más popular y maduro. Vite da build rápido, React Query maneja caching y refetch automático, Tailwind permite diseño responsive rápido.

**Single dashboard endpoint:** En lugar de múltiples requests desde el frontend (que causarían waterfall), un único endpoint `GET /api/v1/dashboard` devuelve todo lo necesario. Server-side hace queries paralelas.

**Backend handler:**
```go
func (h *DashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
    g, ctx := errgroup.WithContext(r.Context())
    var stats DashboardStats
    var activity []ActivityEntry
    var costs CostSummary
    var status SystemStatus

    g.Go(func() error { return h.getStats(ctx, &stats) })
    g.Go(func() error { return h.getActivity(ctx, &activity) })
    g.Go(func() error { return h.getCosts(ctx, &costs) })
    g.Go(func() error { return h.getStatus(ctx, &status) })

    if err := g.Wait(); err != nil {
        respondError(w, err)
        return
    }
    respondJSON(w, DashboardResponse{stats, activity, costs, status})
}
```

**Layout:**
```
┌─────────────────────────────────────────────────────────┐
│  Navbar: Logo | Search | Notifications | Profile        │
├─────────┬───────────────────────────────────────────────┤
│ Sidebar │  Dashboard Content                             │
│ ─────── │  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐             │
│ Dashboard│  │Agents│ │Flows │ │Skills│ │Runs  │             │
│ Memories │  │  12  │ │   8  │ │  25  │ │ 143  │             │
│ Agents   │  └─────┘ └─────┘ └─────┘ └─────┘             │
│ Flows    │                                               │
│ Skills   │  ┌──────────────┐ ┌──────────────────┐       │
│ Cost     │  │ Recent Activity│ │ Cost Summary     │       │
│ Settings │  │ • Run flow X  │ │ Today: $12.50    │       │
│          │  │ • Agent Y    │ │ Month: $345.00   │       │
│          │  │ • Created ... │ │ [bar chart 7d]   │       │
│          │  └──────────────┘ └──────────────────┘       │
│          │                                               │
│          │  ┌────────────────────────────────────┐       │
│          │  │ Quick Actions                        │       │
│          │  │ [Create Agent] [Run Flow] [Memories] │       │
│          │  └────────────────────────────────────┘       │
└─────────┴───────────────────────────────────────────────┘
```

## TDD plan

Frontend TDD con Testing Library + Vitest:
1. **Red:** Test `DashboardPage` renders stat cards with mock data
2. **Green:** Implementar DashboardPage con props mock
3. **Refactor:** Conectar a API real con React Query
4. **Iterar:** ActivityFeed, CostSummary, StatusCards, responsive
5. **Sabotaje:** Stat card muestra NaN → test detecta

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Sidebar distrae del contenido principal | Sidebar colapsable en mobile, minimizada en desktop |
| Auto-refresh causa flickering | React Query mantiene datos anteriores mientras refetch |
| Dashboard sin datos útiles si recién instalado | Empty states con CTA a crear recursos |
