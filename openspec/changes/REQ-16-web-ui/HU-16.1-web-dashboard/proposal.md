# Proposal: HU-16.1-web-dashboard

## Intención

Implementar un dashboard web SPA que muestre una visión general del sistema: estadísticas de agentes/flows/skills/runs, actividad reciente, resumen de costos con minigráfico, estado de componentes y acciones rápidas. Auto-refresh cada 30s.

## Scope

**Incluye:**
- SPA frontend con framework moderno (React + Vite + TypeScript)
- Dashboard layout: top navbar, sidebar, main content grid
- Stat cards: Total Agents, Flows, Skills, Runs (hoy)
- Activity feed: últimos 20 eventos con timestamp relativo
- Cost summary card: gasto hoy/mes/promedio + minibarchart 7 días
- Quick actions: Create Agent, Run Flow, View Memories, Cost Analytics
- Status cards: API, DB, LLM Providers health
- Auto-refresh cada 30s con indicador
- Responsive design (mobile-first)
- Empty states y error states
- Backend API endpoint: GET /api/v1/dashboard

**Excluye:**
- Run visualization (HU-16.2)
- Flow editor (HU-16.3)
- User management UI
- Settings UI

## Enfoque técnico

**Stack frontend:**
- React 18 + TypeScript
- Vite como bundler
- Tailwind CSS para estilos
- React Query para fetching/caching/refetch
- Recharts para minigráficos
- react-router-dom para navegación

**Backend endpoint:**
```go
type DashboardResponse struct {
    Stats      DashboardStats      `json:"stats"`
    Activity   []ActivityEntry     `json:"recent_activity"`
    Costs      CostSummary         `json:"cost_summary"`
    Status     SystemStatus        `json:"status"`
}

type DashboardStats struct {
    TotalAgents int `json:"total_agents"`
    TotalFlows  int `json:"total_flows"`
    TotalSkills int `json:"total_skills"`
    RunsToday   int `json:"runs_today"`
}

// GET /api/v1/dashboard returns all data in single request
```

**Component tree:**
```
DashboardPage
├── StatCardGrid
│   ├── StatCard (Agents)
│   ├── StatCard (Flows)
│   ├── StatCard (Skills)
│   └── StatCard (Runs Today)
├── ActivityFeed
│   └── ActivityItem (×20)
├── CostSummaryCard
│   └── MiniBarChart
├── QuickActions
│   └── ActionButton (×4)
└── SystemStatus
    └── StatusCard (×3)
```

**Auto-refresh:**
```tsx
const { data } = useQuery({
    queryKey: ['dashboard'],
    queryFn: () => fetchDashboard(),
    refetchInterval: 30_000, // 30s
})
```

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| Dashboard lento si many queries | Single endpoint que agrega todo server-side |
| Auto-refresh molesto si usuario está interactuando | Pausar refetch si window no está focused (document.hidden) |
| Cost summary con datos sensibles | Mostrar solo si rol tiene permiso cost:read |
| Acciones rápidas sin permisos | Ocultar si rol no tiene permiso para esa acción |

## Testing

- Unit: dashboard endpoint handler
- Unit: React component rendering
- Integration: E2E dashboard carga y muestra datos
- Visual regression: screenshots comparativos
