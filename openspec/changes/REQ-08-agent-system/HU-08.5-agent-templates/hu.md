# HU-08.5-agent-templates

**Origen:** `REQ-08-agent-system`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario
**Como** usuario nuevo del sistema de agentes
**Quiero** poder elegir entre plantillas predefinidas como "Code Reviewer", "Architecture Advisor", "Bug Hunter" o "PR Reviewer" que vienen con system prompt, skills recomendados y configuración optimizada
**Para** crear agentes útiles rápidamente sin tener que configurar todo desde cero

## Criterios de aceptación

### Scenario 1: Listar templates
**Given** 4 templates predefinidos en el sistema
**When** se consulta GET /agent-templates
**Then** retorna: "Code Reviewer", "Architecture Advisor", "Bug Hunter", "PR Reviewer"
**And** cada template incluye: name, description, default_system_prompt, recommended_skills, default_temperature, recommended_model

### Scenario 2: Instanciar template
**Given** el template "Code Reviewer"
**When** se instancia con POST /agents/from-template con slug "code-reviewer" y name "Mi Code Reviewer"
**Then** se crea un nuevo agente
**And** el agente tiene el system prompt del template
**And** los skills recomendados se asignan al agente
**And** la temperatura default es la del template (0.3)
**And** el modelo recomendado se usa como default

### Scenario 3: Customizar antes de instanciar
**Given** el template "Bug Hunter"
**When** se instancia con temperatura=0.5 (override) y un skill adicional "list-files"
**Then** el agente usa temp=0.5 (no la del template)
**And** tiene los skills del template + "list-files"

### Scenario 4: Template no encontrado
**Given** un slug de template inválido "nonexistent"
**When** se intenta instanciar
**Then** retorna HTTP 404
**And** error: "template_not_found"

## Análisis breve

- **Qué pide realmente:** Sistema de plantillas de agente con presets configurables que permitan instanciar agentes preconfigurados, con opción de override de parámetros.
- **Módulos sospechados:** `internal/agent/`, `internal/template/`, `internal/api/`
- **Riesgos / dependencias:** Depende de agent definitions (08.1) para la creación, del skill registry (05.1-05.2) para validar skills recomendados.
- **Esfuerzo tentativo:** S**
