# issue-65.1 — Design

## Decisions

- **Merge de Continue por url, no por índice.** `configureContinue` identifica la
  entrada de domain buscando la que tenga `transport.url` terminado en `/mcp`
  hacia el VPS. Update si la encuentra, append si no. Robusto ante reordenamientos.
- **backupIfExists reutilizado.** Las dos rutas sin backup (uninstall continue,
  remove-engram) usan el helper existente `backupIfExists(path, Timestamp())`, ya
  usado en el resto del installer. Consistencia, no un mecanismo nuevo.
- **Templates: marcar el path, no fusionar protocolos.** No se unifican los dos
  protocolos (Claude con hooks vs OpenCode sin hooks) — eso sería un rediseño. Se
  marca en cada template qué fila de la tabla Tool paths aplica a ese cliente, que
  es el fix mínimo que elimina la contradicción.
- **Conservar la nota de deprecación.** Se quita la advertencia del script fantasma
  `domain-code-graph.sh` pero se mantiene "domain_code_* deprecado" (eso es cierto
  y útil).

## Alternatives

- **Bug — match por índice 0**: descartado, frágil si el orden cambia. Match por url.
- **Doc — reescribir un template único compartido**: descartado por scope; es un
  rediseño. Se marca el path por cliente.
- **Ruido — dejar personality.md**: descartado; 0 referencias, duplicado inline.

## Data Flow

configureContinue: load config.json → leer experimental.modelContextProtocolServers
(o []) → buscar entrada domain por url /mcp → update-or-append → preservar resto →
backup → write.

## TDD Plan

- **Red**: test con config.json de Continue que tiene 2 servers ajenos + espera que
  tras configurar queden los 2 + domain (hoy falla: quedaría solo domain).
- **Green**: merge por url.
- **Red**: test de que re-configurar no duplica.
- **Sabotaje**: volver a `= []any{...}` (reemplazo) → el test de "conserva otros" falla.
- Doc/ruido: verificación por grep (no test Go): grep de domain_orchestrate_status
  (debe dar 0), de domain-code-graph.sh (0), de personality.md (no existe), de `-m`
  en el curl de bootstrap.sh (presente).

## Risk Mitigation

- Match por url estable.
- backupIfExists reutilizado (idempotente, ya testeado).
- Cambios de doc mínimos y quirúrgicos.
