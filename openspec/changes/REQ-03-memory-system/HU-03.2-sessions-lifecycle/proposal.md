# Proposal: HU-03.2-sessions-lifecycle

## Intención

Implementar el lifecycle completo de sesiones en Postgres: inicio, finalización, consulta de activas, estado. Una sesión representa una "interacción continua" del agente con el sistema de memoria, permitiendo agrupar observaciones y prompts por sesión.

## Scope

**Incluye:**
- Tabla `sessions` con migración
- Columnas: `id` (TEXT PK), `user_id` (UUID FK → users(id)), `project_id` (UUID FK → projects(id)), `directory`, `started_at`, `ended_at` (nullable), `summary` (nullable)
- `SessionStart(id, user_id, project_id, directory)` → inserta con started_at = now()
- `GetActiveSession(project_id, user_id)` → sesión con ended_at IS NULL para ese project/user
- `GetSession(id)` → sesión por id
- `ListSessions(project, limit)` → sesiones de un proyecto, ordenadas por started_at DESC
- `SessionStatus(id)` → "active" | "completed"
- Validación: no duplicados, no finalizar dos veces

**Excluye:**
- Asociación automática de observaciones a sesión (se hará en contexto mayor)
- Timeout automático de sesiones (futuro)

## Enfoque técnico

1. **Migración**: `CREATE TABLE sessions (id TEXT PRIMARY KEY, project VARCHAR(255) NOT NULL, directory TEXT, started_at TIMESTAMPTZ NOT NULL DEFAULT now(), ended_at TIMESTAMPTZ, summary TEXT)`
2. **Índice**: BTREE sobre (project, ended_at) para búsqueda rápida de activas
3. **Active = ended_at IS NULL**: simple, sin flag booleano redundante
4. **Capa Go**: `SessionStore` interface con métodos tipados
5. **Error handling**: `ErrSessionAlreadyEnded` y `ErrSessionNotFound` como sentinel errors

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| ID de sesión no único por colisión de UUID cliente | Medio | Cliente debe generar UUID v7 únicos; DB tiene PK constraint |
| Concurrent start/end race condition | Bajo | Usar `INSERT ... ON CONFLICT` y verificar ended_at antes de update con WHERE ended_at IS NULL |

## Testing

- **Unitarios**: store mockeado
- **Integración**: pgtest con container, probar ciclo completo start → get active → end → get active (debe devolver NULL)
- **Sabotaje**: intentar finalizar sesión dos veces → debe fallar con error específico
