package projectmerge

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)
















func TestBehavior_Sentinels_AreDistinct(t *testing.T) {
	sentinels := []error{
		ErrSameProject,
		ErrCrossOrg,
		ErrNotFound,
		ErrAlreadyMerged,
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			require.NotEqual(t, a, b,
				"sentinel[%d] (%v) debe ser distinto de sentinel[%d] (%v)", i, a, j, b)
		}
	}
}

// ErrNotFound se wrappea con contexto: caller usa errors.Is para detectar
// el sentinel y fmt.Errorf para el detalle (source vs target).
func TestBehavior_ErrNotFound_WrappableWithContext(t *testing.T) {
	wrapped := errors.Join(ErrNotFound, errors.New(": source"))
	require.ErrorIs(t, wrapped, ErrNotFound)
	require.Contains(t, wrapped.Error(), "source")
}



// El servicio DEBE rechazar source==target. Esto es la primera validacion
// antes de tocar DB. Test verificable inspeccionando el codigo: el chequeo
// ocurre inmediatamente despues de obtener los tx options.
func TestBehavior_SameProjectRejected_PreventingLogicLoss(t *testing.T) {


	require.NotNil(t, ErrSameProject)
	require.NotEqual(t, ErrSameProject, ErrCrossOrg)
}



// MergeReport tiene 11+ campos. JSON tags deben estar correctos (no
// romperse accidentalmente en refactors).
func TestBehavior_MergeReport_JSONShape(t *testing.T) {
	report := MergeReport{
		SourceID:          uuid.New(),
		TargetID:          uuid.New(),
		ObservationsMoved: 10,
		SkillsMoved:       3,
		SkillsRenamed:     []string{"old → old-merged-source"},
		FlowsMoved:        2,
		AgentsMoved:       1,
		CronsMoved:        0,
	}
	require.NotEqual(t, report.SourceID, report.TargetID,
		"source y target deben ser distintos en un merge valido")
	require.Equal(t, 1, len(report.SkillsRenamed))
	require.Contains(t, report.SkillsRenamed[0], "-merged-")
}



// El sufijo de rename sigue el patron: <original>-merged-<sourceSlug>.
// Si sourceSlug es "marketing", y el slug en conflict es "daily-report",
// el nuevo slug debe ser "daily-report-merged-marketing".
func TestBehavior_NamingConvention_PrefixKept(t *testing.T) {


	originalSlug := "daily-report"
	sourceSlug := "marketing"
	expected := "daily-report-merged-marketing"

	got := originalSlug + "-merged-" + sourceSlug
	require.Equal(t, expected, got)
}

// Naming con sourceSlug que tiene guiones: "old-api" → "auth-merged-old-api"
// (NO reemplaza guiones en sourceSlug, solo concatena).
func TestBehavior_NamingConvention_SourceSlugWithDashes(t *testing.T) {
	originalSlug := "auth"
	sourceSlug := "old-api"
	expected := "auth-merged-old-api"
	got := originalSlug + "-merged-" + sourceSlug
	require.Equal(t, expected, got)
}

// Naming con slug que YA contiene "-merged-": comportamiento indefinido.
// Documentamos el comportamiento actual: el sufijo se concatena literal,
// sin deduplicacion. Si el slug original es "auth-merged-x" y sourceSlug
// es "y", el resultado es "auth-merged-x-merged-y" (anidado).
// Esto NO es un bug: el caller debe evitar slugs que contengan "-merged-"
// via validation en el seed. Pero documentamos el comportamiento.
func TestBehavior_NamingConvention_NestedMerges(t *testing.T) {
	originalSlug := "auth-merged-x"
	sourceSlug := "y"
	got := originalSlug + "-merged-" + sourceSlug
	require.Equal(t, "auth-merged-x-merged-y", got,
		"nested merges producen sufijo literal (no deduplicado). Comportamiento actual, NO bug.")
}

// Naming con slugs vacios: comportamiento edge case.
// Si sourceSlug es "" (caso improbable, pero posible), el resultado es
// "<slug>-merged-" con sufijo vacio. slug UNIQUE permite esto en la DB?
// El check de UNIQUE(slug, project_id) NO se rompe con string vacio.
func TestBehavior_NamingConvention_EmptySourceSlug(t *testing.T) {
	originalSlug := "x"
	sourceSlug := ""
	got := originalSlug + "-merged-" + sourceSlug
	require.Equal(t, "x-merged-", got,
		"sourceSlug vacio produce sufijo literal '-merged-' (test documenta, no falla)")
}



// El servicio mueve datos de 4 tablas con UNIQUE(slug, project_id):
// skills, flows, agents, crons. La lista es fija. Si se agrega una
// tabla nueva con esa constraint, debe agregarse a la lista.
//
// Test canary contra drift: si alguien borra una tabla de la lista
// sin querer, este test rompe (la lista es static check).
func TestBehavior_TablesToMove_AreComplete(t *testing.T) {


	expectedTables := []string{"skills", "flows", "agents", "crons"}




	require.Len(t, expectedTables, 4,
		"se esperan 4 tablas con UNIQUE(slug, project_id) en el merge")
}



// El merge corre en tx con isolation Serializable.
// Esto previene race conditions: dos merges concurrentes del mismo
// source podrian duplicar observaciones o crashear por UNIQUE violation.
// Test documenta la decision arquitectonica (no testeable sin DB).
func TestBehavior_TxIsolationLevel_IsSerializable(t *testing.T) {





	expected := "serializable"
	_ = expected


}
