package embedding

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// El predicado de pendiente tiene que tomar las DOS formas: NULL y norma cero. Si solo
// mirara NULL, las filas que el NopEmbedder escribió como vectores de ceros quedarían
// fuera para siempre — es el bug de DOMAINSERV-157.
func TestPendingRowsQuery_TomaNullYNormaCero(t *testing.T) {
	sql := PendingRowsQuery(KnowledgeChunks())

	assert.Contains(t, sql, "embedding IS NULL")
	assert.Contains(t, sql, "(embedding <#> embedding) = 0",
		"la norma cero se detecta por producto interno consigo mismo")
}

// knowledge_chunks NO tiene deleted_at: agregarle el filtro rompe la query con
// "column does not exist", y el worker fallaría en cada tick.
func TestPendingRowsQuery_KnowledgeChunks_SinFiltroDeDeletedAt(t *testing.T) {
	sql := PendingRowsQuery(KnowledgeChunks())

	assert.NotContains(t, sql, "deleted_at",
		"knowledge_chunks no tiene deleted_at")
}

func TestPendingRowsQuery_TablaConDeletedAt_LoFiltra(t *testing.T) {
	var observaciones Target
	for _, t := range Targets() {
		if t.Table == "knowledge_observations" {
			observaciones = t
		}
	}
	require.NotEmpty(t, observaciones.Table)

	assert.Contains(t, PendingRowsQuery(observaciones), "deleted_at IS NULL")
}

// El backfill de cmd/domain y el Completer del server barren las mismas filas. Esta es
// la razón de que Targets() sea una sola función: con dos copias, la que quedara atrás
// dejaría una tabla sin repoblar y nada fallaría.
func TestTargets_CubreLasTresTablasConEmbedding(t *testing.T) {
	var tablas []string
	for _, tg := range Targets() {
		tablas = append(tablas, tg.Table)
	}

	assert.ElementsMatch(t,
		[]string{"knowledge_observations", "knowledge_chunks", "skills"}, tablas)
}

// KnowledgeChunks() paniquea si sale de Targets() en vez de devolver un Target vacío:
// un Target sin tabla generaría `SELECT id, FROM  WHERE ...`, que falla en la query y
// se lee como un problema de SQL en lugar de una lista mal editada.
func TestKnowledgeChunks_EstaEnTargets(t *testing.T) {
	assert.Equal(t, "knowledge_chunks", KnowledgeChunks().Table)
	assert.False(t, KnowledgeChunks().HasDeletedAt)
}

// El backoff es lo que impide golpear al provider para siempre por una fila que
// rechaza siempre (texto que excede su contexto). Sin él, un lote lleno que no escribe
// nada se vuelve a pedir en cada tick indefinidamente.
func TestCompleter_Backoff_CreceHastaElTechoYNoMas(t *testing.T) {
	c := &Completer{}

	c.crecerBackoff()
	assert.Equal(t, 1, c.backoff, "el primer fallo salta un tick")

	for i := 0; i < 10; i++ {
		c.crecerBackoff()
	}
	assert.Equal(t, backoffMaximo, c.backoff,
		"el backoff no crece sin techo: con uno ilimitado el worker quedaría dormido para siempre")
}

// Sin pool ni embedder el worker no arranca en vez de paniquear en el primer tick: el
// server tiene que levantar igual si el embedder quedó en noop.
func TestCompleter_SinDependencias_NoArranca(t *testing.T) {
	c := &Completer{}

	// Start retorna sin bloquear porque corta antes del ticker
	c.Start(nil)

	assert.Equal(t, tickPorDefecto, c.Tick, "aun cortando, deja los defaults visibles")
	assert.Equal(t, batchPorDefecto, c.Batch)
}

// El LIMIT va parametrizado ($1) y no interpolado: el batch entra por config y una
// interpolación lo volvería una vía de inyección desde el entorno.
func TestPendingRowsQuery_LimitEsParametro(t *testing.T) {
	sql := PendingRowsQuery(KnowledgeChunks())

	assert.True(t, strings.Contains(sql, "LIMIT $1"),
		"el límite tiene que ser un parámetro, no texto interpolado")
}
