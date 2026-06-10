# issue-02.4-context-retrieval

**Origen:** `REQ-02-session-lifecycle`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** agente de IA iniciando una nueva sesión
**Quiero** consultar el contexto reciente (últimas sesiones, observaciones, prompts) filtrado por scope y con límite configurable
**Para** retomar el trabajo exactamente donde lo dejé sin necesidad de que el usuario me explique todo de nuevo

## Criterios de aceptación

```gherkin
Feature: Context retrieval

  Scenario: Obtener contexto reciente de un proyecto
    Given existen 5 sesiones en el proyecto "Domain"
    And existen 10 observaciones en esas sesiones
    When se llama a domain_mem_context con project="Domain" y limit=3
    Then se retornan hasta 3 sesiones recientes ordenadas por started_at DESC
    And se retornan hasta 3 observaciones recientes
    And cada sesión incluye id, project, started_at, status

  Scenario: Filtrar por scope personal
    Given existen observaciones con scope="personal" y scope="project"
    When se llama a domain_mem_context con scope="personal"
    Then solo se retornan observaciones con scope="personal"

  Scenario: Filtrar por scope project
    Given existen observaciones con scope="project" en proyecto "Domain"
    When se llama a domain_mem_context con scope="project" y project="Domain"
    Then solo se retornan observaciones con scope="project" y project="Domain"

  Scenario: Scope global retorna observaciones de todos los proyectos
    Given existen observaciones en proyecto "Domain" y proyecto "otro"
    When se llama a domain_mem_context con scope="global"
    Then se retornan observaciones de ambos proyectos

  Scenario: Cross-project con scope personal
    Given existen observaciones personales en varios proyectos
    When se llama a domain_mem_context con scope="personal" y project=""
    Then se retornan observaciones personales de todos los proyectos

  Scenario: Limit controla cantidad de resultados
    Given hay 100 observaciones recientes
    When se llama a domain_mem_context con limit=5
    Then se retornan exactamente 5 observaciones

  Scenario: Limit por defecto es 10
    Given hay 20 observaciones recientes
    When se llama a domain_mem_context sin especificar limit
    Then se retornan 10 observaciones

  Scenario: Resultados formateados para consumo del agente
    Given se llama a domain_mem_context con project="Domain"
    Then el resultado incluye secciones "Sessions", "Observations", "Prompts"
    And cada sección tiene items con formato legible para LLM

  Scenario: Proyecto sin actividad reciente
    Given no hay sesiones ni observaciones en proyecto "vacio"
    When se llama a domain_mem_context con project="vacio"
    Then se retorna un contexto vacío (sin error)
```

## Análisis breve

- **Qué pide realmente:** Función `domain_mem_context(project?, scope?, limit?)` que consulta sesiones recientes, observaciones y prompts, filtrados por scope (project/personal/global) y proyecto, con límite configurable, formateado como texto estructurado para consumo de LLM
- **Módulos sospechados:** `internal/store/context.go` — query builder con filtros dinámicos
- **Riesgos / dependencias:** Depende de tablas sessions, observations, user_prompts (issue-01.1); queries dinámicos con filtros opcionales requieren construcción segura de SQL (sin inyección)
- **Esfuerzo tentativo:** M

## Verificación previa

- [x] Revisar codebase (grep) — proyecto greenfield
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** No existe `internal/store/context.go`
- **Acción derivada:** Crear `internal/store/context.go` con ContextQuery y formateo
