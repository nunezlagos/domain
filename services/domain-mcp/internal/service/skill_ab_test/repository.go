package skill_ab_test

import (
	"context"

	"github.com/google/uuid"
)

// Repository contrato de persistencia/lectura de skill_ab_tests. La impl
// concreta (pg_repository.go) honra la tx-context. Errores not-found se devuelven
// como (nil, nil) en los getters singulares para que el caller decida.
type Repository interface {
	// Create persiste un experimento nuevo (status 'running').
	Create(ctx context.Context, p CreateParams) (*ABTest, error)
	// Get devuelve un test por id; (nil,nil) si no existe.
	Get(ctx context.Context, id uuid.UUID) (*ABTest, error)
	// GetRunningBySlug devuelve el test 'running' del slug; (nil,nil) si no hay.
	GetRunningBySlug(ctx context.Context, slug string) (*ABTest, error)
	// ListRunning devuelve todos los tests 'running' (el cron itera sobre estos).
	ListRunning(ctx context.Context) ([]*ABTest, error)

	// Start setea started_at=NOW() si el test sigue running.
	Start(ctx context.Context, id uuid.UUID) error
	// DeclareWinner cierra el test con un ganador y confidence.
	DeclareWinner(ctx context.Context, id uuid.UUID, winner string, confidence *float64) error
	// Cancel marca el test 'cancelled'.
	Cancel(ctx context.Context, id uuid.UUID) error

	// GetResults devuelve los agregados de ambas variantes.
	GetResults(ctx context.Context, testID uuid.UUID) ([]VariantResult, error)
	// IncrementResult suma una invocacion (y exito) a una variante (Router en caliente).
	IncrementResult(ctx context.Context, testID uuid.UUID, v Variant, success bool) error
	// UpsertResult reemplaza el agregado de una variante (recompute del analyzer).
	UpsertResult(ctx context.Context, testID uuid.UUID, v VariantResult) error

	// SkillIDBySlug resuelve el skill_id vivo desde el slug; uuid.Nil si no existe.
	SkillIDBySlug(ctx context.Context, slug string) (uuid.UUID, error)
	// PinSkillVersion pinea una version en el skill (auto_apply del ganador).
	// Single-tenant: JAMAS usa organization_id.
	PinSkillVersion(ctx context.Context, skillID uuid.UUID, version int) error
}
