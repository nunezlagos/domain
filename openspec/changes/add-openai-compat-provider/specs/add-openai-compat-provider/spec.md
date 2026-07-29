# add-openai-compat-provider — Spec

## ADDED Requirements

### Requirement: base_url override en el provider OpenAI
El sistema MUST exponer `openai.NewWithBaseURL(apiKey, baseURL, model)` que reusa `Complete`/`CompleteStream` contra un `base_url` arbitrario, defaulteando a las constantes cuando los parámetros vienen vacíos.

#### Scenario: Complete contra base_url custom
- **Given** un `base_url` que apunta a un `httptest` server OpenAI-compatible
- **When** se llama `Complete`
- **Then** la request HTTP viaja a ese `base_url` y devuelve la `Response` mapeada desde `choices[0].message.content`

### Requirement: registro dinámico de N providers desde config
El sistema MUST registrar 1..N providers OpenAI-compatibles desde configuración, cada uno con nombre propio, sin código nuevo por endpoint.

#### Scenario: dos endpoints OpenAI-compatibles
- **Given** config con 2 endpoints `{name, base_url, api_key_env, model}`
- **When** arranca `buildLLMFactory`
- **Then** `Factory.Get(name)` devuelve cada provider registrado

### Requirement: no romper providers existentes
El sistema MUST seguir registrando y sirviendo anthropic/openai/google/ollama/minimax sin cambios de comportamiento.

#### Scenario: providers previos intactos
- **Given** las envs actuales de los providers existentes
- **When** arranca el factory con el nuevo wiring
- **Then** todos los providers previos siguen resolviendo por `Factory.Get`

### Requirement: secreto no expuesto
El sistema MUST no loguear ni exponer el `api_key` en logs, métricas ni mensajes de error.

#### Scenario: abuse-case fuga de api_key
- **Given** un atacante con acceso a logs/métricas del server
- **When** se registra y usa un provider OpenAI-compat con `api_key` configurado
- **Then** el `api_key` no aparece en ningún log, métrica ni error emitido
