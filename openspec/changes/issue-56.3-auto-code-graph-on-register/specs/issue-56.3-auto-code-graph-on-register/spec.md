# issue-56.3 — Auto-encadenar code_graph al registrar proyecto

Al registrar un proyecto, el code_graph se construye sin intervención manual, de
modo que un proyecto recién conocido nunca queda con grafo stale ni requiere
correr `domain-code-graph.sh` a mano.

## Requisitos

### Requirement: el register señala que hay que construir el grafo
`domain_session_register` MUST devolver una señal `next_action` de tipo
`build_code_graph` cuando crea un proyecto nuevo, porque el server no tiene el
filesystem del cliente y no puede construir el grafo él mismo.

#### Scenario: proyecto recién registrado
- **Given** un slug que no existe todavía
- **When** se llama `domain_session_register`
- **Then** la respuesta incluye `next_action.kind = "build_code_graph"` con el `slug` y el `cwd`

#### Scenario: proyecto ya existente
- **Given** un slug que ya existe
- **When** se llama `domain_session_register`
- **Then** la respuesta es `known=true` y reutiliza el proyecto (sin duplicar)

### Requirement: el hook reconstruye el grafo cuando está stale
El hook `domain-session-start.sh` MUST disparar el build del grafo no solo
cuando está vacío (`total_nodes <= 3`) sino también cuando el bootstrap reporta
`code_graph.stale = true`.

#### Scenario: grafo stale se reconstruye
- **Given** un bootstrap con `code_graph.stale = true`
- **When** corre el hook SessionStart y `domain-code-graph.sh` + ast-grep están disponibles
- **Then** el hook ejecuta el build del grafo y re-lee `domain_code_graph`

#### Scenario: grafo al día no se reconstruye
- **Given** un bootstrap con `code_graph.stale = false` y `total_nodes > 3`
- **When** corre el hook SessionStart
- **Then** el hook NO dispara el build (evita trabajo innecesario)
