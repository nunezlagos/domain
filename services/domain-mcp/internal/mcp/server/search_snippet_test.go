package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	knowledgesvc "nunezlagos/domain/internal/service/knowledge"
	obssvc "nunezlagos/domain/internal/service/observation"
)

// A diferencia de los listados, estas tres tools conservan cuerpo: cero texto rompe a
// cuatro consumidores en silencio. El snippet va bajo la MISMA key que hoy, porque el
// hook domain-user-prompt.sh lee r.get("content") y ya trunca a 160 por su cuenta.

func TestProyectarBusquedaDeObservaciones_ContentLargo_TruncaYReportaLargoReal(t *testing.T) {
	cuerpo := strings.Repeat("x", 5000)
	r := obssvc.SearchResult{
		Observation: obssvc.Observation{ID: uuid.New(), Content: cuerpo, ObservationType: "decision"},
		Score:       0.87,
		BM25Rank:    2,
	}

	items := proyectarBusquedaDeObservaciones([]obssvc.SearchResult{r})
	require.Len(t, items, 1)

	content, _ := items[0]["content"].(string)
	require.LessOrEqual(t, len(content), snippetBytes+len(marcadorDeTruncado))
	require.Greater(t, len(content), 0, "cero cuerpo no es una opción para mem_search")
	require.EqualValues(t, 5000, items[0]["content_len"])

	// el id tiene que seguir viajando: es lo que habilita el fan-out a mem_get_observation
	require.Equal(t, r.ID.String(), items[0]["id"])
	require.Equal(t, 0.87, items[0]["score"])
	require.Equal(t, 2, items[0]["bm25_rank"])
}

func TestProyectarBusquedaDeObservaciones_ContentCorto_NoTocaNiMarca(t *testing.T) {
	items := proyectarBusquedaDeObservaciones([]obssvc.SearchResult{
		{Observation: obssvc.Observation{Content: "algo corto"}},
	})

	require.Equal(t, "algo corto", items[0]["content"])
	require.NotContains(t, items[0]["content"], "truncated")
	require.EqualValues(t, 10, items[0]["content_len"])
}

// mem_context es el mayor emisor de cuerpo del sistema: el hook SessionStart lo inyecta
// en cada sesión con hasta 20 observaciones
func TestProyectarContextoDeObservaciones_MismoShapeQueBusqueda(t *testing.T) {
	cuerpo := strings.Repeat("y", 5000)
	obs := make([]obssvc.Observation, 20)
	for i := range obs {
		obs[i] = obssvc.Observation{ID: uuid.New(), Content: cuerpo, ObservationType: "note"}
	}

	items := proyectarContextoDeObservaciones(obs)
	require.Len(t, items, 20)

	for i, item := range items {
		content, _ := item["content"].(string)
		require.LessOrEqualf(t, len(content), snippetBytes+len(marcadorDeTruncado), "item %d", i)
		require.EqualValuesf(t, 5000, item["content_len"], "item %d", i)
		require.NotEmptyf(t, item["id"], "item %d", i)
	}

	// las dos tools no pueden divergir en el shape del mismo objeto
	busqueda := proyectarBusquedaDeObservaciones([]obssvc.SearchResult{
		{Observation: obssvc.Observation{ID: uuid.New(), Content: cuerpo}},
	})
	for _, clave := range []string{"id", "content", "content_len", "observation_type", "tags", "created_at"} {
		require.Containsf(t, items[0], clave, "mem_context no expone %s", clave)
		require.Containsf(t, busqueda[0], clave, "mem_search no expone %s", clave)
	}

	// el payload entero de una sesión típica tiene que bajar de forma medible
	b, err := json.Marshal(items)
	require.NoError(t, err)
	require.Less(t, len(b), 20*1000, "20 observaciones de 5000 chars no deben pasar de ~20 KB proyectadas")
}

func TestProyectarBusquedaDeKnowledge_SnippetEraElContentCompleto_AhoraSeAcota(t *testing.T) {
	cuerpo := strings.Repeat("z", 8000)
	r := knowledgesvc.SearchResult{
		DocumentID: uuid.New(),
		ChunkID:    uuid.New(),
		Title:      "Un documento",
		Snippet:    cuerpo,
		Score:      0.5,
	}

	items := proyectarBusquedaDeKnowledge([]knowledgesvc.SearchResult{r})
	require.Len(t, items, 1)

	snippet, _ := items[0]["snippet"].(string)
	require.LessOrEqual(t, len(snippet), snippetBytes+len(marcadorDeTruncado))
	require.EqualValues(t, 8000, items[0]["snippet_len"])
	require.Equal(t, r.DocumentID.String(), items[0]["document_id"])
	require.Equal(t, "Un documento", items[0]["title"])
}

// el 200 no es arbitrario: es el único precedente medido del repo, y es unánime en
// service/search, service/timeline y service/orchestrator
func TestSnippetBytes_CoincideConElPrecedenteDelRepo(t *testing.T) {
	require.Equal(t, 200, snippetBytes)
}
