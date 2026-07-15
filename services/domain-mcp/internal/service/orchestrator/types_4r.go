// Tipos del contrato de la fase sdd-4r (code review por 4 lenses:
// R1 Risk, R2 Readability, R3 Reliability, R4 Resilience). Solo
// declaraciones; la lógica de Build/Validate vive en phases/sdd_4r.go
// y el fan-out por lens lo ejecuta el cliente (controller con toda la
// autoridad). Ver epic DOMAINSERV-4.
package orchestrator

// Severity clasifica un Finding por impacto. Solo los severos
// candidate-caused (BLOCKER/CRITICAL) pueden bloquear; WARNING y
// SUGGESTION son informativos y nunca agendan fixes.
type Severity string

const (
	SeverityBlocker    Severity = "BLOCKER"
	SeverityCritical   Severity = "CRITICAL"
	SeverityWarning    Severity = "WARNING"
	SeveritySuggestion Severity = "SUGGESTION"
)

// EvidenceClass indica cómo se sostiene un Finding: deterministic es
// verificable de forma directa, inferential requiere refuter antes de
// bloquear, insufficient no alcanza para accionar.
type EvidenceClass string

const (
	EvidenceDeterministic EvidenceClass = "deterministic"
	EvidenceInferential   EvidenceClass = "inferential"
	EvidenceInsufficient  EvidenceClass = "insufficient"
)

// CausalDisposition ubica el origen del Finding respecto al cambio bajo
// review. Solo introduced/behavior-activated/worsened son candidate-caused
// y pueden bloquear; el resto es contexto pre-existente o desconocido.
type CausalDisposition string

const (
	CausalIntroduced        CausalDisposition = "introduced"
	CausalBehaviorActivated CausalDisposition = "behavior-activated"
	CausalWorsened          CausalDisposition = "worsened"
	CausalPreExisting       CausalDisposition = "pre-existing"
	CausalBaseOnly          CausalDisposition = "base-only"
	CausalUnknown           CausalDisposition = "unknown"
)

// RiskTier resume el nivel de riesgo del cambio. Insumo para el ruteo
// por tier (cuántas lenses disparar); declarado acá como parte del
// contrato aunque el ruteo llegue en una iteración posterior.
type RiskTier string

const (
	RiskLow    RiskTier = "low"
	RiskMedium RiskTier = "medium"
	RiskHigh   RiskTier = "high"
)

// Finding es un hallazgo de una lens 4R. ProofRefs lleva referencias
// verificables (changed-hunk:/candidate-created-path:...) que sostienen
// el hallazgo: un Finding sin proof no puede bloquear.
type Finding struct {
	ID                string
	Location          string
	Severity          Severity
	EvidenceClass     EvidenceClass
	CausalDisposition CausalDisposition
	ProofRefs         []string
}

// FourRResult es el contrato que el controller sintetiza por lens. Un
// resultado 'clean' exige Findings vacío PERO Evidence no vacío (prueba
// de que la lens efectivamente revisó el scope).
type FourRResult struct {
	Lens     string
	Findings []Finding
	Evidence []string
}
