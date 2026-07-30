package mcpserver

import (
	"encoding/json"

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

// acotarResultado reemplaza un resultado que excede el límite por un envelope JSON
// válido. El corte se aplica al VALOR y el marshal viene después: así un carácter
// multibyte partido en el borde lo normaliza el encoder. Cortar la cadena ya serializada
// dejaría un escape a medias, y el hook SessionStart parsea con json.loads — un JSON
// inválido lo degrada a skpol=degraded y dispara la re-llamada de project_skill_list y
// project_policy_list, que es la regresión de 28.206 tokens que eliminó DOMAINSERV-177.
// O sea: un guard mal hecho acá empeora el problema que viene a resolver.
func acotarResultado(result *mcp.CallToolResult, maxBytes int) *mcp.CallToolResult {
	if result == nil || maxBytes <= 0 {
		return result
	}
	texto := toolResultText(result)
	if len(texto) <= maxBytes {
		return result
	}

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
