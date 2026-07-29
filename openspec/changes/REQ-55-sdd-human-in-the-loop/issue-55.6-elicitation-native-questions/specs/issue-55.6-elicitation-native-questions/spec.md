# issue-55.6 elicitation-native-questions — Spec

Como usuario del SDD, el servidor fuerza la pregunta con un diálogo nativo en spec/design,
imposible de saltear, garantizando que jamás se responda solo ante dudas.

## ADDED Requirements

### Requirement: server soporta elicitation
El servidor MCP DEBE declarar la capability elicitation y correr en modo stateful para
poder originar requests server→cliente.

#### Scenario: capability declarada
- **GIVEN** el server iniciado con WithElicitation() y WithStateLess(false)
- **WHEN** un cliente hace initialize
- **THEN** la respuesta incluye la capability elicitation

### Requirement: la fase spec fuerza la pregunta ante dudas
Cuando la fase spec tiene ambigüedades, el server DEBE disparar elicitation (no depender
de que el modelo llame AskUserQuestion), y el flujo DEBE bloquear hasta la respuesta.

#### Scenario: elicitation nativa
- **GIVEN** la fase spec con una duda y un cliente que soporta elicitation
- **WHEN** se procesa la fase
- **THEN** el server dispara elicitation/create con opciones + texto libre
- **AND** el flujo espera la respuesta del usuario antes de continuar

### Requirement: fallback para clientes sin elicitation
Si la sesión no soporta elicitation, el sistema DEBE caer al gate de evidencia: el
phase_result de spec debe incluir user_answers o el server rechaza el cierre.

#### Scenario: cliente sin soporte
- **GIVEN** una sesión que no soporta elicitation (ErrElicitationNotSupported)
- **WHEN** la fase spec intenta cerrar sin user_answers
- **THEN** el server rechaza el cierre (missing: user_answers), el flujo se detiene

### Requirement: aislamiento multi-usuario
Las elicitations de usuarios concurrentes DEBEN ir cada una a su sesión, sin cross-talk.

#### Scenario: 2 usuarios en el mismo proyecto
- **GIVEN** dos usuarios en el mismo proyecto, cada uno en su sesión MCP
- **WHEN** ambos disparan elicitation
- **THEN** cada respuesta vuelve solo a la sesión que la originó (por Mcp-Session-Id)
