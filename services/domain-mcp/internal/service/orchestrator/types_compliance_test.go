package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func fecha(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func ptr(t time.Time) *time.Time { return &t }

// REQ-2 — la severidad sale del catálogo: obligatorio + vigente es lo único que bloquea.
func TestSeveridadDe_LeyObligatoriaYVigente_Bloquea(t *testing.T) {
	sev := SeveridadDe(true, ptr(fecha(2020, time.January, 1)), fecha(2026, time.August, 7))

	assert.Equal(t, SeveridadBloqueante, sev)
	assert.True(t, sev.Bloquea())
}

// La Ley 21.719 rige recién desde 2026-12-01. Frenar hoy un cambio por algo que todavía no rige
// convertiría el gate en ruido y empujaría al waiver por fatiga.
func TestSeveridadDe_LeyObligatoriaAunNoVigente_SoloAdvierte(t *testing.T) {
	rigeEnDiciembre := ptr(fecha(2026, time.December, 1))

	sev := SeveridadDe(true, rigeEnDiciembre, fecha(2026, time.August, 7))

	assert.Equal(t, SeveridadAdvertencia, sev,
		"la obligación existe pero no rige: no puede detener el flow todavía")
	assert.False(t, sev.Bloquea())
}

// El día exacto en que entra en vigencia ya bloquea: Before() es estricto.
func TestSeveridadDe_ElDiaQueEntraEnVigencia_YaBloquea(t *testing.T) {
	rige := fecha(2026, time.December, 1)

	assert.Equal(t, SeveridadBloqueante, SeveridadDe(true, ptr(rige), rige))
}

// Una norma técnica es voluntaria: nunca bloquea, aunque esté declarada y vigente.
func TestSeveridadDe_NormaVoluntaria_NuncaBloquea(t *testing.T) {
	sev := SeveridadDe(false, nil, fecha(2026, time.August, 7))

	assert.Equal(t, SeveridadSugerencia, sev)
	assert.False(t, sev.Bloquea())
}

// Sin fecha de vigencia el marco ya rige: es el caso de las normas técnicas, que no tienen fecha
// de entrada en vigor. Tratarlo como "no vigente" las volvería inofensivas por accidente.
func TestSeveridadDe_SinFechaDeVigencia_SeConsideraVigente(t *testing.T) {
	assert.Equal(t, SeveridadBloqueante, SeveridadDe(true, nil, fecha(2026, time.August, 7)))
}

// REQ-1 — sin marcos declarados el veredicto es "no aplica", NO "ok".
//
// "ok" afirma que se evaluó algo. Decir que un proyecto cumple cuando nadie definió contra qué es
// la falsa sensación de cumplimiento que este diseño evita.
func TestVeredictoDe_SinMarcosDeclarados_EsNoAplicaYNoOk(t *testing.T) {
	v := VeredictoDe(0, nil)

	assert.Equal(t, VeredictoNoAplica, v)
	assert.NotEqual(t, VeredictoOK, v,
		"un proyecto sin marcos no 'cumple': no hay nada contra qué medirlo")
}

func TestVeredictoDe_ConMarcosYSinHallazgos_EsOk(t *testing.T) {
	assert.Equal(t, VeredictoOK, VeredictoDe(2, nil))
}

func TestVeredictoDe_UnBlockerSinWaiver_Bloquea(t *testing.T) {
	hallazgos := []HallazgoCompliance{
		{ControlSlug: "cifrado-en-reposo", Severidad: SeveridadAdvertencia},
		{ControlSlug: "plazos-de-retencion", Severidad: SeveridadBloqueante},
	}

	assert.Equal(t, VeredictoBloqueado, VeredictoDe(1, hallazgos))
}

// REQ-3 — el waiver destraba. Un gate sin válvula de escape se vuelve insatisfacible y empuja al
// bypass permanente (DOMAINSERV-111/175/195).
func TestVeredictoDe_BlockerConWaiver_NoBloquea(t *testing.T) {
	hallazgos := []HallazgoCompliance{
		{ControlSlug: "plazos-de-retencion", Severidad: SeveridadBloqueante, WaiverID: "w-1"},
	}

	v := VeredictoDe(1, hallazgos)

	assert.Equal(t, VeredictoConHallazgos, v,
		"con waiver el flow sigue, pero el hallazgo NO desaparece del reporte")
}

// Un waiver sobre UN hallazgo no destraba a los demás: cada uno necesita el suyo.
func TestVeredictoDe_WaiverParcial_SigueBloqueando(t *testing.T) {
	hallazgos := []HallazgoCompliance{
		{ControlSlug: "a", Severidad: SeveridadBloqueante, WaiverID: "w-1"},
		{ControlSlug: "b", Severidad: SeveridadBloqueante},
	}

	assert.Equal(t, VeredictoBloqueado, VeredictoDe(1, hallazgos))
}

// Warnings y suggestions no bloquean, pero tampoco dejan el veredicto en "ok": el reporte tiene
// que decir que hay algo.
func TestVeredictoDe_SoloWarningsYSuggestions_ConHallazgosPeroNoBloquea(t *testing.T) {
	hallazgos := []HallazgoCompliance{
		{Severidad: SeveridadAdvertencia},
		{Severidad: SeveridadSugerencia},
	}

	assert.Equal(t, VeredictoConHallazgos, VeredictoDe(3, hallazgos))
}
