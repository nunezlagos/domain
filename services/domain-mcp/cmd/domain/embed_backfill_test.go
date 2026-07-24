package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBackfillArgs_SinArgumentos_UsaDefaults(t *testing.T) {
	o := parseBackfillArgs(nil)
	require.Equal(t, 200, o.limit)
	require.False(t, o.dryRun)
	require.False(t, o.all)
	require.Equal(t, 100, o.pauseMS)
}

// DOMAINSERV-80 H2: el org-uuid era OBLIGATORIO pero el SQL nunca lo usaba —
// no hay columna organization_id en knowledge_observations (tiene project_id) ni
// en knowledge_chunks. Pasa a ser opcional y el backfill declara ser global.
func TestParseBackfillArgs_SinOrg_NoEsError(t *testing.T) {
	o := parseBackfillArgs([]string{"--limit=50"})
	require.Equal(t, 50, o.limit)
	require.Empty(t, o.orgArg)
}

func TestParseBackfillArgs_OrgPosicional_SeAceptaPorCompatibilidad(t *testing.T) {
	o := parseBackfillArgs([]string{"11111111-2222-3333-4444-555555555555", "--all"})
	require.Equal(t, "11111111-2222-3333-4444-555555555555", o.orgArg)
	require.True(t, o.all)
}

func TestParseBackfillArgs_PauseMS_SeParsea(t *testing.T) {
	require.Equal(t, 0, parseBackfillArgs([]string{"--pause-ms=0"}).pauseMS)
	require.Equal(t, 250, parseBackfillArgs([]string{"--pause-ms=250"}).pauseMS)
}

// La query se arma distinto según la tabla: knowledge_chunks NO tiene deleted_at,
// así que incluir el filtro la rompería con "column does not exist".
func TestBuildBackfillQuery_TablaSinDeletedAt_OmiteElFiltro(t *testing.T) {
	q := buildBackfillQuery("knowledge_chunks", "content", "embedding", false)
	require.NotContains(t, q, "deleted_at")
	require.Contains(t, q, "embedding IS NULL")
	require.Contains(t, q, "knowledge_chunks")
}

func TestBuildBackfillQuery_TablaConDeletedAt_IncluyeElFiltro(t *testing.T) {
	q := buildBackfillQuery("knowledge_observations", "content", "embedding", true)
	require.Contains(t, q, "deleted_at IS NULL")
}

func TestBuildBackfillQuery_SoloTomaFilasSinEmbedding(t *testing.T) {
	q := buildBackfillQuery("knowledge_observations", "content", "embedding", true)
	require.Contains(t, strings.ReplaceAll(q, "\n", " "), "embedding IS NULL",
		"el filtro IS NULL es lo que hace idempotente al backfill: sin él, el cron diario re-embeddearía todo cada noche")
}

// Regresión: la 2a tabla apuntaba a knowledge_docs con un embCol dummy
// ("(SELECT id FROM knowledge_docs WHERE 1=0)") y la guarda de SELECT hacía que
// retornara 0 siempre. Los 114 chunks nunca se backfilleaban.
func TestBackfillTargets_IncluyeKnowledgeChunksReal(t *testing.T) {
	targets := backfillTargets()
	require.Len(t, targets, 2)

	byTable := map[string]backfillTarget{}
	for _, tg := range targets {
		byTable[tg.table] = tg
	}

	obs, ok := byTable["knowledge_observations"]
	require.True(t, ok)
	require.Equal(t, "content", obs.textCol)
	require.Equal(t, "embedding", obs.embCol)
	require.True(t, obs.hasDeletedAt)

	ch, ok := byTable["knowledge_chunks"]
	require.True(t, ok, "knowledge_chunks debe ser un target real, no un placeholder")
	require.Equal(t, "content", ch.textCol)
	require.Equal(t, "embedding", ch.embCol)
	require.False(t, ch.hasDeletedAt, "knowledge_chunks no tiene columna deleted_at")

	for _, tg := range targets {
		require.NotContains(t, tg.embCol, "SELECT",
			"ningún target puede llevar un embCol dummy con SELECT")
	}
}
