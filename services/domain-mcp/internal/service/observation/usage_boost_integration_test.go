//go:build integration

package observation_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	obssvc "nunezlagos/domain/internal/service/observation"
)

// DOMAINSERV-145: la señal de consumo se escribía en knowledge_observation_usage_log y
// nadie la leía — la única aparición de SELECT sobre esa tabla en todo el repo era el
// GRANT de su propia migración. El boost es el consumidor: una observación reportada
// como útil sube en el ranking. Sin esto la tabla es un log que nadie lee y el ticket
// no pasa de relevancia a utilidad, que es su objetivo declarado.
//
// El boost entra como TERCERA MODALIDAD del RRF, no como un peso arbitrario: mismo
// 1/(k+rank) que bm25 y vector, así que no hay constante que tunear.
func TestObservation_SearchHybrid_ObservacionUsada_RankeaMasAlto(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	// las dos matchean la query por igual: lo único que las separa es la señal de uso
	ignorada := guardarObservacion(t, f, "el deploy del vps se hace con el instalador")
	util := guardarObservacion(t, f, "el deploy del vps se hace con el instalador canonico")

	sinBoost, err := f.svc.SearchHybrid(ctx, f.orgID, "deploy vps instalador", 10)
	require.NoError(t, err)
	require.Len(t, sinBoost, 2, "las dos observaciones tienen que matchear la query")

	promptID := insertarPromptCapturado(t, f)
	_, _, err = f.svc.RecordUsage(ctx, promptID,
		[]uuid.UUID{ignorada, util}, []uuid.UUID{util})
	require.NoError(t, err)

	conBoost, err := f.svc.SearchHybrid(ctx, f.orgID, "deploy vps instalador", 10)
	require.NoError(t, err)
	require.Len(t, conBoost, 2)

	require.Equal(t, util, conBoost[0].ID,
		"la observación reportada como útil tiene que quedar primera")
	require.Greater(t, scoreDe(t, conBoost, util), scoreDe(t, sinBoost, util),
		"el score de la usada tiene que subir respecto de la corrida sin señal")
	require.Equal(t, scoreDe(t, sinBoost, ignorada), scoreDe(t, conBoost, ignorada),
		"la no reportada no puede cambiar de score: el boost suma, no penaliza")
}

// La ausencia de reporte significa SIN DATO, nunca "no sirvió" — invariante que fija la
// propia migración 000277. Con la tabla vacía el ranking tiene que ser el de siempre.
func TestObservation_SearchHybrid_SinSenalDeUso_RankingIntacto(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	guardarObservacion(t, f, "el deploy del vps se hace con el instalador")
	guardarObservacion(t, f, "el backfill de embeddings corre en el cron")

	res, err := f.svc.SearchHybrid(ctx, f.orgID, "deploy vps instalador", 10)
	require.NoError(t, err)
	require.NotEmpty(t, res, "sin señal de uso la búsqueda tiene que seguir devolviendo resultados")
	for _, r := range res {
		require.Positive(t, r.Score, "un score en cero delataría que el boost rompió el RRF")
	}
}

// Reportar used=false es una señal válida y distinta de no reportar: no debe subir nada.
func TestObservation_SearchHybrid_ReportadaComoNoUsada_NoRecibeBoost(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	candidata := guardarObservacion(t, f, "el deploy del vps se hace con el instalador")

	antes, err := f.svc.SearchHybrid(ctx, f.orgID, "deploy vps instalador", 10)
	require.NoError(t, err)

	promptID := insertarPromptCapturado(t, f)
	_, _, err = f.svc.RecordUsage(ctx, promptID, []uuid.UUID{candidata}, nil)
	require.NoError(t, err)

	despues, err := f.svc.SearchHybrid(ctx, f.orgID, "deploy vps instalador", 10)
	require.NoError(t, err)

	require.Equal(t, scoreDe(t, antes, candidata), scoreDe(t, despues, candidata),
		"una candidata que no se usó no puede recibir boost")
}

// El boost REORDENA lo que ya matcheó, no incorpora candidatos nuevos. Con un FULL
// OUTER JOIN sobre la tabla de usage, una observación usada entraba al resultado por el
// solo hecho de estar usada, sin pasar por bm25 ni por vector.
//
// La aserción compara el CONJUNTO de ids antes y después de registrar el uso, en vez de
// buscar una observación puntual: bajo FakeEmbedder el CTE vec devuelve los N vecinos
// más cercanos sin umbral, así que cualquier observación aparece igual por la vía
// vectorial y una aserción sobre un id concreto no distingue quién la trajo.
func TestObservation_SearchHybrid_ElBoost_NoIncorporaCandidatosNuevos(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	// sin embedding y sin match textual: no puede entrar por vec (filtra embedding IS
	// NOT NULL) ni por bm25, así que si aparece fue el boost quien la trajo
	ajena := insertarObservacionSinEmbedding(t, f, "la migracion 143 elimino el helper current_org_id")
	guardarObservacion(t, f, "el deploy del vps se hace con el instalador")

	antes, err := f.svc.SearchHybrid(ctx, f.orgID, "deploy vps instalador", 10)
	require.NoError(t, err)
	require.NotContains(t, idsDe(antes), ajena, "sin señal de uso la ajena no matchea nada")

	promptID := insertarPromptCapturado(t, f)
	_, _, err = f.svc.RecordUsage(ctx, promptID, []uuid.UUID{ajena}, []uuid.UUID{ajena})
	require.NoError(t, err)

	despues, err := f.svc.SearchHybrid(ctx, f.orgID, "deploy vps instalador", 10)
	require.NoError(t, err)

	require.ElementsMatch(t, idsDe(antes), idsDe(despues),
		"registrar uso no puede cambiar QUÉ se devuelve, solo en qué orden")
}

func guardarObservacion(t *testing.T, f *fixture, contenido string) uuid.UUID {
	t.Helper()

	o, err := f.svc.Save(context.Background(), obssvc.SaveInput{
		OrganizationID: f.orgID,
		ProjectID:      f.projectID,
		CreatedBy:      &f.owner,
		Content:        contenido,
	})
	require.NoError(t, err)
	return o.ID
}

// el FK de knowledge_observation_usage_log apunta a prompt_captured, así que la fila
// tiene que existir antes de reportar uso
func insertarPromptCapturado(t *testing.T, f *fixture) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := f.pool.QueryRow(context.Background(), `
INSERT INTO prompt_captured (user_id, project_id, content, char_count, estimated_tokens_in)
VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		f.owner, f.projectID, "cómo se hace el deploy del vps", 30, 8).Scan(&id)
	require.NoError(t, err)
	return id
}

// va por SQL y no por Save porque el service siempre calcula embedding: con el
// FakeEmbedder toda observación entra por el CTE vec y el test dejaría de discriminar
func insertarObservacionSinEmbedding(t *testing.T, f *fixture, contenido string) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := f.pool.QueryRow(context.Background(), `
INSERT INTO knowledge_observations (project_id, created_by, content, observation_type, content_hash)
VALUES ($1, $2, $3, 'note', digest($3, 'sha256')) RETURNING id`,
		f.projectID, f.owner, contenido).Scan(&id)
	require.NoError(t, err)
	return id
}

func idsDe(res []obssvc.SearchResult) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(res))
	for _, r := range res {
		out = append(out, r.ID)
	}
	return out
}

func scoreDe(t *testing.T, res []obssvc.SearchResult, id uuid.UUID) float64 {
	t.Helper()

	for _, r := range res {
		if r.ID == id {
			return r.Score
		}
	}
	t.Fatalf("la observación %s no aparece en los resultados", id)
	return 0
}
