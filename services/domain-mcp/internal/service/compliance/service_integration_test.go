//go:build integration

package compliance_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/compliance"
	"nunezlagos/domain/internal/store/txctx"
)

func (f *fixture) svc() *compliance.Service {
	return &compliance.Service{Pool: f.pools.App}
}

// crearControl inserta un control del catálogo y lo vincula a un marco con su referencia.
func (f *fixture) vincularControl(t *testing.T, marco uuid.UUID, slugControl, referencia string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var control uuid.UUID
	err := f.pools.App.QueryRow(ctx,
		`INSERT INTO compliance_controls (slug, nombre) VALUES ($1, $1)
		 ON CONFLICT (slug) WHERE deleted_at IS NULL DO UPDATE SET nombre = EXCLUDED.nombre
		 RETURNING id`, slugControl).Scan(&control)
	require.NoError(t, err)
	_, err = f.pools.App.Exec(ctx,
		`INSERT INTO compliance_framework_controls (framework_id, control_id, referencia) VALUES ($1,$2,$3)
		 ON CONFLICT (framework_id, control_id) DO UPDATE SET referencia = EXCLUDED.referencia`,
		marco, control, referencia)
	require.NoError(t, err)
	return control
}

// REQ-1 — el opt-in visto desde el service: sin declarar, no hay marcos ni controles exigidos.
func TestService_ProyectoSinDeclarar_NoTieneMarcosNiControles(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	svc := f.svc()

	marco := f.crearMarco(t, "ley-21719", "ley", true)
	f.vincularControl(t, marco, "cifrado-en-reposo", "Art. 14")

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(ctx, tx)

	marcos, err := svc.MarcosDelProyecto(scoped, f.proyectoA)
	require.NoError(t, err)
	assert.Empty(t, marcos, "sin declaración no hay marcos, aunque el catálogo los tenga")

	controles, err := svc.ControlesExigidos(scoped, f.proyectoA, time.Now())
	require.NoError(t, err)
	assert.Empty(t, controles, "sin marcos declarados no se exige ningún control")
}

// REQ-2 — el crosswalk desde el service: un control exigido por dos marcos sale dos veces, cada
// una con SU referencia. Eso es lo que permite evaluarlo una vez y reportarlo en ambos.
func TestService_ControlesExigidos_UnControlDosMarcos_CadaUnoConSuReferencia(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	svc := f.svc()

	gdpr := f.crearMarco(t, "gdpr", "reglamento", true)
	ley := f.crearMarco(t, "ley-21719", "ley", true)
	f.vincularControl(t, gdpr, "cifrado-en-reposo", "Art. 32")
	f.vincularControl(t, ley, "cifrado-en-reposo", "Art. 14")

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(ctx, tx)
	require.NoError(t, svc.DeclararMarco(scoped, f.proyectoA, gdpr, &f.userID, true))
	require.NoError(t, svc.DeclararMarco(scoped, f.proyectoA, ley, &f.userID, true))

	controles, err := svc.ControlesExigidos(scoped, f.proyectoA, time.Now())
	require.NoError(t, err)

	require.Len(t, controles, 2, "el mismo control sale una vez por cada marco que lo exige")
	refs := map[string]string{}
	for _, c := range controles {
		assert.Equal(t, "cifrado-en-reposo", c.ControlSlug)
		refs[c.FrameworkSlug] = c.Referencia
	}
	assert.Equal(t, "Art. 32", refs["gdpr"])
	assert.Equal(t, "Art. 14", refs["ley-21719"])
}

// REQ-5 — vigencia: la 21.719 rige recién en dic-2026. "Te aplica" y "te va a aplicar" no son lo
// mismo, y de esa diferencia sale la severidad del finding.
func TestService_MarcoNoVigenteAun_SeReportaComoNoVigente(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	var id uuid.UUID
	require.NoError(t, f.pools.App.QueryRow(ctx,
		`INSERT INTO compliance_frameworks (slug, nombre, tipo, obligatorio, vigente_desde)
		 VALUES ('ley-21719','Ley 21.719','ley',true,'2026-12-01') RETURNING id`).Scan(&id))
	f.vincularControl(t, id, "rat", "Art. 15")

	sctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(sctx, tx)
	require.NoError(t, f.svc().DeclararMarco(scoped, f.proyectoA, id, &f.userID, true))

	antes := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
	despues := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	cAntes, err := f.svc().ControlesExigidos(scoped, f.proyectoA, antes)
	require.NoError(t, err)
	require.Len(t, cAntes, 1)
	assert.False(t, cAntes[0].Vigente,
		"antes del 2026-12-01 la obligación existe pero todavía no rige: no puede ser un BLOCKER")

	cDespues, err := f.svc().ControlesExigidos(scoped, f.proyectoA, despues)
	require.NoError(t, err)
	require.Len(t, cDespues, 1)
	assert.True(t, cDespues[0].Vigente)
}

// Declarar es idempotente: el caso real es corregir una desactivación, no fallar por duplicado.
func TestService_DeclararMarco_DosVeces_ActualizaYNoDuplica(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	svc := f.svc()
	marco := f.crearMarco(t, "gdpr", "reglamento", true)

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(ctx, tx)

	require.NoError(t, svc.DeclararMarco(scoped, f.proyectoA, marco, &f.userID, true))
	require.NoError(t, svc.DeclararMarco(scoped, f.proyectoA, marco, &f.userID, false))

	marcos, err := svc.MarcosDelProyecto(scoped, f.proyectoA)
	require.NoError(t, err)
	assert.Empty(t, marcos, "declarado con activo=false deja de aplicar")

	require.NoError(t, svc.DeclararMarco(scoped, f.proyectoA, marco, &f.userID, true))
	marcos, err = svc.MarcosDelProyecto(scoped, f.proyectoA)
	require.NoError(t, err)
	assert.Len(t, marcos, 1, "reactivar tiene que funcionar, no fallar por duplicado")
}

// REQ-3 — el guard de copyright, en los dos sentidos.
func TestService_GuardDeFuente_LeyIngestable_NormaNo(t *testing.T) {
	svc := &compliance.Service{}

	ley := compliance.Framework{Slug: "ley-21719", FuenteTipo: "texto_libre"}
	require.NoError(t, svc.GuardDeFuente(ley),
		"las leyes chilenas son texto público: su articulado se puede ingestar completo")

	iso := compliance.Framework{Slug: "iso-27001", FuenteTipo: "solo_referencia"}
	err := svc.GuardDeFuente(iso)
	require.ErrorIs(t, err, compliance.ErrFuenteNoRedistribuible,
		"el texto de ISO es de pago y no se puede redistribuir ni en un repo privado")
	assert.Contains(t, err.Error(), "iso-27001", "el error nombra el marco para poder diagnosticar")
}

// El estado inválido se rechaza en el service y no llega al CHECK de la base: el error del CHECK
// es incomprensible para quien llama por MCP.
func TestService_RegistrarEstado_EstadoInvalido_SeRechazaAntesDeLaBase(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(ctx, tx)

	err := f.svc().RegistrarEstado(scoped, f.proyectoA, uuid.New(), "casi", "", nil)

	require.ErrorIs(t, err, compliance.ErrEstadoInvalido)
}

// Registrar es idempotente por (project_id, control_id): una re-evaluación actualiza en vez de
// acumular filas históricas que después habría que desduplicar al reportar.
func TestService_RegistrarEstado_ReEvaluacion_ActualizaLaMismaFila(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	marco := f.crearMarco(t, "gdpr", "reglamento", true)
	control := f.vincularControl(t, marco, "cifrado-en-reposo", "Art. 32")

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(ctx, tx)

	require.NoError(t, f.svc().RegistrarEstado(scoped, f.proyectoA, control, "falta", "sin cifrar", &f.userID))
	require.NoError(t, f.svc().RegistrarEstado(scoped, f.proyectoA, control, "ok", "AES-256 at rest", &f.userID))

	var n int
	var estado, evidencia string
	require.NoError(t, tx.QueryRow(ctx,
		`SELECT count(*) FROM project_control_status WHERE project_id = $1`, f.proyectoA).Scan(&n))
	require.NoError(t, tx.QueryRow(ctx,
		`SELECT estado, evidencia FROM project_control_status
		 WHERE project_id = $1 AND control_id = $2`, f.proyectoA, control).Scan(&estado, &evidencia))

	assert.Equal(t, 1, n, "la re-evaluación actualiza, no acumula")
	assert.Equal(t, "ok", estado)
	assert.Equal(t, "AES-256 at rest", evidencia)
}

// El catálogo se lista sin scope de proyecto: es global por diseño.
func TestService_ListarCatalogo_SinScope_DevuelveTodo(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	f.crearMarco(t, "ley-21719", "ley", true)
	f.crearMarco(t, "iso-27001", "norma_tecnica", false)

	marcos, err := f.svc().ListarCatalogo(context.Background())

	require.NoError(t, err)
	assert.Len(t, marcos, 2)
}
