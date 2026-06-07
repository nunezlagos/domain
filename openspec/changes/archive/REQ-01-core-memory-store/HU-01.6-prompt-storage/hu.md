# HU-01.6-prompt-storage

**Origen:** `REQ-01-core-memory-store`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** que los prompts que ingreso se guarden automáticamente
**Para** mantener el contexto de mis intenciones entre sesiones y poder reutilizar instrucciones previas

## Criterios de aceptación

```gherkin
Scenario: Guardar un prompt válido
  Given una sesión activa con ID "session-1"
  When se guarda un prompt con content "analiza el archivo main.go" y project "Domain"
  Then el prompt se persiste en la tabla user_prompts
  And el registro tiene session_id, content, project y created_at

Scenario: Rechazar prompt con content vacío
  Given una sesión activa
  When se intenta guardar un prompt con content vacío
  Then la operación falla con error "content cannot be empty"

Scenario: Buscar prompts por FTS5
  Given existen prompts con contenido "refactorizar modulo auth" y "testear handler login"
  When se busca con query "refactorizar"
  Then se obtiene solo el prompt que contiene "refactorizar modulo auth"

Scenario: Listar prompts recientes con filtro de proyecto
  Given prompts en proyectos "Domain" y "otro"
  When se listan prompts con project="Domain"
  Then solo se devuelven los prompts del proyecto "Domain"
  And los resultados se ordenan por created_at descendente

Scenario: Eliminar prompt por ID
  Given un prompt existente con ID 42
  When se elimina el prompt con ID 42
  Then el registro ya no existe en user_prompts

Scenario: Prompt capturado alimenta el contexto local del proceso
  Given un prompt recién guardado
  When capturePrompt() se ejecuta
  Then el contenido está disponible en el contexto process-local para domain_mem_save
```

## Análisis breve

- **Qué pide realmente:** CRUD para `user_prompts` (la tabla ya existe por HU-01.1) — `AddPrompt()`, `GetPrompt()`, `ListPrompts()`, `DeletePrompt()`, `SearchPrompts()`; integración con FTS5 existente; un mecanismo `capturePrompt()` que alimenta un buffer en memoria para que `domain_mem_save` pueda usarlo
- **Módulos sospechados:** `internal/store/prompt.go` — nuevo archivo con operaciones CRUD; posible ampliación de `internal/store/store.go` si `capturePrompt()` necesita acceso al *sql.DB
- **Riesgos / dependencias:** Depende de HU-01.1 (tabla user_prompts existe); FTS5 triggers ya creados en migración 002; session_id debe ser FK válida a sessions
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep) — verificar si ya existe lógica de prompts
- [ ] Revisar schema actual — confirmar tabla user_prompts y prompts_fts
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:** —
- **Acción derivada:** —
