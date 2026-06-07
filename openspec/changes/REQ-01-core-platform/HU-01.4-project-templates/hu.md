# HU-01.4-project-templates

**Origen:** `REQ-01-core-platform`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** líder técnico configurando un nuevo proyecto en Domain
**Quiero** poder elegir un template de proyecto que defina skills default, agentes preconfigurados, preferencias de memoria y flujos típicos
**Para** que cada proyecto tenga su propia forma de trabajo sin tener que configurar todo desde cero

## Criterios de aceptación

### Escenario 1: Crear template de proyecto

```gherkin
Dado que soy administrador de la organización
Cuando creo un template con:
  | name           | "Go Backend"              |
  | description    | "Proyecto Go con Postgres" |
  | default_skills | ["code-review", "architecture", "docker"] |
  | settings       | {"memory_default_scope": "project"} |
Entonces el template se guarda en la tabla `project_templates`
Y se le asigna un id único
```

### Escenario 2: Crear proyecto desde template

```gherkin
Dado que existe el template "Go Backend"
Cuando creo un proyecto desde ese template con:
  | name          | "mi-api"           |
  | slug          | "mi-api"           |
  | repository_url| "https://github.com/org/mi-api" |
Entonces se crea el proyecto con las settings del template
Y los skills del template se pre-asignan al proyecto
Y los agentes default del template se crean para el proyecto
Y los flows default del template se crean para el proyecto
```

### Escenario 3: Template define forma de trabajo del proyecto

```gherkin
Dado un proyecto creado desde el template "Go Backend"
Cuando guardo una observación en ese proyecto
Entonces el scope default es "project" (definido en el template)
Y los skills recomendados son los del template
Y el agente "Code Reviewer" del template está disponible

Dado el template "Frontend React" con scope default "personal"
Cuando guardo una observación en un proyecto de ese template
Entonces el scope default es "personal"
```

### Escenario 4: Listar templates disponibles

```gherkin
Dado que existen los templates "Go Backend", "Frontend React", "Data Pipeline"
Cuando consulto GET /project-templates
Entonces obtengo los 3 templates
Y cada uno incluye: name, description, default_skills, settings
```

### Escenario 5: Actualizar template

```gherkin
Dado un template "Go Backend" existente
Cuando actualizo sus default_skills para agregar "security-audit"
Entonces los proyectos existentes NO se ven afectados
Y los nuevos proyectos creados desde el template incluyen "security-audit"
```

### Escenario 6: Template por defecto

```gherkin
Dado que el template "default" tiene is_default = true
Cuando creo un proyecto sin especificar template
Entonces se usa el template "default"
```

### Escenario 7: Proyecto sin template

```gherkin
Dado que quiero un proyecto sin template
Cuando creo un proyecto con template explícitamente vacío
Entonces el proyecto se crea sin skills pre-asignados
Y sin agentes preconfigurados
Y con settings default genéricos
```

## Análisis breve

- **Qué pide realmente:** CRUD de project_templates con settings JSONB y default_skills array. Template define scope default, skills pre-asignados, agentes preconfigurados y flows típicos. Al crear proyecto desde template, se copian las settings pero no se linkean (cambios futuros al template no afectan proyectos existentes).
- **Módulos sospechados:** `internal/store/pg/project_template.go`, `cmd/domain/project_template.go`, `internal/service/project.go`
- **Riesgos / dependencias:** Depende de HU-01.1 (migración 000021). Los templates por defecto deben crearse como seed data. Skills y agentes referenciados deben existir o crearse con el proyecto.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
