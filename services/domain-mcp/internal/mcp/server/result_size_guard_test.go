package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func resultadoDeTexto(texto string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: texto}}}
}

func handlerQueDevuelve(texto string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return resultadoDeTexto(texto), nil
	}
}

func TestResilientWrapper_Wrap_PayloadSobreLimite_DevuelveJSONValidoConTruncated(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{})
	r.SetMaxResultBytes(500)

	wrapped := r.Wrap("tool_gorda", handlerQueDevuelve(`{"datos":"`+strings.Repeat("x", 5000)+`"}`))
	out, err := wrapped(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.NotNil(t, out)

	texto := toolResultText(out)

	// el consumidor crítico es el hook SessionStart, que parsea con json.loads:
	// si el guard emite JSON inválido, degrada a skpol=degraded (DOMAINSERV-177)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(texto), &envelope), "el guard debe emitir JSON parseable")

	require.Equal(t, true, envelope["truncated"])
	// 10 bytes del prefijo {"datos":" + 5000 de relleno + 2 del cierre "}
	require.EqualValues(t, 5012, envelope["original_bytes"])
	require.EqualValues(t, 500, envelope["max_bytes"])
	require.Contains(t, texto, "domain_")
	require.NotContains(t, texto, strings.Repeat("x", 3000))
}

func TestResilientWrapper_Wrap_PayloadBajoLimite_PasaIntacto(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{})
	r.SetMaxResultBytes(500)

	wrapped := r.Wrap("tool_chica", handlerQueDevuelve(`{"ok":true}`))
	out, err := wrapped(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.Equal(t, `{"ok":true}`, toolResultText(out))
}

// El caso borde de seguridad del ADR-161.3: un contenido que ubica un carácter
// multibyte justo en el límite. El corte tiene que ocurrir ANTES de serializar, así el
// marshal normaliza el rune partido; cortar la cadena JSON ya serializada dejaría el
// escape a medias y el json.loads del hook fallaría.
func TestResilientWrapper_Wrap_MultibyteEnElBorde_NoRompeElParseo(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{})
	limite := 300
	r.SetMaxResultBytes(limite)

	for offset := -4; offset <= 4; offset++ {
		relleno := limite + offset
		if relleno < 1 {
			continue
		}
		// ñ ocupa 2 bytes: con estos offsets el corte cae dentro del carácter
		payload := strings.Repeat("ñ", relleno)

		wrapped := r.Wrap("tool_utf8", handlerQueDevuelve(payload))
		out, err := wrapped(context.Background(), mcp.CallToolRequest{})
		require.NoError(t, err)

		var envelope map[string]any
		require.NoErrorf(t, json.Unmarshal([]byte(toolResultText(out)), &envelope),
			"offset %d: el guard emitió JSON inválido", offset)
		require.Equal(t, true, envelope["truncated"], "offset %d", offset)
	}
}

func TestAcotarResultado_SinLimiteONil_NoToca(t *testing.T) {
	grande := resultadoDeTexto(strings.Repeat("y", 10000))

	require.Same(t, grande, acotarResultado(grande, 0), "maxBytes 0 significa sin límite")
	require.Same(t, grande, acotarResultado(grande, -1))
	require.Nil(t, acotarResultado(nil, 100))
}

// un resultado de error no se convierte en envelope de éxito: el breaker y las métricas
// leen IsError río abajo (resilience.go:347 y :369)
func TestAcotarResultado_ResultadoDeError_PreservaIsError(t *testing.T) {
	errResult := mcp.NewToolResultError(strings.Repeat("z", 5000))

	out := acotarResultado(errResult, 200)
	require.True(t, out.IsError)
	require.Contains(t, toolResultText(out), "truncated")
}

func TestNewResilientWrapper_SinConfigurar_TieneLimitePorDefecto(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{})
	require.Equal(t, maxResultBytesPorDefecto, r.maxResultBytes,
		"un guard sin límite por defecto no es un guard: las 165 tools quedarían sin cubrir")
}
