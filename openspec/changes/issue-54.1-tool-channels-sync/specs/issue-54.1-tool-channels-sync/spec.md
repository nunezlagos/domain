# issue-54.1 — Sincronizar clasificación tool→canal

Corrige la clasificación de `domain_code_graph` (deprecada, ya no funciona),
sincroniza el doc `TOOL_CHANNELS.md` con el código, y agrega un test que impide
futuras desincronizaciones. El canal es metadata de auditoría + contrato de CI,
no se consume en runtime.

## Requisitos

### Requirement: domain_code_graph clasificada según su estado real
`domain_code_graph` MUST estar en `ChannelUserIntent` (manual, solo a pedido
explícito), NO en `ChannelHook`. La herramienta fue deprecada (kill 007,
2026-07-07) y ya no se pre-carga por hook; etiquetarla como `ChannelHook`
declara un comportamiento automático que no existe. La línea MUST llevar un
comentario de deprecación.

#### Scenario: code_graph no está en el canal hook
- **Given** el map toolChannel en tool_channels.go
- **When** se consulta el canal de domain_code_graph
- **Then** es ChannelUserIntent, no ChannelHook

### Requirement: TOOL_CHANNELS.md refleja el map toolChannel
El doc `TOOL_CHANNELS.md` MUST listar cada tool en el canal que le asigna el map
`toolChannel`. En particular MUST incluir `domain_issue_list` y
`domain_flow_cancel` (hoy ausentes) en `user-intent`, y `domain_code_graph` MUST
figurar en `user-intent` (no en `hook`). Los conteos por canal del header MUST
coincidir con el map.

#### Scenario: el doc lista las tools hoy ausentes
- **Given** TOOL_CHANNELS.md
- **When** se buscan domain_issue_list y domain_flow_cancel
- **Then** ambas aparecen en la sección user-intent

#### Scenario: code_graph figura en user-intent en el doc
- **Given** TOOL_CHANNELS.md
- **When** se busca domain_code_graph
- **Then** aparece bajo user-intent, no bajo hook

### Requirement: un test detecta desincronización doc↔código
MUST existir un test que falle si `TOOL_CHANNELS.md` no refleja el map
`toolChannel` (tool faltante en el doc, tool en el canal equivocado, o conteos
del header que no cuadran). Así CI atrapa la desincronización que hoy pasó
inadvertida.

#### Scenario: el test falla si una tool falta en el doc
- **Given** una tool presente en el map toolChannel pero ausente del doc
- **When** corre el test de sincronización
- **Then** el test falla señalando la tool faltante
