# HU-05.6-agent-skill-contract

**Origen:** `REQ-05-skill-system`
**Persona:** dx-engineer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer creando skills
**Quiero** un contrato formal entre Agent y Skill (JSON schema de inputs/outputs, semántica de errores, tool-calling format)
**Para** que el LLM pueda invocar skills correctamente y los errores se propaguen de forma uniforme

## Criterios de aceptación

### Escenario 1: Skill declara JSON schema

```gherkin
Dado que existe skill `summarize-text` con definition:
  ```yaml
  input_schema:
    type: object
    properties:
      text: { type: string, minLength: 1, maxLength: 50000 }
      max_words: { type: integer, default: 200, minimum: 50, maximum: 2000 }
    required: [text]
  output_schema:
    type: object
    properties:
      summary: { type: string }
      word_count: { type: integer }
    required: [summary, word_count]
  ```
Entonces el skill registry valida el schema (draft 2020-12)
Y rechaza skills con schemas inválidos
```

### Escenario 2: Tool-calling format compatible Anthropic + OpenAI

```gherkin
Dado que un agent con provider=anthropic carga sus skills
Cuando el motor de ejecución prepara la llamada LLM
Entonces se traduce cada skill a Anthropic tool format:
  `{name, description, input_schema}`
Y para provider=openai se traduce a OpenAI function calling format
Y para gemini al format equivalente
Y la traducción está cubierta por tests
```

### Escenario 3: Validación de inputs antes de execute

```gherkin
Dado que el LLM emite tool_use con args `{"text": "..."}` (faltó `max_words`)
Cuando el motor invoca la skill
Entonces se aplican defaults del schema (`max_words=200`)
Y luego se valida con JSON Schema validator
Y si después de defaults sigue faltando un required → error tipado "InvalidInput" sin ejecutar la skill
```

### Escenario 4: Output validation

```gherkin
Dado que la skill ejecuta y devuelve `{"summary": "...", "word_count": "not-an-int"}`
Cuando el motor recibe el output
Entonces se valida contra output_schema
Y si falla → error "InvalidOutput" + se registra incidente
Y el agent recibe un tool_result con error explícito
```

### Escenario 5: Errores tipados propagados al agente

```gherkin
Dado que una skill lanza error
Cuando el motor procesa
Entonces se mapea a una de las categorías:
  | code              | semántica                              | retry |
  | InvalidInput      | args no validan schema                 | no    |
  | InvalidOutput     | result no valida schema                | no    |
  | NotAuthorized     | RBAC denied                            | no    |
  | NotFound          | recurso target inexistente             | no    |
  | RateLimited       | external API throttle                  | sí (backoff) |
  | Timeout           | execution > skill.timeout_seconds      | sí (1x) |
  | DependencyFailed  | upstream caído                         | sí (backoff) |
  | InternalError     | bug en el skill                        | no    |
Y el tool_result al LLM incluye `{error_code, message}` para que el modelo decida
```

### Escenario 6: Idempotency hint

```gherkin
Dado que skill declara `idempotent: true` en su manifest
Cuando el motor reintenta (timeout/dep failed)
Entonces el retry se permite con la MISMA llamada
Y si `idempotent: false` el motor NO reintenta y devuelve error al agente
```

### Escenario 7: Skill puede invocar otra skill

```gherkin
Dado que skill `A` declara `depends_on: [B]`
Cuando A se ejecuta
Entonces el contexto de ejecución expone un client `ctx.InvokeSkill("B", input)`
Y la invocación interna respeta el mismo contrato (validate input/output + errores tipados)
Y se logea cadena de invocación para tracing
```

### Escenario 8: Timeout por skill

```gherkin
Dado que skill define `timeout_seconds: 30`
Cuando la ejecución supera 30s
Entonces se cancela con `Timeout` error
Y el goroutine de la skill recibe context cancellation
```

## Análisis breve

- **Qué pide:** schema validate (input/output) + translation tool-calling format + error taxonomy + timeout + idempotency + skill-to-skill calls
- **Esfuerzo:** M
- **Riesgos:** schemas inválidos en registry; mistranslate provider formats; leaks de errores con info sensible
