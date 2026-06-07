# Proposal: HU-04.4-mcp-session-tools

## Intención

Exponer 4 herramientas MCP para el ciclo de vida de sesiones de trabajo: `domain_mem_session_start`, `domain_mem_session_end`, `domain_mem_session_summary`, `domain_mem_capture_passive`. Las sesiones agrupan observaciones bajo un mismo contexto temporal, permitiendo memoria episódica.

## Scope

**Incluye:**
- `domain_mem_session_start`: Crear sesión con id único y directory. Idempotente (si existe, devolver la existente).
- `domain_mem_session_end`: Cerrar sesión activa con timestamp. Summary opcional que se persiste.
- `domain_mem_session_summary`: Guardar summary estructurado. Parsear markdown para extraer secciones (Accomplished, Decisions, Next Steps) y crear observaciones hijas.
- `domain_mem_capture_passive`: Crear observación de tipo "pattern" o "context" desde texto, vinculada opcionalmente a session_id.
- Validaciones: sesión debe existir para end/summary, no se puede end una sesión ya ended.

**Excluye:**
- CRUD de observaciones (HU-04.2)
- Búsqueda (HU-04.3)
- Admin (HU-04.5)
- Resolución de proyecto (HU-04.6)

## Enfoque técnico

1. **Modelo de sesión:** `Session { ID, Directory, Status (active|ended), StartedAt, EndedAt, HasSummary }`. Store en tabla separada o embebido en observaciones.
2. **Idempotencia:** `session_start` hace `INSERT OR IGNORE` / `SELECT` si existe.
3. **Markdown parser:** Regex simple para `## Accomplished`, `## Decisions`, `## Next Steps`. Cada item de lista (`- item`) se convierte en observación.
4. **Capture passive:** Toma texto arbitrario, crea observation con type "pattern" (si discovery) o "context" (default), vincula session_id si se provee.
5. **Timestamps:** `time.Now().UTC()` en start/end.

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Markdown parsing frágil | Observaciones incompletas | Log warning si no se puede parsear, guardar raw igual |
| Session ID no único | Colisión | Usar UUID v4 si el cliente no provee id |
| Session abierta para siempre | Orfandad | Timeout de 24h de inactividad (futuro) |
| Capture_passive sin content | Observación vacía | Validar content no vacío (>3 chars) |

## Testing

- **Unit:** Ciclo completo start → end → summary. Idempotencia. Sesión inexistente. Markdown parsing con varios formatos.
- **Integration:** Server real, secuencia de 4 tools.
- **Sabotaje:** End sin start → error. Summary sin content → error.
