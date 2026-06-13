# Tasks: issue-35.4-runners-coverage-audit

## Backend

- [ ] **T1]: Crear `internal/admin/runners_usage/queries.go`:
  - `QueryAgentRuns(ctx, pool, days int) (AgentUsage, error)`.
  - `QueryFlowRuns(ctx, pool, days int) (FlowUsage, error)`.
  - `QuerySkillExecutions(ctx, pool, days int) (SkillUsage, error)`.
  - `QueryBySource(ctx, pool, days int) (map[string]int, error)`.
  - `QueryTopOrgs(ctx, pool, days int, limit int) ([]OrgUsage, error)`.
  - `QueryHighFailureAgents(ctx, pool, days int, limit int) ([]AgentFailRate, error)`.

- [ ] **T2]: Crear `internal/admin/runners_usage/categorize.go`:
  - `Categorize(total int, days int) string` retorna
    "USADO" / "POCO USADO" / "NUNCA USADO".
  - Threshold: `USADO if total >= max(10, days/3)`.
  - `POCO USADO if 1 <= total < threshold`.
  - `NUNCA USADO if total == 0`.

- [ ] **T3]: Crear `internal/admin/runners_usage/format.go`:
  - `FormatTable(report *Report) string`: tabla ASCII.
  - `FormatJSON(report *Report) ([]byte, error)`: JSON
    estructurado.
  - `WriteReport(report, dir string) error`: escribe
    `reports/runners-usage-<YYYYMMDD>.json`.

- [ ] **T4]: Crear `cmd/domain-admin/runners_usage.go`:
  - Parse args: `--days=30`, `--email=<addr>`, `--output=<dir>`.
  - Run queries en paralelo (`errgroup`).
  - Build report struct.
  - Categorize cada runner.
  - Print tabla a stdout.
  - Write JSON a file.
  - Si `--email`: send via mail.Mailer (issue 34.3).

- [ ] **T5]: Wire en `cmd/domain/main.go`:
  - Subcommand `admin runners-usage` o como comando separado
    `domain-admin` (decisión: comando separado, no requiere
    server corriendo).

- [ ] **T6]: Sanitización PII:
  - El reporte SOLO tiene UUIDs, no nombres ni emails.
  - Verificar con grep del output antes de mergear.
  - Si por bug se incluye nombre/email, falla con "PII detected,
    refusing to write report".

- [ ] **T7]: Cross-reference con cost_logs (issue 15.3):
  - Sumar el `cost_usd` total de cada runner en la ventana.
  - Include en el reporte: "agent_runner cost: $X, flow_runner
    cost: $Y".
  - Útil para el análisis de "vale la pena server-side vs
    delegar al MCP client".

## Tests

- [ ] `TestCategorize_Boundaries**` — total=0 → NUNCA,
  total=1 (days=30) → POCO, total=10 → USADO, total=15 →
  USADO. Edge cases de threshold.
- [ ] `TestCategorize_AdaptsToShortWindow**` — days=5, total=1 →
  USADO (porque threshold es max(10, 5/3) = 5).
- [ ] `TestQueryAgentRuns_CountsAndRates**` — DB con 100 agent
  runs (80 success, 20 failed) → query retorna total=100,
  success=80, failed=20, rate=0.8.
- [ ] `TestQueryTopOrgs_LimitApplied**` — DB con 15 orgs con
  runs → query limit=10 → retorna 10.
- [ ] `TestQueryHighFailureAgents**` — agent A con 10 runs (9
  failed) → retornado. Agent B con 10 runs (1 failed) → NO
  retornado (rate < 0.5).
- [ ] `TestFormat_NoPII**` — Report con datos de prueba →
  FormatJSON → grep el output buscando "@" (email) y palabras
  como "name" → NO debe encontrar.
- [ ] `TestInsufficientDataWarning**` — server con 3 días de
  data → el output incluye "insufficient data" warning.
- [ ] `TestReportFileWritten**` — comando corre → file
  `reports/runners-usage-20260612.json` existe con JSON
  válido.
- [ ] `T-sabotaje`: Hardcodear `return "USADO"` en
  `Categorize` (sabotaje: ignora el threshold) → test
  `TestCategorize_Boundaries` DEBE FALLAR (total=0 retorna
  USADO en vez de NUNCA) → restaurar lógica → test verde.
  Documentar en commit body.
