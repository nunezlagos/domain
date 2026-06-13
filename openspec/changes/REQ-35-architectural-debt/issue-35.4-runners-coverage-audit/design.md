# Design: issue-35.4-runners-coverage-audit

## Contexto

El código de domain tiene 3 runners server-side: agent, flow,
skill (este último parcialmente implementado, ver 35.2). Mantener
cada uno cuesta tiempo: tests, bugs, features, docs. Si NUNCA
se usan, es dead weight. Si se usan poco, hay que decidir si
vale la pena.

El método científico: recolectar datos reales de uso, analizar,
tomar decisión basada en evidencia.

## Decisión arquitectónica

**Estrategia:** comando standalone que query la DB y genera
reporte JSON + tabla ASCII. Idempotente, sin side effects.

1. **Comando: `domain admin runners-usage [--days=30]`:**
   - `--days`: ventana de análisis (default 30, max 365).
   - Solo lectura: NUNCA modifica estado.
   - Auth: requiere `DOMAIN_DATABASE_AUTH_URL` (admin-level).

2. **Queries (corren en paralelo):**
   ```sql
   -- agent_runs
   SELECT
     COUNT(*) AS total,
     COUNT(*) FILTER (WHERE status = 'succeeded') AS succeeded,
     COUNT(*) FILTER (WHERE status = 'failed') AS failed,
     AVG(EXTRACT(EPOCH FROM (finished_at - started_at))) AS avg_duration_sec
   FROM agent_runs
   WHERE started_at >= NOW() - INTERVAL '$1 days';

   -- top 5 agents
   SELECT agent_id, COUNT(*) AS n
   FROM agent_runs
   WHERE started_at >= NOW() - INTERVAL '$1 days'
   GROUP BY agent_id
   ORDER BY n DESC LIMIT 5;

   -- flow_runs (similar)
   -- skill_executions (similar)
   -- por source (mcp/cron/webhook)
   -- por org (top 10)
   ```

3. **Categorización:**
   - USADO: `total >= 10` ejecuciones en la ventana.
   - POCO USADO: `1 <= total <= 9`.
   - NUNCA USADO: `total == 0`.

4. **Output (stdout + JSON):**
   ```
   === Runner Usage Report (last 30 days) ===

   agent_runner: USADO (245 ejecuciones, 87% success, avg 12.3s)
     top agents:
       - <uuid> (89 runs)
       - <uuid> (45 runs)
       ...

   flow_runner: USADO (1023 ejecuciones, 92% success, avg 45.6s)
     top flows:
       - <uuid> (234 runs)
       ...

   skill_runner (server-side): NUNCA USADO ⚠
     0 ejecuciones en 30 días

   === Source distribution ===
     MCP: 800 (62%)
     Cron: 350 (27%)
     Webhook: 150 (12%)

   === Top 10 orgs by usage ===
     <org_name> (<uuid>): 500 runs
     ...
   ```

5. **JSON output:** `reports/runners-usage-<YYYYMMDD>.json`:
   ```json
   {
     "window_days": 30,
     "generated_at": "2026-06-12T...",
     "agent_runner": {
       "category": "USADO",
       "total": 245,
       "succeeded": 213,
       "failed": 32,
       "success_rate": 0.87,
       "avg_duration_sec": 12.3,
       "top_agents": [{"agent_id": "...", "n": 89}, ...]
     },
     "flow_runner": {...},
     "skill_runner": {...},
     "by_source": {"mcp": 800, "cron": 350, "webhook": 150},
     "top_orgs": [{"org_id": "...", "n": 500}, ...]
   }
   ```

6. **NO contiene PII:** solo `agent_id`, `flow_id`, `org_id`
   (UUIDs). NO nombres, NO emails. Esto lo hace commiteable a
   git sin riesgo.

7. **Top agents/flows con high failure rate:**
   ```sql
   SELECT agent_id, COUNT(*) AS n,
     COUNT(*) FILTER (WHERE status = 'failed') AS failed,
     ROUND(COUNT(*) FILTER (WHERE status = 'failed')::numeric / COUNT(*), 2) AS fail_rate
   FROM agent_runs
   WHERE started_at >= NOW() - INTERVAL '$1 days'
   GROUP BY agent_id
   HAVING COUNT(*) >= 5  -- mínimo para que la tasa sea significativa
     AND COUNT(*) FILTER (WHERE status = 'failed')::numeric / COUNT(*) > 0.5
   ORDER BY fail_rate DESC LIMIT 5;
   ```

8. **Edge case — datos insuficientes:**
   - Si `days < 7`: warning "recommend 30+ days for accurate analysis".
   - Thresholds ajustados: `USADO = total >= max(1, days/3)`.
   - Exit 0 igual.

9. **Email summary (opcional, opt-in):**
   - `--email admin@domain.com` → envía el JSON como attachment.
   - Default: solo stdout + file.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Dashboard en vivo de uso | Out of scope. El comando es on-demand. |
| B | Auto-tomar decisiones (matar runners no usados) | Riesgoso. La decisión es humana. |
| C | Telemetría con PII (org names) | Hace el reporte NO commiteable. Solo UUIDs. |
| D | Análisis de cost (issue 15.3 ya da cost per run) | Lo mencionamos en el reporte (cross-reference), pero no lo duplicamos. |
| E | Predicción ML de "este runner se va a usar más" | Speculación. Datos son mejores que ML para decisiones. |

## Por qué comando on-demand + JSON commiteable gana

- **Simple:** 1 comando, output predecible.
- **Reproducible:** correrlo mañana da el mismo shape de output.
- **Commiteable:** el JSON puede ir a git como report del
  estado actual. Cero PII.
- **Reversible:** las decisiones que se toman basadas en este
  reporte se revisan cada 6 meses (mismo report, datos nuevos).

## Detalle de implementación

- `cmd/domain-admin/runners_usage.go` con la lógica del comando.
- `internal/admin/runners_usage/queries.go` con las queries SQL
  (separadas para testeo).
- `internal/admin/runners_usage/categorize.go` con la lógica
  USADO/POCO/NUNCA.
- `internal/admin/runners_usage/format.go` con el formateo
  ASCII + JSON.
- Wire en `cmd/domain/main.go` con subcommand `admin`.

## Riesgos

- **R1:** Los datos son insuficientes (<30 días). **Aceptable:**
  warning en el output. Decisiones se difieren hasta tener
  datos.
- **R2:** El reporte revela que TODO se usa poco. **Aceptable:**
  confirma que el sistema es chico todavía. Las decisiones se
  ajustan.
- **R3:** Top orgs/agents en el JSON filtran info que el admin
  considera sensible. **Mitigación:** UUIDs sin nombres. El
  admin puede cruzar con otro query si quiere nombres.
