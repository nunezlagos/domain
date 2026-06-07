# Tasks: HU-16.1-web-dashboard

## Backend

- [ ] Implementar endpoint GET /api/v1/dashboard con errgroup para queries paralelas
- [ ] Implementar query: stats (count agents, flows, skills, runs today)
- [ ] Implementar query: recent activity (last 20 entries from audit_log)
- [ ] Implementar query: cost summary (today, month, avg daily, last 7 days)
- [ ] Implementar query: system status (API health, DB ping, LLM providers)
- [ ] Agregar índice en audit_log.created_at para activity feed rápido

## Frontend

- [ ] Inicializar proyecto React + Vite + TypeScript + Tailwind
- [ ] Configurar React Router, React Query
- [ ] Implementar DashboardLayout (navbar + sidebar + content)
- [ ] Implementar StatCard component con iconos
- [ ] Implementar StatCardGrid (4 cards)
- [ ] Implementar ActivityFeed con timestamps relativos
- [ ] Implementar CostSummaryCard con minibarchart (Recharts)
- [ ] Implementar QuickActions con botones navegables
- [ ] Implementar SystemStatus con indicadores de color
- [ ] Implementar auto-refresh cada 30s con useQuery refetchInterval
- [ ] Implementar responsive design (mobile-first)
- [ ] Implementar empty states para cada sección
- [ ] Implementar error states con retry button
- [ ] Pausar auto-refresh cuando window no está focused

## Tests

- [ ] Test unitario: dashboard endpoint handler
- [ ] Test unitario: StatCard renders correct props
- [ ] Test unitario: ActivityFeed formato timestamp relativo
- [ ] Test unitario: CostSummaryChart with data
- [ ] Test de integración: dashboard carga y muestra datos
- [ ] Test E2E: navegar a dashboard, verificar stats visibles
- [ ] Test visual regression: dashboard layout
- [ ] Sabotaje: API retorna error → dashboard muestra error state

## Cierre

- [ ] Verificación manual: abrir /dashboard en browser
- [ ] Suite backend: `go test ./internal/api/...`
- [ ] Suite frontend: `npm run test`
- [ ] Build: `npm run build` sin errores
