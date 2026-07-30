package mcpserver

import (
	"encoding/json"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
)

// Red de última instancia: la proyección por tool (ver inline_slim_dto.go y
// ticket_slim_dto.go) es la defensa real contra el derrame de cuerpo. Este límite existe
// para el caso que la proyección no previó, y es generoso a propósito: un payload que
// llega acá ya es una anomalía.
const maxResultBytesPorDefecto = 256 * 1024

// El preview no se dimensiona contra maxBytes sino fijo y chico: sirve para diagnosticar
// qué tool derramó, no para entregar el dato.
const previewBytesDelEnvelope = 512

// El límite vive en el wrapper y NO en ToolBudget a propósito. ToolBudget se sobreescribe
// por tool (orchestrate_tools.go:535 y siguientes lo hacen sin este campo), así que
// ponerlo ahí dejaría esas tools en cero, o sea sin límite y sin aviso: un guard que se
// desactiva por omisión no es un guard.
func (r *ResilientWrapper) SetMaxResultBytes(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.maxResultBytes = n
}

func (r *ResilientWrapper) limiteDeResultado() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxResultBytes
}

// el corte va sobre el VALOR y el marshal después, así el encoder normaliza un multibyte
// partido en el borde; cortar la cadena ya serializada deja un escape a medias y el
// json.loads del hook SessionStart falla, degradando a skpol=degraded (DOMAINSERV-177)
func acotarResultado(toolName string, result *mcp.CallToolResult, maxBytes int) *mcp.CallToolResult {
	if result == nil || maxBytes <= 0 {
		return result
	}
	texto := toolResultText(result)
	if len(texto) <= maxBytes {
		return result
	}

	// El envelope preserva IsError, así que un truncado sobre un resultado exitoso sigue
	// contando como status ok en las métricas: sin esta línea, recortar un payload en
	// producción es indistinguible de una llamada normal. Se loguea el largo y NUNCA el
	// contenido, que es key bloqueada por la policy secrets-redaction.
	slog.Warn("tool result truncado por exceder el límite de bytes",
		"tool", toolName,
		"original_bytes", len(texto),
		"max_bytes", maxBytes,
	)

	envelope, err := json.Marshal(map[string]any{
		"truncated":      true,
		"original_bytes": len(texto),
		"max_bytes":      maxBytes,
		"preview":        truncate(texto, previewBytesDelEnvelope),
		"hint":           "el resultado excedió el límite de bytes del tool: pedí el detalle con la tool domain_*_get correspondiente, o acotá el limit",
	})
	if err != nil {
		return mcp.NewToolResultError("resultado truncado por exceder el límite de bytes")
	}

	acotado := mcp.NewToolResultText(string(envelope))
	acotado.IsError = result.IsError
	return acotado
}
