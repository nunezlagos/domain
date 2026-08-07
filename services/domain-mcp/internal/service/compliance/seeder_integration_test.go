//go:build integration

package compliance_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/compliance"
	"nunezlagos/domain/internal/store/txctx"
)

// correrSeeder ejecuta el seeder del catálogo dentro de una tx propia.
func (f *fixture) correrSeeder(t *testing.T) seeds.Report {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pools.App.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	rep, err := (&seeds.ComplianceCatalogSeeder{}).Run(ctx, tx, seeds.EnvProd)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	return rep
}

// El catálogo sembrado tiene que quedar consultable por el service, que es como lo va a leer la
// fase sdd-compliance.
func TestSeeder_CargaInicial_QuedaConsultable(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	f.correrSeeder(t)

	marcos, err := f.svc().ListarCatalogo(context.Background())
	require.NoError(t, err)

	porSlug := map[string]compliance.Framework{}
	for _, m := range marcos {
		porSlug[m.Slug] = m
	}
	require.Contains(t, porSlug, "ley-21719")
	require.Contains(t, porSlug, "ley-21595")
	require.Contains(t, porSlug, "gdpr")

	assert.True(t, porSlug["gdpr"].Obligatorio, "el GDPR es obligatorio, no una recomendación")
	assert.Equal(t, "EU", porSlug["gdpr"].Jurisdiccion)
	assert.True(t, porSlug["ley-21719"].AdmiteTextoCompleto(),
		"las leyes chilenas son texto público: su articulado se puede ingestar")
}

// La 21.719 rige recién en dic-2026: sembrarla sin esa fecha la haría aparecer como incumplida hoy.
func TestSeeder_Ley21719_NoVigenteAntesDeDiciembre2026(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	f.correrSeeder(t)

	marco, err := f.svc().BuscarPorSlug(context.Background(), "ley-21719", "")
	require.NoError(t, err)

	require.NotNil(t, marco.VigenteDesde, "sin vigente_desde la ley se reportaría vigente desde siempre")
	assert.False(t, marco.Vigente(time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)))
	assert.True(t, marco.Vigente(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)))
}

// EL VALOR DEL CROSSWALK, medido: un proyecto afecto a los tres marcos recibe cifrado-en-reposo
// una vez por cada marco que lo exige, y lo implementa UNA sola vez.
func TestSeeder_Crosswalk_CifradoEnReposo_LoExigenTresMarcos(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	f.correrSeeder(t)
	svc := f.svc()
	ctx := context.Background()

	sctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(sctx, tx)
	for _, slug := range []string{"ley-21719", "ley-21595", "gdpr"} {
		marco, err := svc.BuscarPorSlug(ctx, slug, "")
		require.NoError(t, err)
		require.NoError(t, svc.DeclararMarco(scoped, f.proyectoA, marco.ID, &f.userID, true))
	}

	controles, err := svc.ControlesExigidos(scoped, f.proyectoA, time.Now())
	require.NoError(t, err)

	exigenCifrado := map[string]string{}
	for _, c := range controles {
		if c.ControlSlug == "cifrado-en-reposo" {
			exigenCifrado[c.FrameworkSlug] = c.Referencia
		}
	}
	require.Len(t, exigenCifrado, 2,
		"cifrado-en-reposo lo exigen la 21.719 y el GDPR; la 21.595 no es de datos personales")
	assert.Equal(t, "Art. 32", exigenCifrado["gdpr"], "el GDPR se cita por artículo")
	assert.Contains(t, exigenCifrado, "ley-21719")
}

// Re-seedear no duplica ni rompe: el catálogo se cambia con seeder + bump, y un re-seed es normal
// en cada deploy.
func TestSeeder_ReSeed_EsIdempotente(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	f.correrSeeder(t)
	primera, err := f.svc().ListarCatalogo(context.Background())
	require.NoError(t, err)

	f.correrSeeder(t)
	segunda, err := f.svc().ListarCatalogo(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(primera), len(segunda), "el re-seed no puede duplicar marcos")
}

// El seeder NO declara marcos por proyecto: el opt-in es explícito, y sembrar una declaración
// sería exactamente el opt-out que este diseño evita.
func TestSeeder_NoDeclaraMarcosPorProyecto(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	f.correrSeeder(t)

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()

	var n int
	require.NoError(t, tx.QueryRow(ctx,
		`SELECT count(*) FROM project_compliance_frameworks`).Scan(&n))

	assert.Zero(t, n,
		"sembrar el catálogo no puede dejar a un proyecto afecto a nada sin que alguien lo declare")
}
