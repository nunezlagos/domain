# Design: issue-02.4-audit-log

## Decisión arquitectónica

**Almacenamiento:** Tabla `audit_log` en Postgres, append-only, BIGSERIAL PK.
**Inmutabilidad:** Trigger BEFORE UPDATE/DELETE en DB + la app nunca ejecuta esas operaciones.
**Consultas:** Índices compuestos para filtros comunes.
**Retención:** Comando CLI prune con batch DELETE.

## Alternativas descartadas

- **Tabla separada por entidad:** Complejidad innecesaria. Una tabla única es más consultable.
- **Log a archivo:** Menos consultable, no permite filtrar fácilmente.
- **Event Sourcing:** Sobredimensionado. No necesitamos replay de eventos.
- **Hash chain (blockchain-like):** Overkill para auditoría interna. Podemos agregar firma en v2.

## Diagrama

```
Handler → operación exitosa
  │
  └─→ audit.Logger.Log(ctx, AuditEntry{
        ActorID:    ctx.UserID,
        Action:     "create",
        EntityType: "project",
        EntityID:   project.ID,
        OldValues:  nil,
        NewValues:  json(project),
        IPAddress:  ctx.RealIP,
      })
  │
  └─→ INSERT INTO audit_log (...)

Consulta:
  GET /api/v1/audit-logs?actor_id=xxx&entity_type=project&action=delete&limit=50&cursor=12345
  │
  └─→ SELECT * FROM audit_log
      WHERE (actor_id = ? OR ? IS NULL)
        AND (entity_type = ? OR ? IS NULL)
        AND (action = ? OR ? IS NULL)
        AND id < ?  -- cursor
      ORDER BY id DESC
      LIMIT ?
```

## Trigger de inmutabilidad

```sql
CREATE OR REPLACE FUNCTION reject_audit_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only: modifications are not allowed';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_log_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION reject_audit_modification();
```

## Store interface

```go
type AuditStore interface {
    Log(ctx context.Context, entry *AuditEntry) error
    Query(ctx context.Context, filter AuditFilter) ([]*AuditEntry, *Cursor, error)
    Prune(ctx context.Context, before time.Time) (int64, error)
}

type AuditEntry struct {
    ID         int64            `json:"id"`
    ActorID    uuid.UUID        `json:"actor_id"`
    Action     string           `json:"action"`
    EntityType string           `json:"entity_type"`
    EntityID   uuid.UUID        `json:"entity_id"`
    OldValues  *json.RawMessage `json:"old_values,omitempty"`
    NewValues  *json.RawMessage `json:"new_values,omitempty"`
    IPAddress  string           `json:"ip_address,omitempty"`
    OccurredAt time.Time        `json:"occurred_at"`
}

type AuditFilter struct {
    ActorID    *uuid.UUID `json:"actor_id,omitempty"`
    EntityType string     `json:"entity_type,omitempty"`
    Action     string     `json:"action,omitempty"`
    Cursor     int64      `json:"cursor,omitempty"`
    Limit      int        `json:"limit,omitempty"`  // default 50, max 200
}
```

## TDD plan

1. Test Log escribe registro con todos los campos
2. Test Query por actor_id
3. Test Query por entity_type
4. Test Query por action
5. Test Query combinado (actor + entity + action)
6. Test Query con cursor pagination
7. Test UPDATE directo a tabla es rechazado por trigger
8. Test DELETE directo a tabla es rechazado por trigger
9. Test Prune elimina solo registros anteriores a la fecha
10. Test Prune respeta registros recientes
11. Test handler llama a Log después de operación exitosa
12. Test handler NO llama a Log si operación falla (rollback)

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Crecimiento excesivo de la tabla | Media | Medio | Retention policy default 90d; prune automático via cron (issue-10) |
| Trigger no creado en migración | Baja | Alto | Test de integración que verifica trigger existe |
| IP falsa por spoofing de headers | Baja | Medio | Usar X-Forwarded-For solo desde proxy confiable; documentar riesgo |
| Performance de consultas sin índices | Baja | Alto | Índices en actor_id, (entity_type, action), occurred_at |
