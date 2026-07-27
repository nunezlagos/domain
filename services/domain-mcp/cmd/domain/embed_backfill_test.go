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

// Un vector de ceros NO es NULL, así que el filtro IS NULL lo dejaba fuera para
// siempre: 48 observaciones que el embedder degradado a noop (DOMAINSERV-157)
// llenó de ceros eran invisibles al backfill, y competían en el ranking con la
// misma distancia contra cualquier búsqueda. Se detectan por el producto interno
// consigo mismo, que es -||v||² y solo vale 0 en el vector nulo.
func TestBuildBackfillQuery_TambienTomaFilasConVectorEnCero(t *testing.T) {
	q := strings.ReplaceAll(buildBackfillQuery("knowledge_observations", "content", "embedding", true), "\n", " ")

	require.Contains(t, q, "embedding <#> embedding",
		"sin esta condición los embeddings en cero no se regeneran nunca")
	require.Contains(t, q, "IS NULL OR")
}

// La idempotencia sigue siendo el invariante: una fila con un vector de norma real
// no se vuelve a tocar, así que re-correr el backfill no gasta llamadas al provider.
func TestBuildBackfillQuery_ConservaLaIdempotencia_NoTomaVectoresConNormaReal(t *testing.T) {
	q := strings.ReplaceAll(buildBackfillQuery("knowledge_chunks", "content", "embedding", false), "\n", " ")

	require.NotContains(t, q, "IS NOT NULL")
	require.Contains(t, q, "= 0", "el criterio es norma cero, no 'tiene embedding'")
}

// El corte del loop de --all miraba len(items) (candidatos), no filas escritas: un
// lote entero que falle de forma permanente devuelve n == limit y el loop vuelve a
// pedir el MISMO lote para siempre. Con el filtro ampliado a los vectores en cero
// dejó de ser hipotético: install.sh corre --all en cada deploy.
func TestDebeIterarOtroLote_LoteCompletoQueNoEscribeNada_Corta(t *testing.T) {
	require.False(t, debeIterarOtroLote(backfillOpts{all: true}, 0, 200, 200),
		"200 candidatos y 0 escrituras significa que ninguno se puede embeddear: insistir es un loop infinito")
}

func TestDebeIterarOtroLote_LoteLlenoConEscrituras_Sigue(t *testing.T) {
	require.True(t, debeIterarOtroLote(backfillOpts{all: true}, 200, 200, 200))
}

func TestDebeIterarOtroLote_LoteParcial_Corta(t *testing.T) {
	require.False(t, debeIterarOtroLote(backfillOpts{all: true}, 71, 71, 200),
		"un lote más chico que el límite ya agotó las filas pendientes")
}

func TestDebeIterarOtroLote_SinAll_CorreUnSoloLote(t *testing.T) {
	require.False(t, debeIterarOtroLote(backfillOpts{}, 200, 200, 200))
}

// En dry-run no se persiste nada, así que iterar daría el mismo lote para siempre.
func TestDebeIterarOtroLote_DryRun_Corta(t *testing.T) {
	require.False(t, debeIterarOtroLote(backfillOpts{all: true, dryRun: true}, 0, 200, 200))
}

// Regresión: la 2a tabla apuntaba a knowledge_docs con un embCol dummy
// ("(SELECT id FROM knowledge_docs WHERE 1=0)") y la guarda de SELECT hacía que
// retornara 0 siempre. Los 114 chunks nunca se backfilleaban.
func TestBackfillTargets_IncluyeKnowledgeChunksReal(t *testing.T) {
	targets := backfillTargets()
	// el conteo atrapa el borrado accidental de un target; subirlo es deliberado
	// (DOMAINSERV-157 agregó skills como tercero)
	require.Len(t, targets, 3)

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

// DOMAINSERV-157: skills entra al backfill.
//
// Era la única tabla con columna embedding que no estaba en backfillTargets, así
// que sus vectores en cero no se regeneraban ni con el binario arreglado: el
// backfill ni los miraba. Sin esto, cerrar el guard de escritura solo evita
// ceros NUEVOS y deja los viejos enterrados.
//
// El texto no sale de UNA columna: el service embebe `Name + " " + Description`
// (skill/service.go), así que el target lleva una expresión SQL. Es válido
// porque buildBackfillQuery interpola textCol tanto en el SELECT como en el
// LENGTH(TRIM(...)).
func TestBackfillTargets_IncluyeSkills_ConExpresionDeTextoYDeletedAt(t *testing.T) {
	var skills *backfillTarget
	for i, tg := range backfillTargets() {
		if tg.table == "skills" {
			skills = &backfillTargets()[i]
		}
	}
	require.NotNil(t, skills, "skills debe estar en backfillTargets: es la única tabla con embedding que quedaba afuera")
	require.True(t, skills.hasDeletedAt, "skills tiene deleted_at (migración 000010:30); omitirlo backfillea filas borradas")
	require.Contains(t, skills.textCol, "name", "el texto embebido combina name y description, no una sola columna")
	require.Contains(t, skills.textCol, "description")
}

// El shape de la expresión tiene que sobrevivir a la interpolación en las DOS
// posiciones donde buildBackfillQuery la usa.
func TestBuildBackfillQuery_TextColEsExpresion_SeInterpolaEnSelectYEnLength(t *testing.T) {
	q := buildBackfillQuery("skills", "name || ' ' || COALESCE(description, '')", "embedding", true)

	require.Contains(t, q, "SELECT id, name || ' ' || COALESCE(description, '') FROM skills")
	require.Contains(t, q, "LENGTH(TRIM(name || ' ' || COALESCE(description, ''))) > 0")
	require.Contains(t, q, "deleted_at IS NULL")
}
