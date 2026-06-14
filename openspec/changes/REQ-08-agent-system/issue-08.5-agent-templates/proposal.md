# Proposal: issue-08.5-agent-templates

## Intención

Implementar un sistema de templates de agente: definiciones preconfiguradas (system prompt, skills, temperatura, modelo) que los usuarios pueden listar e instanciar, con opción de override de parámetros.

## Scope

**In scope:**
- Tabla/seed `agent_templates` con templates predefinidos
- 4 templates base: Code Reviewer, Architecture Advisor, Bug Hunter, PR Reviewer
- Endpoint GET /agent-templates → lista templates
- Endpoint POST /agents/from-template → instancia template → crea agente (usa issue-08.1)
- Override de parámetros en instanciación: temperature, max_tokens, skills adicionales
- Templates seedeados en DB (migration o archivo YAML)

**Out of scope:**
- UI para crear templates custom (solo seed)
- Versionado de templates
- Templates por proyecto (todos son globales)

## Enfoque técnico

- Tabla `agent_templates`: id, slug (unique), name, description, default_system_prompt, default_temperature, default_max_tokens, recommended_model_id, category
- Tabla `agent_template_skills`: template_id, skill_id (recommended)
- Seeds en migration SQL con los 4 templates
- `AgentTemplateService.GetAll()`, `GetBySlug()`, `Instantiate(slug, overrides) -> Agent`
- `Instantiate()` crea un Agent usando AgentService.Create() con los valores del template + overrides

## Riesgos

- Skills recomendados pueden no existir si el skill registry cambia → validar en seed time y en instanciación
- Templates hardcodeados en migration → difícil de actualizar sin nueva migration → considerar archivo YAML externo

## Testing

- **Unit:** TemplateService.GetAll, GetBySlug, Instantiate con overrides
- **Integration:** Template → Agent creation pipeline
- **Gherkin:** Escenarios del hu.md
- **Sabotaje:** Template no encontrado → 404
