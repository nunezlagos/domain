# issue-08.2-normalization-warnings

**Origen:** `REQ-08-project-resolution`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador con proyectos con nombres similares
**Quiero** que el nombre del proyecto se normalice (lowercase, trim, collapse separadores)
**Para** evitar duplicados por diferencias triviales de formato

**Como** desarrollador
**Quiero** recibir warnings cuando el proyecto detectado es similar a otro existente
**Para** saber si debería consolidar o si estoy trabajando en el proyecto incorrecto

## Criterios de aceptación

```gherkin
Scenario: Normaliza a lowercase
  Given un raw project name "My-App"
  When se ejecuta NormalizeProject("My-App")
  Then el resultado debe ser "my-app"

Scenario: Trim espacios
  Given un raw project name "  my-app  "
  When se ejecuta NormalizeProject("  my-app  ")
  Then el resultado debe ser "my-app"

Scenario: Collapse hyphens a un solo hyphen
  Given un raw project name "my---app"
  When se ejecuta NormalizeProject("my---app")
  Then el resultado debe ser "my-app"

Scenario: Collapse underscores a un solo underscore
  Given un raw project name "my___app"
  When se ejecuta NormalizeProject("my___app")
  Then el resultado debe ser "my_app"

Scenario: Hyphen y underscore se consideran separadores equivalentes
  Given un raw project name "my_app"
  When se ejecuta NormalizeProject("my_app")
  Then el resultado debe ser "my_app"
  And NormalizeProject("my-app") debe producir "my-app"
  # Ambos se mantienen pero colapsan duplicados internos

Scenario: Similar-project detecta Levenshtein distance baja
  Given existen proyectos "my-app" y "myapp"
  When se ejecuta CheckSimilarProjects("my-app")
  Then debe incluir warning para "myapp" con distancia Levenshtein calculada

Scenario: Similar-project detecta substring match
  Given existe proyecto "backend-service"
  When se ejecuta CheckSimilarProjects("backend")
  Then debe incluir warning: "backend" es substring de "backend-service"

Scenario: Sin proyectos similares no genera warnings
  Given solo existe proyecto "completely-unique-name"
  When se ejecuta CheckSimilarProjects("totally-different")
  Then no debe generar warnings

Scenario: Warnings se incluyen en respuesta de creación de sesión
  Given un proyecto con similar existente
  When se crea una nueva sesión con ese proyecto
  Then la respuesta HTTP debe incluir `warnings: [{type: "similar_project", ...}]`

Scenario: Normalización es idempotente
  Given un raw project name cualquiera
  When se ejecuta NormalizeProject() dos veces
  Then ambos resultados deben ser idénticos
```

## Análisis breve

- **Qué pide realmente:** Pipeline de normalización (lowercase, trim, collapse separadores), detector de similitud con Levenshtein y substring, inclusión de warnings en respuestas de API
- **Módulos sospechados:** `internal/project/normalize.go` + `internal/project/similar.go`
- **Riesgos / dependencias:** Levenshtein para N proyectos puede ser O(N*M); limitar a proyectos activos recientes
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:** —
- **Acción derivada:** —
