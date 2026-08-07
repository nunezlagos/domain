//go:build integration

package compliance_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/compliance"
	"nunezlagos/domain/internal/store/txctx"
)

const razonValida = "el cifrado en reposo entra en el sprint siguiente, ya hay ticket"

// REQ-3 — el waiver con razón escrita destraba y queda auditado.
func TestWaiver_ConRazon_SeOtorgaYQuedaAuditado(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ahora := time.Now()

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(ctx, tx)

	id, err := f.svc().OtorgarWaiver(scoped, f.proyectoA,
		"cifrado-en-reposo", "gdpr", razonValida, &f.userID, nil, nil)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)

	vivos, err := f.svc().WaiversVigentes(scoped, f.proyectoA, ahora)
	require.NoError(t, err)
	require.Len(t, vivos, 1)
	assert.Equal(t, razonValida, vivos[0].Razon, "la razón se conserva: es lo auditable")
	require.NotNil(t, vivos[0].OtorgadoPor, "sin actor el waiver no es auditable")
	assert.Equal(t, f.userID, *vivos[0].OtorgadoPor)
	assert.False(t, vivos[0].OtorgadoAt.IsZero())
}

// REQ-3 — sin razón utilizable se rechaza. Un waiver sin razón es un bypass con otro nombre.
func TestWaiver_SinRazonUtilizable_SeRechaza(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(ctx, tx)

	for _, razon := range []string{"", "   ", "ok", "no aplica"} {
		_, err := f.svc().OtorgarWaiver(scoped, f.proyectoA,
			"cifrado-en-reposo", "gdpr", razon, &f.userID, nil, nil)
		require.ErrorIs(t, err, compliance.ErrRazonRequerida,
			"la razón %q no debería alcanzar para otorgar un waiver", razon)
	}
}

// Un waiver vencido NO destraba: la excepción caduca, la obligación no. Sin esto una excepción
// temporal se vuelve permanente y nadie lo nota.
func TestWaiver_Vencido_NoApareceComoVigente(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ahora := time.Now()
	ayer := ahora.Add(-24 * time.Hour)

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(ctx, tx)

	_, err := f.svc().OtorgarWaiver(scoped, f.proyectoA,
		"cifrado-en-reposo", "gdpr", razonValida, &f.userID, &ayer, nil)
	require.NoError(t, err)

	vivos, err := f.svc().WaiversVigentes(scoped, f.proyectoA, ahora)
	require.NoError(t, err)
	assert.Empty(t, vivos, "un waiver vencido no puede seguir destrabando")
}

// Re-otorgar corrige la razón en vez de acumular filas.
func TestWaiver_ReOtorgar_ActualizaLaRazonYNoDuplica(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(ctx, tx)

	_, err := f.svc().OtorgarWaiver(scoped, f.proyectoA,
		"cifrado-en-reposo", "gdpr", razonValida, &f.userID, nil, nil)
	require.NoError(t, err)
	corregida := "corregido: el proveedor confirmó cifrado a nivel de volumen"
	_, err = f.svc().OtorgarWaiver(scoped, f.proyectoA,
		"cifrado-en-reposo", "gdpr", corregida, &f.userID, nil, nil)
	require.NoError(t, err)

	vivos, err := f.svc().WaiversVigentes(scoped, f.proyectoA, time.Now())
	require.NoError(t, err)
	require.Len(t, vivos, 1, "re-otorgar corrige, no acumula")
	assert.Equal(t, corregida, vivos[0].Razon)
}

// Revocar deja sin efecto pero CONSERVA la fila: el registro de que se otorgó y por qué es lo que
// hay que poder auditar después.
func TestWaiver_Revocar_DejaDeVigirPeroConservaElRegistro(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(ctx, tx)

	id, err := f.svc().OtorgarWaiver(scoped, f.proyectoA,
		"cifrado-en-reposo", "gdpr", razonValida, &f.userID, nil, nil)
	require.NoError(t, err)
	require.NoError(t, f.svc().RevocarWaiver(scoped, f.proyectoA, id))

	vivos, err := f.svc().WaiversVigentes(scoped, f.proyectoA, time.Now())
	require.NoError(t, err)
	assert.Empty(t, vivos)

	var n int
	require.NoError(t, tx.QueryRow(ctx,
		`SELECT count(*) FROM compliance_waivers WHERE id = $1 AND revocado_at IS NOT NULL`,
		id).Scan(&n))
	assert.Equal(t, 1, n, "la fila se conserva revocada: el registro es el punto")
}

// Tras revocar se puede otorgar uno nuevo: el índice único es parcial sobre los vivos.
func TestWaiver_TrasRevocar_SePuedeOtorgarOtro(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()
	scoped := txctx.WithTxContext(ctx, tx)

	id, err := f.svc().OtorgarWaiver(scoped, f.proyectoA, "cifrado-en-reposo", "gdpr",
		razonValida, &f.userID, nil, nil)
	require.NoError(t, err)
	require.NoError(t, f.svc().RevocarWaiver(scoped, f.proyectoA, id))

	_, err = f.svc().OtorgarWaiver(scoped, f.proyectoA, "cifrado-en-reposo", "gdpr",
		"segunda excepción con su propia justificación escrita", &f.userID, nil, nil)

	require.NoError(t, err, "un waiver revocado no puede bloquear que se otorgue otro")
}

// RLS — los waivers de otro proyecto no son visibles. Un waiver es una decisión sensible: revela
// qué obligación se decidió no cumplir.
func TestWaiver_RLS_NoSeVenLosDeOtroProyecto(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	ctxB, txB, cerrarB := f.enScope(t, f.proyectoB)
	scopedB := txctx.WithTxContext(ctxB, txB)
	_, err := f.svc().OtorgarWaiver(scopedB, f.proyectoB, "cifrado-en-reposo", "gdpr",
		razonValida, &f.userID, nil, nil)
	require.NoError(t, err)
	require.NoError(t, txB.Commit(ctxB))
	cerrarB()

	ctxA, txA, cerrarA := f.enScope(t, f.proyectoA)
	defer cerrarA()
	vivos, err := f.svc().WaiversVigentes(txctx.WithTxContext(ctxA, txA), f.proyectoA, time.Now())

	require.NoError(t, err)
	assert.Empty(t, vivos, "el proyecto A está viendo los waivers del proyecto B")
}

// El CHECK de la base es la segunda capa: aunque alguien saltee el service, una razón corta no
// entra. Defensa en profundidad sobre lo que hace auditable al waiver.
func TestWaiver_CheckDeLaBase_RechazaRazonCorta(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()

	ctx, tx, cerrar := f.enScope(t, f.proyectoA)
	defer cerrar()

	_, err := tx.Exec(ctx,
		`INSERT INTO compliance_waivers (project_id, control_slug, framework_slug, razon)
		 VALUES ($1,'x','y','corta')`, f.proyectoA)

	require.Error(t, err, "el CHECK de la base tiene que rechazar una razón de 5 caracteres")
	assert.True(t, strings.Contains(err.Error(), "constraint") ||
		strings.Contains(err.Error(), "check"), "el error viene del CHECK: %v", err)
}
