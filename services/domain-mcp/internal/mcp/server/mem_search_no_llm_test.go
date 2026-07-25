package mcpserver

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-116: mem_search no puede depender de una llamada a LLM. La policy
// llm-nunca-en-camino-caliente es dura y una búsqueda SIEMPRE tiene a alguien
// esperando, así que el reordenamiento lo hace el cliente con su propio criterio
// —tiene el contexto de la tarea, que el server no— sobre los scores que el
// server sí puede calcular solo (BM25 + vector + RRF).
//
// El contrato observable es el schema del tool: mientras ofrezca `rerank`, un
// cliente puede pedir inferencia en el camino caliente.
func TestToolMemSearch_Schema_NoOfreceParametrosDeRerank(t *testing.T) {
	raw, err := json.Marshal(toolMemSearch().InputSchema)
	require.NoError(t, err)
	schema := string(raw)

	require.NotContains(t, schema, "rerank",
		"el schema no debe ofrecer rerank: habilita una llamada a LLM en el camino de búsqueda")
	require.NotContains(t, schema, "rerank_top_n",
		"rerank_top_n solo tenía sentido para acotar el prompt del LLM")
}

// Los scores son lo que reemplaza al rerank: el server los calcula sin inferencia
// y el cliente decide con ellos. Si dejaran de viajar, sacar el rerank sería una
// pérdida neta de información en vez de un movimiento de responsabilidad.
func TestToolMemSearch_Schema_SigueAceptandoLimitConfigurable(t *testing.T) {
	raw, err := json.Marshal(toolMemSearch().InputSchema)
	require.NoError(t, err)

	require.Contains(t, string(raw), "limit",
		"el cliente necesita pedir más candidatos de los que va a usar para poder reordenar")
}
