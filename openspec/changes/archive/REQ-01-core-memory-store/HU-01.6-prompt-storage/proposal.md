# Proposal: HU-01.6-prompt-storage

## Intención

Persistir los prompts del usuario en la tabla `user_prompts` para mantener trazabilidad de intenciones entre sesiones. Esto permite que el sistema recuerde qué instrucciones se dieron previamente y las reutilice como contexto sin depender de la sesión activa.

## Scope

**Incluye:**
- `AddPrompt(sessionID, content, project string) (int64, error)` — inserta un prompt, valida content no vacío
- `GetPrompt(id int64) (*Prompt, error)` — obtiene prompt por ID
- `ListPrompts(project string, limit, offset int) ([]Prompt, error)` — lista prompts con filtro opcional por proyecto
- `DeletePrompt(id int64) error` — elimina prompt por ID
- `SearchPrompts(query, project string, limit int) ([]Prompt, error)` — búsqueda FTS5 sobre `prompts_fts`
- `capturePrompt(content string)` — alimenta buffer process-local para que `domain_mem_save` posterior pueda referenciarlo
- `GetCapturedPrompt() string` — recupera y limpia el buffer de contexto
- Tests para todas las operaciones

**No incluye:**
- Sesiones (REQ-02)
- Privacy stripping (HU-01.7) — aunque se aplica en AddPrompt, el stripping mismo es otra HU
- Export/import de prompts (HU-01.8) — cubierto en esa HU
- CLI para prompts (REQ-03)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Archivo | `internal/store/prompt.go` — nuevo, funciones con `Store` receiver |
| Validación | Content vacío → `fmt.Errorf("content cannot be empty")` |
| Ordenamiento | `ORDER BY created_at DESC` en ListPrompts |
| FTS5 | Reusa `prompts_fts` creada en HU-01.1; misma interface que SearchObservations |
| Context process-local | Variable global `capturedPrompt string` con `sync.Mutex` en paquete aparte `internal/context/` o dentro de `store/` |
| Seguridad | Sanitizar query FTS5 (escapar caracteres especiales como `"`, `*`, `-`) |

El buffer process-local (`capturedPrompt`) es un mecanismo simple: cuando el usuario envía un prompt, `capturePrompt()` lo guarda en una variable protegida por mutex. Luego, cuando se invoca `domain_mem_save` (vía MCP o CLI), `GetCapturedPrompt()` recupera ese contenido y lo incluye como contexto. Esto evita que el prompt se pierda entre la captura y la acción del usuario.

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| FTS5 query malformada por caracteres especiales | Media | Sanitizer de queries: escapar/eliminar caracteres reservados de FTS5 antes de ejecutar |
| Session ID inválida | Baja | FK constraint en SQLite maneja el error; propagar error al caller con mensaje claro |
| Race condition en capturedPrompt | Baja | `sync.Mutex` alrededor de lectura/escritura del buffer |
| Prompts duplicados | Baja | No hay dedup explícita para prompts (no es necesario); el usuario puede eliminar manualmente |

## Testing

- **AddPrompt exitoso:** Insertar prompt con session_id, content, project válidos → verificar fila existe
- **AddPrompt content vacío:** Insertar con content="" → esperar error
- **AddPrompt session inexistente:** Insertar con session_id fake → esperar error FK
- **GetPrompt:** Insertar prompt → leer por ID → verificar campos
- **ListPrompts:** Insertar 3 prompts en 2 proyectos → listar con project filter → verificar cantidad y orden
- **DeletePrompt:** Insertar → eliminar → GetPrompt devuelve error
- **SearchPrompts:** Insertar prompts con contenido conocido → search con MATCH → verificar resultados
- **capturePrompt:** Llamar capturePrompt → GetCapturedPrompt devuelve el mismo string
- **Sabotaje:** Romper validación de content vacío → AddPrompt("") no falla → test cae → restaurar
