// Package skill_ab_test — HU-52.4: A/B testing de prompts (traffic split entre
// dos versiones de un skill).
//
// Backend puro. Repository + Service pattern, igual que skill_metrics/
// skill_suggestions. El paquete tiene tres piezas:
//
//   - Service (service.go): Create / Start / GetResults / DeclareWinner / Cancel.
//   - Router (router.go): dado (skill_slug, user_id) en request-time, decide la
//     variante de forma DETERMINISTA via SHA-256(skill_slug+user_id)%100 contra
//     traffic_split_a. Si no hay test 'running' para el slug -> pin normal.
//   - Analyzer (analyzer.go): ESTADISTICA PURA (z-test de proporciones de dos
//     muestras), SIN LLM ni deps externas. Decide winner ('a'|'b') o
//     'inconclusive' segun alpha=0.05.
//
// Single-tenant (regla dura 1): NADA de organization_id. La entidad natural es el
// skill_slug. El pin del ganador (auto_apply) usa skills.pinned_version, JAMAS
// organization_id (no existe en skills en runtime desde mig 000142).
package skill_ab_test

import (
	"time"

	"github.com/google/uuid"
)

// DefaultTrafficSplitA es el split por defecto (50/50).
const DefaultTrafficSplitA = 0.50

// DefaultMinInvocations es el minimo de invocaciones POR VARIANTE antes de que
// el Analyzer corra el z-test.
const DefaultMinInvocations = 100

// DefaultAlpha es el nivel de significancia del z-test (5%).
const DefaultAlpha = 0.05

// Variant identifica la variante servida.
type Variant string

const (
	VariantA Variant = "a"
	VariantB Variant = "b"
)

// Winner posibles resultados del Analyzer.
const (
	WinnerA            = "a"
	WinnerB            = "b"
	WinnerInconclusive = "inconclusive"
)

// Status de un experimento.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

// ABTest es un experimento A/B. version_a/version_b son numeros de
// skill_versions; traffic_split_a la fraccion 0..1 del trafico a la variante 'a'.
type ABTest struct {
	ID              uuid.UUID  `json:"id"`
	SkillSlug       string     `json:"skill_slug"`
	VersionA        int        `json:"version_a"`
	VersionB        int        `json:"version_b"`
	TrafficSplitA   float64    `json:"traffic_split_a"`
	MinInvocations  int        `json:"min_invocations"`
	AutoApplyWinner bool       `json:"auto_apply_winner"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	Winner          *string    `json:"winner,omitempty"`
	Confidence      *float64   `json:"confidence,omitempty"`
	Status          string     `json:"status"`
	CreatedBy       *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// VersionFor devuelve el numero de version de skill_versions para una variante.
func (t *ABTest) VersionFor(v Variant) int {
	if v == VariantA {
		return t.VersionA
	}
	return t.VersionB
}

// VariantResult es el agregado de una variante (lo que consume el Analyzer).
type VariantResult struct {
	Version          string    `json:"version"`
	InvocationsCount int       `json:"invocations_count"`
	SuccessCount     int       `json:"success_count"`
	SuccessRate      *float64  `json:"success_rate,omitempty"`
	AvgFeedback      *float64  `json:"avg_feedback,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreateParams entrada de Service.Create.
type CreateParams struct {
	SkillSlug       string
	VersionA        int
	VersionB        int
	TrafficSplitA   float64 // 0..1; <=0 usa DefaultTrafficSplitA
	MinInvocations  int     // <=0 usa DefaultMinInvocations
	AutoApplyWinner bool
	StartNow        bool // si true, setea started_at=NOW() al crear
	CreatedBy       *uuid.UUID
}
