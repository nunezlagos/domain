// Package skill_suggestions — HU-52.3: LLM-as-judge con human-in-the-loop.
//
// El judge (MiniMax-M3) analiza cada skill (metricas 30d + feedback + top-3
// similares por similitud lexica tsvector, SIN embeddings) y propone acciones:
// split | merge | refine | archive. NADA se auto-aplica (regla dura 6): el cron
// SOLO persiste sugerencias 'pending'; el Apply muta `skills` y corre
// EXCLUSIVAMENTE por accion humana (approve -> apply desde la UI/CLI).
//
// Separacion fisica deliberada:
//   - service.go: Create (usado por el cron y manualmente), Get, List, Approve,
//     Reject, Apply (solo accion humana). Idempotencia + rollback.
//   - llm_judge.go: el razonador LLM (arma prompt, parsea, valida threshold).
//   - el cron (internal/scheduler/cron/system) JAMAS llama Apply: solo Create.
//
// Single-tenant (regla dura 1): NADA de organization_id en skill_suggestions.
// La entidad natural es skill_slug (estable). Audit en cada Create/Apply.
package skill_suggestions

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Kind de sugerencia (refleja el CHECK de la mig 000182).
const (
	KindSplit   = "split"
	KindMerge   = "merge"
	KindRefine  = "refine"
	KindArchive = "archive"
)

// Status de una sugerencia (refleja el CHECK de la mig 000182).
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
	StatusApplied  = "applied"
)

// ConfidenceThreshold — solo se persisten sugerencias con confidence >= 0.6
// (la spec). El judge descarta las de menor confianza antes de Create.
const ConfidenceThreshold = 0.6

// MaxBatch — rate_limit: a lo sumo 50 sugerencias persistidas por corrida del
// judge (acota costo LLM y ruido).
const MaxBatch = 50

// DefaultListLimit / MaxListLimit acotan los listados de la UI/CLI.
const (
	DefaultListLimit = 50
	MaxListLimit     = 200
)

// Errores tipados del service.
var (
	ErrNotFound         = errors.New("skill suggestion not found")
	ErrInvalidKind      = errors.New("kind invalido (split|merge|refine|archive)")
	ErrNotPending       = errors.New("la sugerencia no esta pending (ya revisada)")
	ErrNotApproved      = errors.New("la sugerencia no esta approved (no aplicable)")
	ErrAlreadyApplied   = errors.New("la sugerencia ya fue aplicada")
	ErrSkillNotFound    = errors.New("skill objetivo no encontrado o ya borrado")
	ErrSeedManaged      = errors.New("no se puede archivar/dividir un skill seed_managed")
	ErrApplyUnavailable = errors.New("apply de refine/split requiere LLM (MINIMAX_API_KEY) o content en payload")
)

// Suggestion es una sugerencia del judge tal como se expone aguas arriba.
//   - Confidence es puntero: nil = no reportada por el LLM.
//   - Payload/AppliedChanges son JSON crudo (la forma depende de Kind).
type Suggestion struct {
	ID             uuid.UUID  `json:"id"`
	SkillSlug      string     `json:"skill_slug"`
	Kind           string     `json:"kind"`
	Payload        []byte     `json:"payload"`
	Rationale      *string    `json:"rationale,omitempty"`
	LLMModel       *string    `json:"llm_model,omitempty"`
	LLMConfidence  *float64   `json:"llm_confidence,omitempty"`
	Status         string     `json:"status"`
	ReviewedBy     *uuid.UUID `json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	AppliedAt      *time.Time `json:"applied_at,omitempty"`
	AppliedChanges []byte     `json:"applied_changes,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// CreateInput parametriza Create. El cron arma esto desde la salida del judge.
//   - Payload: propuesta concreta (forma segun Kind, ver mig 000182).
//   - Confidence: nil OK; si <0.6 el caller (judge) ya la descarto.
type CreateInput struct {
	SkillSlug  string
	Kind       string
	Payload    []byte
	Rationale  *string
	LLMModel   *string
	Confidence *float64
}

// ListFilter filtros opcionales de List. Campos vacios se ignoran.
type ListFilter struct {
	SkillSlug string
	Kind      string
	Status    string
	Limit     int
	Offset    int
}

// ApplyResult resume el efecto de un Apply (lo que se persiste en applied_changes
// y se audita). CreatedSkills: slugs nuevos (split/merge). SupersededSlugs:
// originales soft-deleted. RefineVersion: version creada por refine. Archived:
// slug archivado.
type ApplyResult struct {
	Kind            string   `json:"kind"`
	CreatedSkills   []string `json:"created_skills,omitempty"`
	SupersededSlugs []string `json:"superseded_slugs,omitempty"`
	ArchivedSlug    string   `json:"archived_slug,omitempty"`
	RefineVersion   *int     `json:"refine_version,omitempty"`
}

// validKind valida contra el CHECK de la migracion.
func validKind(k string) bool {
	switch k {
	case KindSplit, KindMerge, KindRefine, KindArchive:
		return true
	}
	return false
}
