package lifecycle

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)






// Comportamiento: default = 30 dias cuando no se setea.
func TestRetentionDays_DefaultIs30(t *testing.T) {
	s := &Service{}
	require.Equal(t, 30, s.retentionDays())
}

// Comportamiento: RetentionDays=0 cae al default (no se interpreta como "infinito").
// Esto es un canary: si alguien cambia el comportamiento para que 0 = "no retention",
// debe actualizar el test Y la HU-23.2 explicitamente.
func TestRetentionDays_ZeroFallsBackToDefault(t *testing.T) {
	s := &Service{RetentionDays: 0}
	require.Equal(t, 30, s.retentionDays(), "RetentionDays=0 NO debe interpretarse como 'sin retention'")
}

// Comportamiento: valor negativo cae al default. Validacion defensiva:
// el caller deberia validar >= 1 antes, pero si pasa -1 no debe romper.
func TestRetentionDays_NegativeFallsBackToDefault(t *testing.T) {
	s := &Service{RetentionDays: -5}
	require.Equal(t, 30, s.retentionDays())
}

// Comportamiento: valor custom positivo se respeta tal cual.
func TestRetentionDays_CustomValueRespected(t *testing.T) {
	s := &Service{RetentionDays: 7}
	require.Equal(t, 7, s.retentionDays())

	s = &Service{RetentionDays: 90}
	require.Equal(t, 90, s.retentionDays())
}

// Comportamiento: la ventana de retention se calcula como N*24h.
// Documenta la conversion: 30 dias = 30 * 24h = 720h.
func TestRetentionDuration_Conversion(t *testing.T) {
	s := &Service{RetentionDays: 30}
	dur := time.Duration(s.retentionDays()) * 24 * time.Hour
	require.Equal(t, 720*time.Hour, dur)

	s = &Service{RetentionDays: 7}
	dur = time.Duration(s.retentionDays()) * 24 * time.Hour
	require.Equal(t, 168*time.Hour, dur)
}

// Comportamiento: errores sentinels son distintos (los callers usan errors.Is).
// Si alguien refactorea y rompe la identidad, este test falla.
func TestSentinels_AreDistinct(t *testing.T) {
	require.NotEqual(t, ErrEntityNotSupported, ErrNotFound)
	require.NotEqual(t, ErrEntityNotSupported, ErrRetentionExpired)
	require.NotEqual(t, ErrNotFound, ErrRetentionExpired)
}

// Comportamiento: Restore() con entity no soportada → ErrEntityNotSupported
// sin tocar DB. Testeable con Service{} vacio (sin Pool).
func TestRestore_UnsupportedEntity_NoDBRequired(t *testing.T) {
	s := &Service{}
	err := s.Restore(nil, "no_existe", uuid.New(), uuid.New(), nil)
	require.ErrorIs(t, err, ErrEntityNotSupported)


}
