# issue-54.1 — Sincronizar clasificación tool→canal

## Why

Estado verificado contra el código: la clasificación tool→canal está casi toda
bien, pero quedan 2 inconsistencias:

- `domain_code_graph` (tool_channels.go:59) está en `ChannelHook` (se pre-carga
  en SessionStart), pero code_graph fue deprecado el 2026-07-07 (kill 007,
  commit 4d3c134c) y **ya no funciona**. La etiqueta declara un comportamiento
  automático inexistente.
- `TOOL_CHANNELS.md` se declara "GENERADO desde tool_channels.go — NO editar a
  mano", pero **no existe generador** (sin go:generate, Makefile ni script,
  verificado). Está desincronizado: le faltan `domain_issue_list` y
  `domain_flow_cancel` (ambas `ChannelUserIntent`).

El canal es metadata de auditoría + contrato de CI, no se consume en runtime, así
que el riesgo es bajo — pero una etiqueta falsa sobre una tool muerta confunde a
quien lee la clasificación, y el doc ya venía mintiendo sobre su contenido.

## Scope

Entra: `tool_channels.go` (reclasificar code_graph), `TOOL_CHANNELS.md`
(sincronizar a mano), `tool_channels_test.go` (test de sincronización).

Fuera: reclasificar otras tools (ya están bien); crear un generador real del doc
(el test de sincronización alcanza para el objetivo).

## Approach

- Cambiar `domain_code_graph` de `ChannelHook` a `ChannelUserIntent` + comentario
  de deprecación.
- Actualizar `TOOL_CHANNELS.md` consistente con el map: mover code_graph a
  user-intent, agregar issue_list y flow_cancel, actualizar conteos (hook 5→4,
  user-intent 95→98).
- Test nuevo que parsea el doc y lo compara con el map `toolChannel` (tools y
  canales), fallando ante cualquier divergencia.

## Risks

- El test de sincronización debe parsear el doc de forma robusta (secciones por
  canal). Mitigación: parseo simple por encabezado `## <canal> (N)` + líneas
  `` - `tool` ``.
- Bajo riesgo funcional: nada de esto cambia el comportamiento del server.

## Testing

TDD: el cambio de code_graph lo cubren los tests existentes
(TestAllToolsHaveChannel, TestChannelDistribution). El test de sincronización es
nuevo. `cd services/domain-mcp && go test ./internal/mcp/server/` + go vet.
