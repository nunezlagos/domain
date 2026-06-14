# Design: issue-03.2-sessions-lifecycle

## Decisión arquitectónica

**Tabla única con ended_at nullable como marcador de estado activa/completada.**

```
sessions
├── id          TEXT PRIMARY KEY              -- UUID v7 generado por el cliente
├── user_id     UUID NOT NULL REFERENCES users(id)
├── project_id  UUID NOT NULL REFERENCES projects(id)
├── directory   TEXT
├── started_at  TIMESTAMPTZ NOT NULL DEFAULT now()
├── ended_at    TIMESTAMPTZ                   -- NULL = activa
└── summary     TEXT                          -- resumen al finalizar
```

**Reglas de negocio:**
- Una sesión activa = `ended_at IS NULL`
- Para finalizar: `UPDATE sessions SET ended_at = now(), summary = $1 WHERE id = $2 AND ended_at IS NULL`
- Si el UPDATE afecta 0 filas, la sesión no existe o ya está cerrada → error
- No hay límite de sesiones activas por proyecto (múltiples agentes)

**Índices:**
- `sessions_active_idx` BTREE (project_id, user_id, ended_at) WHERE ended_at IS NULL → búsqueda rápida de activas por proyecto y usuario

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| Flag booleano `is_active` | Redundante con ended_at; riesgo de inconsistencia |
| Tabla separada `active_sessions` | Complejidad adicional innecesaria; eventual consistency |
| Timestamp de timeout automático | Fuera de scope por ahora; se agrega después si es necesario |

## Diagrama

```
SessionStart(id, project, dir)
  └── INSERT INTO sessions (id, project, directory)
        └── started_at = now(), ended_at = NULL

SessionEnd(id, summary)
  └── UPDATE sessions SET ended_at = now(), summary = $summary
        WHERE id = $id AND ended_at IS NULL
        └── rows affected = 0 → ErrSessionAlreadyEnded

GetActiveSession(project)
  └── SELECT * FROM sessions
        WHERE project = $project AND ended_at IS NULL
        ORDER BY started_at DESC LIMIT 1

SessionStatus(id)
  └── SELECT ended_at IS NULL AS is_active FROM sessions WHERE id = $id
        └── no rows → ErrSessionNotFound
```

## TDD plan

1. **Red**: Test: start → get active → assert active
2. **Green**: Implementar migración + SessionStart + GetActiveSession
3. **Red**: Test: start → end → get active → assert nil
4. **Green**: Implementar SessionEnd
5. **Refactor**: Extraer validaciones comunes
6. **Sabotaje**: end dos veces → debe fallar con error específico

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| ID duplicado por cliente | PK constraint en DB, error propagado al cliente |
| Race condition en end | UPDATE condicional con WHERE ended_at IS NULL + check rows affected |
| Sesiones huérfanas (start sin end) | Aceptable por ahora; limpieza futura con cron si es necesario |
