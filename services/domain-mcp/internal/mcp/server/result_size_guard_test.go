package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

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

	require.Same(t, grande, acotarResultado("tool_test", grande, 0), "maxBytes 0 significa sin límite")
	require.Same(t, grande, acotarResultado("tool_test", grande, -1))
	require.Nil(t, acotarResultado("tool_test", nil, 100))
}

// un resultado de error no se convierte en envelope de éxito: el breaker y las métricas
// leen IsError río abajo (resilience.go:347 y :369)
func TestAcotarResultado_ResultadoDeError_PreservaIsError(t *testing.T) {
	errResult := mcp.NewToolResultError(strings.Repeat("z", 5000))

	out := acotarResultado("tool_test", errResult, 200)
	require.True(t, out.IsError)
	require.Contains(t, toolResultText(out), "truncated")
}

func TestNewResilientWrapper_SinConfigurar_TieneLimitePorDefecto(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{})
	require.Equal(t, maxResultBytesPorDefecto, r.maxResultBytes,
		"un guard sin límite por defecto no es un guard: las 165 tools quedarían sin cubrir")
}

type cacheFalso struct {
	guardado map[string][]byte
}

func (c *cacheFalso) Get(key string) ([]byte, bool) { v, ok := c.guardado[key]; return v, ok }
func (c *cacheFalso) Set(key string, value []byte, ttl time.Duration) {
	if c.guardado == nil {
		c.guardado = map[string][]byte{}
	}
	c.guardado[key] = value
}
func (c *cacheFalso) FlushPrefix(prefix string) int { return 0 }

// El punto de insercion del guard es ANTES del cache, y ese orden es la razon de ser de
// la linea elegida: si alguien la mueve despues del store.Set, el payload gordo queda
// viviendo cacheado y ningun otro test lo detecta.
func TestResilientWrapper_Wrap_GuardCorreAntesDelCache_LoCacheadoYaEstaAcotado(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{})
	r.SetMaxResultBytes(400)
	store := &cacheFalso{}
	r.SetCache(store)
	r.SetCacheable("tool_cacheable", time.Hour)
	r.SetOrgIDAccessor(func() string { return "org-de-prueba" })

	wrapped := r.Wrap("tool_cacheable", handlerQueDevuelve(strings.Repeat("g", 9000)))
	_, err := wrapped(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)

	require.Len(t, store.guardado, 1, "el resultado exitoso debe haberse cacheado")
	for _, crudo := range store.guardado {
		require.NotContains(t, string(crudo), strings.Repeat("g", 1000),
			"se cacheo el payload SIN acotar: el guard quedo despues del store.Set")
		require.Contains(t, string(crudo), "truncated")
	}
}

// Una entrada escrita antes de que el guard existiera sigue en el cache con su TTL: el
// hit tambien tiene que pasar por el guard o se sirve intacta.
func TestResilientWrapper_Wrap_CacheHitConPayloadViejo_TambienSeAcota(t *testing.T) {
	r := NewResilientWrapper(ToolBudget{})
	store := &cacheFalso{}
	r.SetCache(store)
	r.SetCacheable("tool_con_cache_viejo", time.Hour)
	r.SetOrgIDAccessor(func() string { return "org-de-prueba" })

	// se siembra con el guard desactivado, como si la entrada fuera anterior al cambio
	r.SetMaxResultBytes(0)
	sembrar := r.Wrap("tool_con_cache_viejo", handlerQueDevuelve(strings.Repeat("v", 9000)))
	_, err := sembrar(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.Len(t, store.guardado, 1)

	r.SetMaxResultBytes(400)
	out, err := sembrar(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)

	texto := toolResultText(out)
	require.NotContains(t, texto, strings.Repeat("v", 1000), "el hit sirvio el payload viejo sin acotar")
	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(texto), &envelope))
	require.Equal(t, true, envelope["truncated"])
}

func TestAcotarResultado_ContenidoVacioOMultiple_SeComportaPorLargoTotal(t *testing.T) {
	sinContenido := &mcp.CallToolResult{}
	require.Same(t, sinContenido, acotarResultado("t", sinContenido, 100), "sin contenido no hay nada que acotar")

	// toolResultText une los items con un espacio: el limite aplica al total, no a cada uno
	dosItems := &mcp.CallToolResult{Content: []mcp.Content{
		mcp.TextContent{Type: "text", Text: strings.Repeat("a", 60)},
		mcp.TextContent{Type: "text", Text: strings.Repeat("b", 60)},
	}}
	out := acotarResultado("t", dosItems, 100)
	require.NotSame(t, dosItems, out, "121 bytes sumados superan el limite de 100")

	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(toolResultText(out)), &envelope))
	require.EqualValues(t, 121, envelope["original_bytes"])
}

// truncate usa len(s) <= n, asi que el borde exacto NO lleva marcador
func TestTruncate_LargoExactoDelLimite_NoAgregaMarcador(t *testing.T) {
	exacto := strings.Repeat("e", snippetBytes)

	require.Equal(t, exacto, truncate(exacto, snippetBytes))
	require.NotContains(t, truncate(exacto, snippetBytes), marcadorDeTruncado)
	require.Contains(t, truncate(exacto+"x", snippetBytes), marcadorDeTruncado)
}
