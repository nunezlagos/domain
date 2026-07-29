# issue-55.7 json-control-char-escaping — Design

## Decisions
- D1. Localizar toda construcción de tool result que NO use json.Marshal (concatenación
  manual de JSON, fmt.Sprintf de estructuras). Reemplazar por marshalling real.
- D2. Preferir el helper toolResultJSON / mcp.NewToolResultText de la lib, que serializan
  correcto, sobre string-building manual.
- D3. Test de regresión: contenido con \n, \t, comillas, backslash → parseable estricto.

## Observado en la sesión (evidencia)
Fallos repetidos de "Invalid control character at line 1 column N" al parsear respuestas
de mem_save, orchestrate_phase_result, flow_status, issue_create_answer. Workaround:
json.loads(..., strict=False). El fix elimina la necesidad del workaround.

## Alternatives
- A1. Documentar "usá strict=False siempre": parche, no fix. Descartado.
