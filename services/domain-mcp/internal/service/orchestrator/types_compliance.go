// Tipos y reglas del contrato de la fase sdd-compliance (issue-56.5).
//
// La fase tiene handler y autoridad propios porque sdd-4r NO PUEDE cumplir este rol, y no por
// preferencia de diseño: su prompt declara "el controller tiene toda la autoridad, esta fase no
// bloquea", y su template r1_shift_left excluye por regla dura todo hallazgo pre-existing. Una
// obligación de compliance es un ESTADO DEL SISTEMA —"no hay RAT", "no se declaró retención"— y no
// una propiedad del diff, así que dentro de R1 quedaría muda por diseño.
package orchestrator

import "time"

// SeveridadCompliance clasifica el incumplimiento de una obligación.
type SeveridadCompliance string

const (
	// SeveridadBloqueante — ley obligatoria y VIGENTE incumplida. Es la única que detiene.
	SeveridadBloqueante SeveridadCompliance = "BLOCKER"
	// SeveridadAdvertencia — obligación real, pero el marco todavía no rige.
	SeveridadAdvertencia SeveridadCompliance = "WARNING"
	// SeveridadSugerencia — norma voluntaria (ISO, salvo que un contrato la exija).
	SeveridadSugerencia SeveridadCompliance = "SUGGESTION"
)

// Bloquea responde si esta severidad detiene el flow.
func (s SeveridadCompliance) Bloquea() bool { return s == SeveridadBloqueante }

// SeveridadDe deriva la severidad del CATÁLOGO y no de una tabla propia de severidades.
//
// Esa decisión evita un incumplimiento silencioso por configuración: con una tabla aparte alguien
// podría dejar una ley obligatoria marcada como sugerencia y nada lo delataría. Acá la severidad es
// función de dos hechos que ya viven en compliance_frameworks —si el marco obliga y si ya rige—,
// así que no puede desalinearse de ellos.
//
// vigenteDesde nil significa "ya rige": es el caso de las normas técnicas, que no tienen fecha de
// entrada en vigor.
func SeveridadDe(obligatorio bool, vigenteDesde *time.Time, ahora time.Time) SeveridadCompliance {
	if !obligatorio {
		return SeveridadSugerencia
	}
	// la Ley 21.719 rige recién desde 2026-12-01: la obligación existe, pero frenar hoy un
	// cambio por algo que todavía no rige convertiría el gate en ruido y empujaría al waiver
	if vigenteDesde != nil && ahora.Before(*vigenteDesde) {
		return SeveridadAdvertencia
	}
	return SeveridadBloqueante
}

// HallazgoCompliance es una obligación sin satisfacer, con la cita del marco que la exige.
type HallazgoCompliance struct {
	ControlSlug   string              `json:"control_slug"`
	FrameworkSlug string              `json:"framework_slug"`
	Referencia    string              `json:"referencia,omitempty"`
	Severidad     SeveridadCompliance `json:"severidad"`
	Detalle       string              `json:"detalle,omitempty"`
	// WaiverID apunta al waiver vigente que lo destraba, si hay uno.
	WaiverID string `json:"waiver_id,omitempty"`
}

// Veredictos posibles de la fase.
const (
	VeredictoNoAplica     = "not_applicable"
	VeredictoOK           = "ok"
	VeredictoConHallazgos = "con_hallazgos"
	VeredictoBloqueado    = "bloqueado"
)

// ResultadoCompliance es el contrato que el cliente reporta al cerrar la fase.
type ResultadoCompliance struct {
	Veredicto string               `json:"veredicto"`
	Hallazgos []HallazgoCompliance `json:"hallazgos"`
	// ControlesExigidos viaja a sdd-4r por PriorOutputs: sdd-compliance decide QUÉ se exige y R1
	// verifica que el diff no lo viole, sin duplicar trabajo ni tocar su scoping por changed-hunk.
	ControlesExigidos []string `json:"controles_exigidos"`
	MarcosEvaluados   []string `json:"marcos_evaluados"`
}

// VeredictoDe resuelve el veredicto a partir de los hallazgos y de si el proyecto declaró marcos.
//
// Un proyecto sin marcos declarados NO es "ok": es "no aplica". La diferencia importa porque "ok"
// afirma que se evaluó algo, y decir que un proyecto cumple cuando nadie definió contra qué es
// exactamente la falsa sensación de cumplimiento que este diseño evita.
func VeredictoDe(marcosDeclarados int, hallazgos []HallazgoCompliance) string {
	if marcosDeclarados == 0 {
		return VeredictoNoAplica
	}
	for _, h := range hallazgos {
		if h.Severidad.Bloquea() && h.WaiverID == "" {
			return VeredictoBloqueado
		}
	}
	if len(hallazgos) > 0 {
		return VeredictoConHallazgos
	}
	return VeredictoOK
}
