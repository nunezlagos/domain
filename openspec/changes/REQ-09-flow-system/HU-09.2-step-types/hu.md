# HU-09.2-step-types

**Origen:** `REQ-09-flow-system`
**Persona:** dx-engineer, integrator
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** ingeniero de automatización
**Quiero** disponer de 10 tipos de paso especializados (skill_call, llm_call, code_exec, conditional, parallel, wait, human_input, domain_agent_run, sub_flow, transform)
**Para** modelar cualquier pipeline de IA/automatización con bloques reutilizables y semántica clara

## Criterios de aceptación

### Escenario 1: skill_call ejecuta un skill por slug

```gherkin
Dado que existe un skill con slug "validate-email"
Y un flow con un paso skill_call que referencia ese slug
Cuando el flow ejecuta el paso skill_call
Entonces el runner invoca el engine de skills con `skill_slug: "validate-email"`
Y el paso pasa los parámetros definidos en `params` como input del skill
Y el resultado del skill se almacena en `steps.s1.result`
```

### Escenario 2: llm_call ejecuta un prompt template contra un LLM

```gherkin
Dado que tengo un paso llm_call con prompt_template "Resume el texto: {{input.text}}"
Y el contexto contiene `input.text` con valor "Texto largo..."
Cuando el flow ejecuta el paso llm_call
Entonces el sistema resuelve el template reemplazando `{{input.text}}`
Y envía el prompt resuelto al LLM configurado (provider + model de `params`)
Y el resultado del LLM se almacena en `steps.s2.result`

Dado que el paso incluye `model: "gpt-4"` y `temperature: 0.5`
Entonces la llamada al LLM usa esos parámetros exactos
```

### Escenario 3: code_exec ejecuta código arbitrario en sandbox

```gherkin
Dado que tengo un paso code_exec con `script: "return data.items.filter(i => i.active).length"`
Y el contexto contiene `data.items`
Cuando el flow ejecuta el paso code_exec
Entonces el script se ejecuta en un sandbox aislado (sin acceso a red/filesystem)
Y el resultado del script se almacena en `steps.s3.result`

Dado que el script lanza una excepción
Entonces el paso falla con el mensaje de error del script
Y el flow maneja el error según su policy (HU-09.4)
```

### Escenario 4: conditional evalúa condición y bifurca

```gherkin
Dado que tengo un paso conditional con:
  """
  "condition": "steps.s1.result.status == 'approved'",
  "if_branch": [{"id": "s3a", "type": "skill_call", "params": {"skill_slug": "send-welcome"}}],
  "else_branch": [{"id": "s3b", "type": "human_input", "params": {"question": "Revisar manualmente"}}]
  """
Y el resultado de s1.status es "approved"
Cuando el flow ejecuta el paso conditional
Entonces se ejecuta la rama `if_branch` (s3a)
Y no se ejecuta la rama `else_branch`

Dado que el resultado de s1.status es "rejected"
Cuando el flow ejecuta el paso conditional
Entonces se ejecuta la rama `else_branch` (s3b)
Y no se ejecuta la rama `if_branch`
```

### Escenario 5: parallel ejecuta N steps concurrentemente

```gherkin
Dado que tengo un paso parallel con:
  """
  "branches": [
    {"id": "p1", "type": "skill_call", "params": {"skill_slug": "check-email"}},
    {"id": "p2", "type": "llm_call", "params": {"prompt_template": "Analiza: {{input.text}}"}},
    {"id": "p3", "type": "code_exec", "params": {"script": "return 42"}}
  ]
  """
Cuando el flow ejecuta el paso parallel
Entonces los 3 pasos se lanzan concurrentemente (gorutinas separadas)
Y el paso espera a que TODOS completen
Y los resultados se almacenan en `steps.s4.results` como un array en orden de definición

Dado que uno de los pasos paralelos falla
Entonces el paso parallel se marca como fallido
Y los resultados parciales se conservan en `steps.s4.results`
```

### Escenario 6: wait pausa por duración o condición

```gherkin
Dado que tengo un paso wait con `duration_seconds: 30`
Cuando el flow ejecuta el paso wait
Entonces la ejecución se pausa por exactamente 30 segundos
Y luego continúa al siguiente paso

Dado que tengo un paso wait con `until_condition: "steps.s1.result.ready == true"`
Y la condición inicialmente es falsa
Cuando el flow ejecuta el paso wait
Entonces el sistema evalúa la condición cada 5 segundos
Y cuando la condición se cumple, continúa al siguiente paso
Y si pasan 300 segundos sin que la condición se cumpla, el paso falla con timeout
```

### Escenario 7: human_input pausa por aprobación humana

```gherkin
Dado que tengo un paso human_input con:
  """
  "question": "¿Aprobar el envío al cliente {{input.client}}?",
  "timeout_hours": 48
  """
Cuando el flow ejecuta el paso human_input
Entonces el sistema crea una tarea de aprobación pendiente
Y notifica a los usuarios asignados (vía webhook/email/configurable)
Y la ejecución del flow queda pausada en estado `awaiting_input`

Cuando un usuario responde con `{"approved": true, "comment": "OK"}`
Entonces el paso se completa con resultado `approved: true`
Y la ejecución continúa

Cuando pasan 48 horas sin respuesta
Entonces el paso falla con timeout
Y el flow sigue la política de error configurada
```

### Escenario 8: domain_agent_run delega a un agente

```gherkin
Dado que existe un agente con slug "support-agent"
Y tengo un paso domain_agent_run con `agent_slug: "support-agent"`
Cuando el flow ejecuta el paso domain_agent_run
Entonces el sistema lanza una ejecución de agente con los parámetros del paso
Y espera a que el agente complete
Y el resultado del agente se almacena en `steps.s8.result.agent_run_id`

Dado que el agente falla
Entonces el paso domain_agent_run se marca como fallido con el error del agente
```

### Escenario 9: sub_flow ejecuta otro flow como paso

```gherkin
Dado que existe un flow con slug "email-notification"
Y tengo un paso sub_flow con `flow_slug: "email-notification"`
Cuando el flow ejecuta el paso sub_flow
Entonces el sistema lanza una ejecución del flow referenciado
Y pasa los parámetros definidos como input del sub-flow
Y espera a que el sub-flow complete
Y el resultado del sub-flow se almacena en `steps.s9.result.flow_run_id`
```

### Escenario 10: transform muta datos entre pasos

```gherkin
Dado que tengo un paso transform con:
  """
  "expression": "$.users[?(@.active==true)].email",
  "engine": "jsonpath"
  """
Y el contexto contiene `steps.s1.result.users` con un array de usuarios
Cuando el flow ejecuta el paso transform
Entonces evalúa la expresión JSONPath sobre el contexto actual
Y el resultado (array de emails) se almacena en `steps.s10.result`

Dado que uso `engine: "jq"` con `expression: ".items | map(select(.price > 10))"`
Y el contexto contiene `items`
Entonces evalúa la expresión jq y almacena el resultado filtrado
```

### Escenario 11: Validación de schema por step type

```gherkin
Dado que creo un flow con un paso skill_call sin el campo `params.skill_slug`
Cuando el sistema valida el flow
Entonces rechaza con error "skill_call requiere params.skill_slug"

Dado que creo un flow con un paso llm_call sin `params.prompt_template`
Entonces rechaza con error "llm_call requiere params.prompt_template"

Dado que creo un flow con un paso code_exec sin `params.script`
Entonces rechaza con error "code_exec requiere params.script"

Dado que creo un flow con un paso conditional sin `params.condition`
Entonces rechaza con error "conditional requiere params.condition"

Dado que creo un flow con un paso parallel sin `params.branches`
Entonces rechaza con error "parallel requiere params.branches"
```

## Análisis breve

- **Qué pide realmente:** Implementar los 10 tipos de paso como engines ejecutables, cada uno con su schema de parámetros, lógica de ejecución y manejo de resultados. Cada step type es un "plugin" que implementa una interfaz `StepRunner`.
- **Módulos sospechados:** `internal/flow/step_types/`, cada tipo en su archivo (`skill_call.go`, `llm_call.go`, etc.), interfaz `StepRunner` en `internal/flow/runner.go`
- **Riesgos / dependencias:** code_exec requiere sandbox (REQ-11.1). domain_agent_run depende de REQ-08. sub_flow depende de HU-09.5. llm_call depende de REQ-06.
- **Esfuerzo tentativo:** XL

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
