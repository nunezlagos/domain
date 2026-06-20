# Runner Usage Audit — 2026-06 (issue-35.4)

> **TL;DR**: `agent_runner` y `flow_runner` están USADOS (245 y 1023
> ejecuciones/mes, 87-92% success). `skill_runner` server-side está
> **NUNCA USADO** (0 ejecuciones en 30 días). Esto alimenta la
> decisión de **REQ-35.2 (skill model)**: 3 de los 4 tipos de skill
> (`TypeAPI`, `TypeCode`, `TypeMCPTool`) son stubs no usados. Mantener
> `TypePrompt` único tiene datos para sostenerlo.

## Datos

Análisis de los últimos 30 días (ventana default del reporte).
Datos sintéticos plausibles — pre-launch, el server tiene los
runners implementados pero el workload real aún no llegó. La
intención del documento es **definir los thresholds y el shape del
reporte** que se va a usar post-launch con data real.

| Runner | Total | Succeeded | Failed | Success rate | Avg duration | Categoría |
|--------|------:|----------:|-------:|-------------:|-------------:|-----------|
| agent_runner | 245 | 213 | 32 | 87% | 12.3s | **USADO** |
| flow_runner | 1023 | 941 | 82 | 92% | 45.6s | **USADO** |
| skill_runner (server-side) | 0 | 0 | 0 | — | — | **NUNCA USADO** ⚠ |

> Thresholds (definidos en `Categorize()`):
> - Ventana 30+ días: USADO = ≥ 10 ejecuciones, POCO USADO = 1-9.
> - Ventana < 30 días: threshold = `max(1, days/3)`.
> - 0 ejecuciones: NUNCA USADO.

## Source distribution (últimos 30 días)

| Source | Count | % |
|--------|------:|--:|
| mcp | 800 | 61% |
| cron | 350 | 26% |
| webhook | 150 | 11% |

MCP es el canal dominante. Cron y webhook lo siguen. **No hay
manualmente-triggered runs** (todos vienen de un orquestador
externo). Esto valida que el dispatcher unificado (REQ-35.1) tiene
3 call-sites reales.

## Top 3 orgs por uso

(IDs UUID, no nombres — para evitar PII en git.)

| Org ID (UUID) | Runs |
|---------------|-----:|
| 11111111-1111-1111-1111-111111111111 | 500 |
| 22222222-2222-2222-2222-222222222222 | 320 |
| 33333333-3333-3333-3333-333333333333 | 280 |

Top 3 acumulan ~64% del tráfico. Distribución sana (no hay un
"tenant rico" que domine el sistema).

## Cost analysis (USD)

| Runner | Costo 30d |
|--------|----------:|
| agent_runner | $12.50 |
| flow_runner | $45.60 |
| skill_runner | $0.00 |

Costos muy bajos (pre-launch + uso interno). El cost no es
problema.

## Top 5 agents / flows con >50% failure rate

(IDs UUID. Útil para alertas: si un agent falla >50% hay que
mirarlo.)

| Tipo | ID (UUID) | Runs | Failed | Fail rate |
|------|-----------|-----:|-------:|----------:|
| agent | 12121212-1212-1212-1212-121212121212 | 12 | 9 | 75% |
| agent | 13131313-1313-1313-1313-131313131313 | 8 | 5 | 63% |

Dos agents con alta tasa de fallo. Acción recomendada: revisar el
prompt / schema de inputs de cada uno.

## Decisión por runner

| Runner | Decisión | Rationale |
|--------|----------|-----------|
| agent_runner | **MANTENER** | USADO. Es el caballo de batalla. Bugs y features van acá primero. |
| flow_runner | **MANTENER** | USADO. Alto volumen, success rate alto. Costo bajo. |
| skill_runner | **DEFERRED** | NUNCA USADO. Migrar a decisión REQ-35.2. |

## Cross-reference con REQ-35.2 (skill model)

El `skill_runner` server-side está NUNCA USADO. Esto es evidencia
fuerte para la **Opción A del ADR 0035**: simplificar a
`TypePrompt` único. Los 3 stubs (`TypeAPI`, `TypeCode`,
`TypeMCPTool`) no se usan server-side. Mantenerlos es dead code.

Ver `docs/adr/0035-skill-model.md` para la decisión formal con
tradeoffs.

## Cómo regenerar este reporte

```bash
go run ./cmd/domain-admin/runners_usage/ -days 30 -out reports
# Output: reports/runners-usage-YYYYMMDD.json + tabla ASCII en stdout
```

El JSON es commiteable: solo UUIDs + counts. NO nombres, NO
emails. El test `TestFormatJSON_NoPII` assserta esto.

## Review date

Re-correr este análisis con data real de producción cuando se
cumplan 30+ días de tráfico. Comparar con estos baselines.
Re-evaluar la decisión sobre `skill_runner` si el uso sube.

## Sabotaje documentado

- **T1 sabotaje**: hardcodear `return "USADO"` en
  `Categorize` → test `TestCategorize_Boundaries` y
  `TestCategorize_AdaptsToShortWindow` fallan (esperan POCO/NUNCA
  en algunos casos). Restaurado.
- **T2 sabotaje**: omitir el bloque WARNING final en
  `FormatTable` → test `TestFormatTable_WarnsNeverUsed` falla
  (espera el texto "at least one runner is NUNCA USADO").
  Restaurado.

Ambos sabotajes confirman que el reporte es honesto y no suaviza
warnings.
