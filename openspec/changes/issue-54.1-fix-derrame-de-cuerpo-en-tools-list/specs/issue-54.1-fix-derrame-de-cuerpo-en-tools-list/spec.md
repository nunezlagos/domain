# Acotar el cuerpo por item en los tools MCP de listado y búsqueda

## ADDED Requirements

### MUST-1 — Ninguna de las 9 tools devuelve el cuerpo completo por item

#### Scenario: un listado de skills no incluye el Content
- **Given** una skill persistida con un `Content` de 5000 caracteres
- **When** se invoca `domain_skill_list`
- **Then** el JSON de respuesta NO contiene la clave `Content` ni la subcadena del cuerpo
- **And** contiene `ContentLen` con el valor 5000

#### Scenario: el listado preserva los campos de encadenamiento
- **Given** la misma skill
- **When** se invoca `domain_skill_list`
- **Then** el JSON conserva `ID` y `Slug`, de modo que sigue siendo posible pedir el detalle

### MUST-2 — Las tres tools de búsqueda conservan snippet acotado, nunca cero cuerpo

#### Scenario: mem_search devuelve snippet de 200 y su longitud
- **Given** una observación con un `content` de 5000 caracteres
- **When** se invoca `domain_mem_search`
- **Then** la clave `content` sigue presente con como máximo 200 caracteres de texto
- **And** aparece `content_len` con el valor 5000
- **And** el nombre de la clave `content` NO cambia, porque el hook la lee por ese nombre exacto

#### Scenario: mem_context aplica el mismo shape que mem_search
- **Given** 20 observaciones de 5000 caracteres cada una
- **When** se invoca `domain_mem_context`
- **Then** cada item expone el mismo par `content` de hasta 200 caracteres más `content_len`

### MUST-3 — El guard de bytes emite siempre JSON válido

#### Scenario: un resultado que excede el límite se trunca sin romper el parseo
- **Given** un handler que devuelve un payload por encima del límite de bytes
- **When** el resultado pasa por `ResilientWrapper.Wrap`
- **Then** la respuesta es JSON parseable sin excepción
- **And** incluye `truncated` en true

#### Scenario: un cuerpo hostil no puede degradar el bootstrap de todas las sesiones
- **Given** un atacante que logra persistir una observación cuyo contenido corta un escape unicode justo en el límite de bytes
- **When** el guard trunca ese payload y el hook SessionStart lo parsea
- **Then** el guard NO emite JSON inválido, por lo que el token NO cae a `skpol=degraded`
- **And** el agente no re-llama `project_skill_list` ni `project_policy_list`

### MUST-4 — El gate sdd-review sigue bloqueando ante una violación real

#### Scenario: el review evalúa contra el cuerpo real de cada policy
- **Given** que `domain_project_policy_list` ya no devuelve `body_md`
- **When** corre la fase `sdd-review`
- **Then** la fase invoca `domain_policy_get` por cada slug aplicable
- **And** `domain_policy_get` figura en `RequiredToolCalls`

#### Scenario: un diff que viola una policy sale en rojo, verificado por ejecución
- **Given** un diff que viola una policy conocida
- **When** corre la fase `sdd-review` con el shape nuevo
- **Then** el verdict es `violations_found` y la validación bloquea el avance a archive

#### Scenario: el prompt nuevo llega efectivamente a la base
- **Given** una edición del system_prompt seedeado de `sdd-review`
- **When** se aplica el cambio
- **Then** `agentTemplatesSeedVersion` pasa de 20 a 21 en el mismo commit
- **And** el seeder no se skippea, de modo que el prompt viejo deja de gobernar

### MUST-5 — El hook de install-user sigue inyectando memorias

#### Scenario: el hook encuentra el texto donde lo espera
- **Given** el shape nuevo de `domain_mem_search`
- **When** corre el hook `domain-user-prompt.sh`
- **Then** el bloque de parseo sigue obteniendo texto y la lista de items NO queda vacía

#### Scenario: el cambio de shape y el hook viajan juntos
- **Given** que el CI filtra por `services/domain-mcp/**` y no cubre `install-user/`
- **When** se commitea el cambio de shape
- **Then** el hook y el server van en el mismo commit
- **And** ambas suites se corren a mano

### MUST-6 — Un linter impide que reaparezcan resultados armados a mano

#### Scenario: el linter detecta un CallToolResult manual nuevo
- **Given** un archivo de `internal/mcp/server` que construye el resultado directamente en vez de usar el helper
- **When** corre el linter en CI
- **Then** falla con exit distinto de cero y reporta el archivo y la línea

#### Scenario: el guard arranca sin deuda tolerada
- **Given** los 17 sitios manuales existentes
- **When** se integra el linter
- **Then** los 17 ya fueron normalizados y el linter reporta cero violaciones sin baseline de excepciones

### MUST-7 — La re-hidratación post-compactación recupera el resumen completo

#### Scenario: context-preservation no se conforma con el snippet
- **Given** un `session_summary` de más de 200 caracteres
- **When** un agente re-hidrata siguiendo la policy `context-preservation`
- **Then** la policy indica pedir la observación completa con `domain_mem_get_observation` usando el id del resultado
- **And** el cambio de la policy viaja con el bump del seeder de platform policies
