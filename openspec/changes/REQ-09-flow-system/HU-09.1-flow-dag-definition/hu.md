# HU-09.1-flow-dag-definition

**Origen:** `REQ-09-flow-system`
**Persona:** dx-engineer, integrator
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** ingeniero de automatización
**Quiero** definir flujos de trabajo como DAGs con pasos ordenados, validación de ciclos e importación/exportación YAML/JSON
**Para** modelar pipelines complejos de forma declarativa, reutilizable y versionable

## Criterios de aceptación

### Escenario 1: Crear un flow DAG con campos obligatorios

```gherkin
Dado que soy un usuario autenticado con permiso `flow:write`
Cuando envío un POST a `/api/v1/flows` con el cuerpo:
  """
  {
    "name": "Customer Onboarding",
    "slug": "customer-onboarding",
    "description": "Flujo de onboarding de nuevos clientes",
    "project_id": "proj-abc-123",
    "steps": [
      {"id": "s1", "type": "skill_call", "params": {"skill_slug": "validate-email"}},
      {"id": "s2", "type": "llm_call", "params": {"prompt_template": "Bienvenido {{name}}"}, "depends_on": ["s1"]},
      {"id": "s3", "type": "code_exec", "params": {"script": "return ctx.result"}, "depends_on": ["s2"]}
    ]
  }
  """
Entonces el sistema responde con HTTP 201
Y el body contiene el campo `id` (UUID)
Y el body contiene el campo `version` igual a 1
Y el body contiene `slug` igual a "customer-onboarding"
Y los `steps` se almacenan en el orden enviado
```

### Escenario 2: Validación rechaza DAG con ciclos

```gherkin
Dado que existe un flow con slug "existing-flow"
Cuando envío un PUT a `/api/v1/flows/existing-flow` con steps que tienen dependencias cíclicas:
  """
  "steps": [
    {"id": "a", "type": "skill_call", "params": {}, "depends_on": ["b"]},
    {"id": "b", "type": "skill_call", "params": {}, "depends_on": ["c"]},
    {"id": "c", "type": "skill_call", "params": {}, "depends_on": ["a"]}
  ]
  """
Entonces el sistema responde con HTTP 422
Y el body contiene `error.code` igual a "CYCLE_DETECTED"
Y el body contiene `error.details.cycle` listando ["a", "b", "c"]
Y el flow no se actualiza
```

### Escenario 3: Validación rechaza steps con tipo inválido

```gherkin
Dado que quiero crear un flow
Cuando envío un POST a `/api/v1/flows` con un step de tipo `invalid_type`
Entonces el sistema responde con HTTP 422
Y el body contiene `error.field` igual a "steps[0].type"
Y el error lista los tipos válidos: skill_call, llm_call, code_exec, conditional, parallel, wait, human_input, domain_agent_run, sub_flow, transform
```

### Escenario 4: Validación rechaza steps sin campos requeridos

```gherkin
Dado que quiero crear un flow
Cuando envío un POST a `/api/v1/flows` con un step sin el campo `id`
Entonces el sistema responde con HTTP 422
Y el body contiene `error.field` igual a "steps[0].id"
Y el mensaje indica que el campo es requerido

Cuando envío un POST con un step sin el campo `type`
Entonces el sistema responde con HTTP 422
Y el body contiene `error.field` igual a "steps[0].type"
```

### Escenario 5: Exportar flow a YAML

```gherkin
Dado que existe un flow con slug "customer-onboarding"
Cuando envío un GET a `/api/v1/flows/customer-onboarding/export?format=yaml`
Entonces el sistema responde con HTTP 200
Y el Content-Type es `application/x-yaml`
Y el body YAML contiene los campos `name`, `slug`, `description`, `version`
Y el body YAML contiene la clave `steps` con el array de pasos
Y al reimportar ese YAML se crea un flow idéntico
```

### Escenario 6: Importar flow desde JSON

```gherkin
Dado que tengo un archivo JSON con una definición de flow válida
Cuando envío un POST a `/api/v1/flows/import` con Content-Type `application/json`
Entonces el sistema responde con HTTP 201
Y el flow se crea con los mismos campos del archivo
Y se asigna un nuevo `id` y `version` 1

Dado que el archivo JSON contiene un `slug` que ya existe
Cuando envío el POST de importación
Entonces el sistema responde con HTTP 409
Y el error indica que el slug ya está en uso
```

### Escenario 7: Listar flows con paginación y filtro por proyecto

```gherkin
Dado que existen 25 flows en el proyecto "proj-abc-123"
Cuando envío un GET a `/api/v1/flows?project_id=proj-abc-123&limit=10&offset=0`
Entonces el sistema responde con HTTP 200
Y el body contiene `data` con 10 items
Y el body contiene `pagination.total` igual a 25
Y el body contiene `pagination.limit` igual a 10
Y el body contiene `pagination.offset` igual a 0
```

### Escenario 8: Slug se genera automáticamente si no se provee

```gherkin
Dado que creo un flow con name "Mi Flow de Prueba"
Y no envío el campo `slug`
Entonces el sistema genera un slug automáticamente a partir del name
Y el slug sigue el formato `mi-flow-de-prueba`
Y es único dentro del proyecto
```

## Análisis breve

- **Qué pide realmente:** CRUD completo de definiciones de flujo como DAG, con validación topológica (detección de ciclos), schema validation de steps, y serialización/deserialización YAML/JSON.
- **Módulos sospechados:** `internal/flow/`, `internal/api/handlers/flow.go`, `internal/models/flow.go`, `internal/validation/dag.go`
- **Riesgos / dependencias:** Algoritmo de detección de ciclos (DFS topological sort). Depende del módulo de proyectos (REQ-01). La validación de step types se delega a HU-09.2.
- **Esfuerzo tentativo:** L

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
- **Evidencia:**
- **Acción derivada:**
