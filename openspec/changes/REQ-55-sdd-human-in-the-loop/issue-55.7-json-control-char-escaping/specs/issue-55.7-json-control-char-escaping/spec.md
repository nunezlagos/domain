# issue-55.7 json-control-char-escaping — Spec

Como cliente del MCP, todos los tool results de domain son JSON válido parseable sin
strict=False.

## ADDED Requirements

### Requirement: tool results con control chars escapados
Todo tool result de domain DEBE ser JSON válido: los caracteres de control (0x0A, 0x09,
etc.) dentro de strings DEBEN venir escapados.

#### Scenario: contenido multilínea se serializa válido
- **GIVEN** un tool cuyo resultado contiene saltos de línea y tabs
- **WHEN** el server devuelve el tool result
- **THEN** el JSON es parseable con json.loads estricto (sin strict=False)

#### Scenario: sin regresión de parseo
- **GIVEN** los tools que fallaban en la sesión (mem_save, phase_result, flow_status)
- **WHEN** un cliente parsea sus respuestas
- **THEN** ninguna lanza "Invalid control character"
