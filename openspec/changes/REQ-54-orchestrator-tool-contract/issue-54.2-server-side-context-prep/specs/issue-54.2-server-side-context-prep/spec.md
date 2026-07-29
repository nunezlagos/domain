# issue-54.2 server-side context prep — Spec

Como orquestador SDD, al iniciar cada fase preparo server-side el contexto read-only barato
y se lo entrego al cliente ya armado, de modo que las tools de consulta dejen de ser
huérfanas y el cliente reciba la fuente de verdad sin tener que pedirla.

## ADDED Requirements

### Requirement: preparación de contexto por fase
El sistema DEBE, antes de entregar el prompt de una fase al cliente, ejecutar server-side
las tools read-only declaradas por esa fase (`PrepToolCalls`) y agregar el resultado como
`prepared_context` al contexto de la fase.

#### Scenario: la fase corre solo sus tools de preparación
- **GIVEN** la fase `sdd-apply` con `PrepToolCalls = ["policy_list", "project_skill_list"]`
- **WHEN** el orquestador prepara el contexto de la fase
- **THEN** `prepared_context` contiene las policies y skills del proyecto desde la BD
- **AND** no se ejecutan tools de otras fases

### Requirement: refinamiento inteligente opcional con Minimax
Cuando haya un LLM (Minimax) disponible en el servidor, el sistema PUEDE refinar el contexto
crudo (filtrar/resumir lo pertinente a la fase) antes de entregarlo.

#### Scenario: Minimax filtra lo pertinente
- **GIVEN** un contexto crudo con 19 policies y Minimax disponible
- **WHEN** el orquestador refina el contexto para `sdd-apply`
- **THEN** `prepared_context` contiene el subconjunto de policies pertinentes a la fase

### Requirement: degradación elegante
La preparación NUNCA DEBE bloquear una fase. Si Minimax no está disponible o excede el
timeout, el sistema DEBE entregar el contexto crudo; si toda la preparación falla, la fase
DEBE correr como si no hubiera `prepared_context`.

#### Scenario: sin Minimax, entrega contexto crudo
- **GIVEN** el servidor sin key de Minimax
- **WHEN** el orquestador prepara el contexto de `sdd-apply`
- **THEN** `prepared_context` es el listado crudo de policies/skills
- **AND** la fase no reporta error

#### Scenario: timeout de Minimax degrada a crudo
- **GIVEN** Minimax disponible pero que excede el timeout de preparación
- **WHEN** el orquestador prepara el contexto
- **THEN** `prepared_context` es el contexto crudo
- **AND** el step no queda bloqueado por la preparación
