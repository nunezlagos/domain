# Design: HU-08.5-agent-templates

## Decisión arquitectónica

**Patrón:** Seed data + Factory method.

```
[Seed Migration] ──▶ agent_templates table
                          │
                          ▼
                AgentTemplateService
                    │          │
                    ▼          ▼
              GetAll()    GetBySlug(slug)
                              │
                              ▼
                   Instantiate(slug, overrides)
                              │
                              ▼
                   AgentService.Create(agent)
                              │
                              ▼
                        Nuevo Agent
```

## Alternativas descartadas

1. **Templates en archivos YAML en filesystem:** Más flexibles pero requieren ruta configurable. Preferimos DB para consistencia con el resto.
2. **Templates como agentes con flag is_template:** Confunde el modelo de datos. Tabla separada es más clara.
3. **No permitir overrides:** Muy restrictivo. El usuario debe poder personalizar.

## Diagrama

```
Seed Data:
┌─────────────────────────────────────────────────────────────────────┐
│ agent_templates                                                     │
├──────────────┬──────────────────────┬──────────┬────────┬──────────┤
│ slug          │ name                │ prompt   │ temp   │ model    │
├──────────────┼──────────────────────┼──────────┼────────┼──────────┤
│ code-reviewer │ Code Reviewer       │ ...      │ 0.3    │ gpt-4    │
│ arch-advisor  │ Architecture Advisor│ ...      │ 0.5    │ claude-3 │
│ bug-hunter    │ Bug Hunter          │ ...      │ 0.2    │ gpt-4    │
│ pr-reviewer   │ PR Reviewer         │ ...      │ 0.3    │ gpt-4    │
└──────────────┴──────────────────────┴──────────┴────────┴──────────┘

Table agent_template_skills:
┌──────────────┬────────────────────────────┐
│ template     │ skill                      │
├──────────────┼────────────────────────────┤
│ code-reviewer │ list-files, read-file, ... │
│ arch-advisor  │ analyze-diagram, ...      │
│ ...          │ ...                        │
└──────────────┴────────────────────────────┘
```

## TDD plan

1. **Red:** Test listar templates retorna 4 templates
2. **Green:** Implementar GetTemplates() con seed data
3. **Refactor:** Implementar Instantiate() con AgentService
4. **Sabotaje:** Template no encontrado → 404

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|-----------|
| Skills recomendados no existen | Validar en seed migration; si falta un skill, loggear warning |
| Override inválido (temp > 2) | Validar igual que en agent creation; retornar 422 |
| Templates hardcodeados difíciles de mantener | Segunda iteración: cargar desde archivo YAML externo |
| Usuario quiere templates custom | Post-MVP: agregar CRUD de templates
