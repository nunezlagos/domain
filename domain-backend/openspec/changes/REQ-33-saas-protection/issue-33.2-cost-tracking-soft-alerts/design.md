# Design: issue-33.2-cost-tracking-soft-alerts

## Contexto

`cost_logs` ya trackea cada request LLM con su costo (issue-15.3).
El job `runUsageAlertEvaluator` ya corre cada 5 min y evalúa
agregaciones. Lo que falta:

1. Threshold PER-ORG (no global). Default $100/día, configurable.
2. Email al admin (no bloqueante, soft).
3. Anti-spam: 1 alerta por org por día.

Sin esto, un cliente con un agente en loop llamando Anthropic nos
puede gastar el budget del mes antes de que nos enteremos.

## Decisión arquitectónica

**Estrategia:** extensión del job existente + nueva tabla
`cost_alert_thresholds` + tabla `cost_alerts_sent` para dedup.

1. **Tabla `org_cost_alert_thresholds`:**
   ```sql
   CREATE TABLE org_cost_alert_thresholds (
     organization_id UUID PRIMARY KEY REFERENCES organizations(id),
     daily_usd NUMERIC(10, 2) NOT NULL DEFAULT 100.00,
     updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
   );
   ```
   Default $100/día, configurable per-org via admin endpoint
   (similar a rate limit de 33.1).

2. **Tabla `cost_alerts_sent` (anti-spam):**
   ```sql
   CREATE TABLE cost_alerts_sent (
     id BIGSERIAL PRIMARY KEY,
     organization_id UUID NOT NULL REFERENCES organizations(id),
     alert_date DATE NOT NULL,  -- UTC date
     amount_usd NUMERIC(10, 2) NOT NULL,
     sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     UNIQUE(organization_id, alert_date)
   );
   ```
   La UNIQUE constraint + el INSERT ... ON CONFLICT DO NOTHING
   es la dedup natural. Una alerta por org por día UTC.

3. **Job extension** (`runUsageAlertEvaluator` en
   `cmd/domain/main.go`): además de lo que ya hace, agregar:
   ```sql
   SELECT ocl.organization_id, SUM(ocl.cost_usd) AS total
   FROM cost_logs ocl
   WHERE ocl.created_at >= date_trunc('day', NOW() AT TIME ZONE 'UTC')
     AND ocl.created_at < date_trunc('day', NOW() AT TIME ZONE 'UTC') + INTERVAL '1 day'
   GROUP BY ocl.organization_id
   HAVING SUM(ocl.cost_usd) >= (
     SELECT daily_usd FROM org_cost_alert_thresholds t
     WHERE t.organization_id = ocl.organization_id
   )
   ```
   Para cada org en el resultado:
   - `INSERT INTO cost_alerts_sent ... ON CONFLICT DO NOTHING`.
   - Si el insert afectó 1 fila (no era duplicado) → enviar email.

4. **Email content:**
   ```
   Subject: [domain] cost alert: org <name> exceeded $<threshold>/day (now $<total>)
   Body:
     Organization: <name> (<id>)
     Date (UTC): <YYYY-MM-DD>
     Threshold: $<threshold>
     Current spend: $<total>
     Breakdown by provider:
       Anthropic: $X (Claude Sonnet: $A, Claude Haiku: $B)
       OpenAI: $Y (GPT-4: $C)
     Link: https://api.tudominio.com/admin/orgs/<id>/usage
   ```
   Template minimalista, sin HTML. Plain text.

5. **Email sender:** reusar el `EmailSender` que ya existe en
   `internal/service/usagealerts/` (issue-15.3). Si SMTP no
   configurado → fallback log (warning).

6. **Aggregation por provider/model:** la query group-by debe
   incluir provider + model para el breakdown:
   ```sql
   SELECT provider, model, SUM(cost_usd) AS sub
   FROM cost_logs
   WHERE organization_id = $1 AND created_at >= $2
   GROUP BY provider, model
   ```
   Esto corre DESPUÉS de identificar las orgs que crossed, no
   en la query principal (optimización).

7. **Admin endpoint `PUT /api/v1/admin/cost-threshold/{orgID}`:**
   body `{daily_usd: 50}`. Para subir/bajar el threshold de un
   cliente sin deploy. Requiere auth de admin.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Hard cap (bloquear requests cuando excede) | El user fue explícito: "soft alert, no paywall". El admin decide qué hacer. |
| B | Slack webhook en vez de email | Útil como feature futura, pero email es el universal-fallback. |
| C | Alerta global (sin per-org) | Muy ruidoso: 1 cliente te alerta TODOS los días. Per-org es granular. |
| D | Aggregation en tiempo real (no batch cada 5 min) | Más complejo (triggers o stream). Batch cada 5 min es suficiente para el caso de uso. |
| E | Multi-currency (EUR, ARS, etc) | El sistema es USD (precios LLM son USD). Multi-currency es feature futura. |

## Por qué tabla de thresholds + alerts sent gana

- **Configurable sin deploy:** UPDATE en la tabla, el job lo
  lee en la próxima corrida.
- **Anti-spam built-in:** UNIQUE constraint hace la dedup
  natural. Cero código extra.
- **Audit trail:** el INSERT en `cost_alerts_sent` queda
  como log. El admin puede ver histórico.
- **Extensible:** si en el futuro se quiere alerta por hora o
  por modelo específico, se agrega columna + branch.

## Detalle de implementación

- Migración: `migrations/000093_cost_alerts.sql` con las 2 tablas.
- `internal/service/usagealerts/threshold_checker.go`:
  - `CheckThresholds(ctx) ([]Alert, error)` — corre la query
    principal.
  - `SendAlerts(ctx, alerts []Alert) error` — para cada alert,
    INSERT en `cost_alerts_sent` (ON CONFLICT DO NOTHING) y si
    insertó, enviar email.
  - `RenderEmailBody(alert) (subject, body string)` — template
    plain text.
- Modificar `runUsageAlertEvaluator` (existente) para invocar
  `CheckThresholds` + `SendAlerts` después del flujo actual.
- Admin endpoint: `internal/api/handler/admin/cost_threshold.go`
  con `PUT /cost-threshold/{orgID}`.

## Riesgos

- **R1:** La query de agregación puede ser lenta con muchos
  cost_logs. **Mitigación:** índice en `(organization_id,
  created_at)` y agregación pre-calculada vía materialized view
  (refresca cada hora).
- **R2:** Email puede caer en spam. **Aceptable:** es
  notificación operacional, no marketing. Si se quiere mejorar,
  configurar SPF/DKIM en el dominio del server.
- **R3:** Múltiples pods corren el job → duplicación de emails.
  **Mitigación:** la UNIQUE constraint + el INSERT ON CONFLICT
  ya previene. Solo 1 email se envía.
