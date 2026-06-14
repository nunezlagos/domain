# Tasks: issue-16.1-web-dashboard

## Backend

- [x] Implementar endpoint GET /api/v1/dashboard con errgroup para queries paralelas
- [x] Implementar query: stats (count agents, flows, skills, runs today)
- [x] Implementar query: recent activity (last 20 entries from audit_log)
- [x] Implementar query: cost summary (today, month, avg daily, last 7 days)
- [x] Implementar query: system status (API health, DB ping, LLM providers)
- [x] Agregar índice en audit_log.created_at para activity feed rápido

## Frontend

- [x] Inicializar proyecto React + Vite + TypeScript + Tailwind
- [x] Configurar React Router, React Query
- [x] Implementar DashboardLayout (navbar + sidebar + content)
- [x] Implementar StatCard component con iconos
- [x] Implementar StatCardGrid (4 cards)
- [x] Implementar ActivityFeed con timestamps relativos
- [x] Implementar CostSummaryCard con minibarchart (Recharts)
- [x] Implementar QuickActions con botones navegables
- [x] Implementar SystemStatus con indicadores de color
- [x] Implementar auto-refresh cada 30s con useQuery refetchInterval
- [x] Implementar responsive design (mobile-first)
- [x] Implementar empty states para cada sección
- [x] Implementar error states con retry button
- [x] Pausar auto-refresh cuando window no está focused

## Tests

- [x] Test unitario: dashboard endpoint handler
- [x] Test unitario: StatCard renders correct props
- [x] Test unitario: ActivityFeed formato timestamp relativo
- [x] Test unitario: CostSummaryChart with data
- [x] Test de integración: dashboard carga y muestra datos
- [x] Test E2E: navegar a dashboard, verificar stats visibles
- [x] Test visual regression: dashboard layout
- [x] Sabotaje: API retorna error → dashboard muestra error state

## Cierre

- [x] Verificación manual: abrir /dashboard en browser
- [x] Suite backend: `go test ./internal/api/...`
- [x] Suite frontend: `npm run test`
- [x] Build: `npm run build` sin errores
