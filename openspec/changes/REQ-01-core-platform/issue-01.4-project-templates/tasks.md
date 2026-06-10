# Tasks: issue-01.4-project-templates

## Backend

- [ ] `migrations/000021_create_project_templates.sql`: tabla + índices
- [ ] `internal/store/pg/project_template_store.go`: CRUD interface + implementation
- [ ] `internal/service/project_template.go`: lógica de negocio + validación de settings
- [ ] `cmd/domain/project_template.go`: comandos cobra (create, list, get, update, delete)
- [ ] Seed data: templates built-in (default, go-backend, frontend, data-pipeline)
- [ ] Integrar con ProjectService.Create: aceptar template_id, copiar settings

## Tests

- [ ] Test unitario: crear template con settings válidos
- [ ] Test unitario: crear template con name duplicado → error
- [ ] Test unitario: crear proyecto desde template verifica settings y skills
- [ ] Test unitario: actualizar template no afecta proyectos existentes
- [ ] Test unitario: settings inválidos → error de validación
- [ ] Test de integración: seed templates se crean al migrate up
- [ ] Sabotaje: eliminar template → proyectos existentes siguen intactos

## Cierre

- [ ] Verificación manual: `domain project-template create`, `domain project create --template go-backend`
- [ ] Suite verde
