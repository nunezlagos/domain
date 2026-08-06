package flow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-218 REQ-218.7. El TTL pasa a medir INACTIVIDAD y no duración de tarea: mientras el
// agente valide, su autorización se corre hacia adelante; si se queda quieto un TTL entero,
// vence. Sin esto, una fase única más larga que el TTL —un sdd-apply grande— pierde la
// autorización a mitad de camino aunque el agente esté trabajando.

func TestNecesitaRenovacion_VigenciaBajoElUmbral_RenuevaSi(t *testing.T) {
	ahora := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	expira := ahora.Add(10 * time.Minute)

	require.True(t, NecesitaRenovacion(expira.Unix(), ahora))
}

func TestNecesitaRenovacion_VigenciaHolgada_NoRenueva(t *testing.T) {
	ahora := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	expira := ahora.Add(25 * time.Minute)

	require.False(t, NecesitaRenovacion(expira.Unix(), ahora),
		"renovar con vigencia holgada escribiría en la base en cada pre-edit, que es camino caliente")
}

// El umbral es un borde y se fija explícitamente: justo en 15 min todavía NO renueva, así que
// mover la constante rompe este test en vez de cambiar el comportamiento en silencio.
func TestNecesitaRenovacion_JustoEnElUmbral_NoRenueva(t *testing.T) {
	ahora := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	expira := ahora.Add(UmbralDeRenovacion)

	require.False(t, NecesitaRenovacion(expira.Unix(), ahora))
}

// Un token ya vencido no se renueva: eso sería resucitarlo. ValidateToken lo rechaza antes,
// pero la función tiene que ser correcta por sí sola.
func TestNecesitaRenovacion_TokenVencido_NoRenueva(t *testing.T) {
	ahora := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	expira := ahora.Add(-1 * time.Minute)

	require.False(t, NecesitaRenovacion(expira.Unix(), ahora),
		"renovar un token vencido lo resucitaría, y el TTL dejaría de acotar la inactividad")
}

func TestUmbralDeRenovacion_EsMenorQueElTTL(t *testing.T) {
	require.Less(t, UmbralDeRenovacion, FlowTokenTTL,
		"un umbral >= TTL renovaría en cada validación y el camino caliente escribiría siempre")
}
