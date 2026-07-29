# issue-54.1 — Design

## Decisions

- **code_graph → ChannelUserIntent** (no PolicyTriggered). code_graph ya no
  funciona; UserIntent = "manual solo a pedido explícito" es la etiqueta honesta
  para una tool en desuso, y alinea con `domain_code_build` que ya está ahí.
- **Doc a mano, no generador nuevo.** El header dice "generado" pero no hay
  generador. En vez de escribir uno (scope creep), se actualiza el doc a mano y
  se agrega un TEST que garantiza que no se vuelva a desincronizar. El test es la
  salvaguarda real; el "generado" del header se puede ajustar a "mantener
  sincronizado — validado por TestToolChannelsDocInSync".
- **Test parsea el doc, compara con el map.** La fuente de verdad es el map
  `toolChannel` en Go; el doc es la proyección. El test parsea las secciones del
  `.md` y verifica cobertura bidireccional (toda tool del map en el doc con su
  canal; ninguna tool en el doc que no esté en el map).

## Alternatives

- **Escribir un generador real (go:generate)**: descartado por scope — el
  objetivo es que no se desincronice, y un test lo logra sin el peso de un
  generador que también hay que mantener.
- **code_graph a PolicyTriggered** (junto a las otras code_*): descartado — esas
  son "parte del protocolo code graph" que también está muerto, pero al menos no
  afirman ejecución automática. UserIntent es más honesto para una tool en desuso.

## Data Flow

map toolChannel (fuente de verdad) → TOOL_CHANNELS.md (proyección, a mano) →
TestToolChannelsDocInSync valida que la proyección refleja la fuente.

## TDD Plan

- **Red**: TestToolChannelsDocInSync — parsea el doc actual y compara con el map;
  hoy falla (faltan issue_list, flow_cancel; code_graph en hook).
- **Green**: actualizar el doc a mano hasta que el test pase.
- **Red/Green code_graph**: el cambio de canal lo cubren los tests existentes
  (siguen verdes: hook baja a 4, no vacío; user-intent sube).
- **Sabotaje**: quitar una tool del doc → el test de sincronización falla.

## Risk Mitigation

- Parseo del doc simple y tolerante (por encabezado de sección + líneas de tool).
- Nada cambia en runtime (metadata + CI).
