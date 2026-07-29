# issue-54.6 tools-coverage-matrix — Spec

Como plataforma domain, cada tool queda asignada a exactamente un canal de
invocación auditable, con las 10 fases del SDD con contrato y prep poblados,
de modo que la cobertura 100% sea verificable.

## ADDED Requirements

### Requirement: toda tool tiene canal
Cada tool expuesta por el registry del servidor DEBE tener exactamente un
canal primario asignado en la matriz (HOOK | FIRST_RESPONSE | PHASE_CONTRACT |
PHASE_PREP | POLICY_TRIGGERED | USER_INTENT), verificado por un test que
recorre el registry vivo.

#### Scenario: cero huérfanas
- **GIVEN** el registry completo de tools del servidor
- **WHEN** corre TestAllToolsHaveChannel
- **THEN** toda tool del registry aparece en la matriz
- **AND** el test falla si alguna no tiene canal

#### Scenario: tool nueva sin canal rompe CI
- **GIVEN** un desarrollador agrega una tool nueva al servidor
- **WHEN** no la clasifica en la matriz
- **THEN** el test falla en CI con el nombre de la tool faltante

### Requirement: manual-por-diseño es una decisión, no una omisión
Las tools de canal USER_INTENT (CRUD administrativo, deletes, confirmaciones
humanas) DEBEN estar registradas como manuales deliberadas; automatizarlas se
considera regresión.

#### Scenario: deletes nunca se auto-invocan
- **GIVEN** la tool domain_ticket_delete
- **WHEN** se consulta la matriz
- **THEN** su canal es USER_INTENT
- **AND** ningún hook, contrato de fase ni prep la referencia

### Requirement: las 10 fases con contrato y prep poblados
Cada fase del pipeline SDD DEBE declarar su `required_tool_calls` y su entrada
de preparación de contexto — vacío solo de forma EXPLÍCITA y con razón
documentada.

#### Scenario: activación total del contrato
- **GIVEN** el catálogo seedeado de agent_templates
- **WHEN** se inspeccionan las 10 fases
- **THEN** cada una tiene required_tool_calls definido (lista o vacío explícito)
- **AND** cada una tiene entrada en el mapeo de preparación de contexto

### Requirement: matriz consultable por agentes
La matriz DEBE existir como knowledge doc en la BD, generado desde la fuente
en código (nunca editado a mano), consultable vía domain_knowledge_search.

#### Scenario: un agente consulta el canal de una tool
- **GIVEN** la matriz persistida como knowledge doc
- **WHEN** un agente busca "canal de domain_mem_save"
- **THEN** obtiene el canal y el criterio de asignación
