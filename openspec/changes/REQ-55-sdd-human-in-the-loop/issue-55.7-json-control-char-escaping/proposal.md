# Como cliente del MCP, todos los tool results de domain son JSON válido parseable sin trucos (strict=False), porque el servidor escapa los caracteres de control dentro de los strings.

## Why
Síntoma RECURRENTE toda la sesión 2026-07-03: `json.loads` del lado cliente falla con
"Invalid control character at line 1 column N" al parsear tool results de domain
(mem_save, orchestrate_phase_result, flow_status, etc.). Rompió múltiples verificaciones
y forzó workarounds (strict=False, payload vía archivo). Detectado por el usuario como
punto a atacar.

## Causa probable
El server serializa strings con saltos de línea/tabs CRUDOS (0x0A, 0x09) dentro del campo
`text` del content JSON-RPC, en vez de escaparlos (\n, \t). json.Marshal de Go escapa
correcto por default → el bug está donde se construye el content text A MANO (concatenación
de strings sin pasar por Marshal).

## Scope
- Auditar dónde domain construye el `text` de los tool results sin json.Marshal.
- Garantizar que TODO tool result pase por serialización que escape control chars.
- Test: un tool que devuelve contenido con \n/\t crudos → el JSON de salida es parseable
  con json.loads estricto.

## Risks
- Bajo: es escapeo de serialización, no cambia semántica. Riesgo si algún cliente
  dependía del comportamiento roto (improbable).

## Testing
Test Go: tool result con string multilínea → output JSON válido estricto.
