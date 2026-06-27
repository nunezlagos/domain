package skill_ab_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"log/slog"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/audit"
)

// router.go — enrutamiento determinista de variantes en request-time.
//
// El bucket de un usuario para un slug es:
//
//	bucket = SHA-256(skill_slug + ":" + user_id) % 100   (0..99)
//
// Si bucket < traffic_split_a*100 -> variante 'a' (version_a), si no -> 'b'.
// DETERMINISTA: mismo (slug,user) -> misma variante siempre, sin estado. user_id
// viene del Principal en request-time (skill_executions NO tiene caller).
//
// Si NO hay test 'running' para el slug -> Decision.InABTest=false y el caller
// usa el pin normal del skill.

// Decision es el resultado del enrutamiento para una request.
type Decision struct {
	// InABTest=false: no hay test running; usar el pin normal del skill.
	InABTest bool
	// TestID del experimento (zero si !InABTest).
	TestID uuid.UUID
	// Variant servida ("a"|"b"); vacia si !InABTest.
	Variant Variant
	// Version de skill_versions a pinear para esta request; 0 si !InABTest.
	Version int
}

// RouterRepository es el subconjunto de persistencia que el Router necesita.
type RouterRepository interface {
	GetRunningBySlug(ctx context.Context, slug string) (*ABTest, error)
	IncrementResult(ctx context.Context, testID uuid.UUID, v Variant, success bool) error
}

// Router decide la variante para (skill_slug, user_id). Loguea la decision en
// audit (slug, version, user_id) — NUNCA el input del skill.
type Router struct {
	repo     RouterRepository
	recorder audit.Recorder
	logger   *slog.Logger
}

// NewRouter inyecta el repo, el audit recorder (puede ser nil) y el logger.
func NewRouter(repo RouterRepository, recorder audit.Recorder, logger *slog.Logger) *Router {
	if logger == nil {
		logger = slog.Default()
	}
	return &Router{repo: repo, recorder: recorder, logger: logger}
}

// Bucket computa el bucket determinista 0..99 para (slug, userID).
// Funcion PURA (testeable sin DB): mismo input -> mismo output siempre.
func Bucket(skillSlug string, userID uuid.UUID) int {
	h := sha256.Sum256([]byte(skillSlug + ":" + userID.String()))
	// Tomamos los primeros 8 bytes como uint64 big-endian para un entero estable.
	n := binary.BigEndian.Uint64(h[:8])
	return int(n % 100)
}

// pick decide la variante segun el bucket y el split. Funcion PURA.
//
//	bucket < trafficSplitA*100 -> 'a', si no -> 'b'.
//
// trafficSplitA se clampa a [0,1]. split=0 -> siempre 'b'; split=1 -> siempre 'a'.
func pick(bucket int, trafficSplitA float64) Variant {
	if trafficSplitA < 0 {
		trafficSplitA = 0
	}
	if trafficSplitA > 1 {
		trafficSplitA = 1
	}
	if float64(bucket) < trafficSplitA*100 {
		return VariantA
	}
	return VariantB
}

// Route resuelve la variante para (skillSlug, userID) en request-time.
//
//	- Si no hay test 'running' para el slug -> Decision{InABTest:false} (pin normal).
//	- Si lo hay -> bucketea, elige variante, loguea en audit y devuelve la version.
//
// Degradacion (regla dura 5): si GetRunningBySlug falla, NO rompe la request:
// loguea y devuelve InABTest=false para caer al pin normal.
func (r *Router) Route(ctx context.Context, skillSlug string, userID uuid.UUID) Decision {
	test, err := r.repo.GetRunningBySlug(ctx, skillSlug)
	if err != nil {
		r.logger.Warn("ab-router: lookup fallo, usando pin normal",
			slog.String("skill_slug", skillSlug), slog.Any("err", err))
		return Decision{InABTest: false}
	}
	if test == nil {
		return Decision{InABTest: false}
	}

	variant := pick(Bucket(skillSlug, userID), test.TrafficSplitA)
	version := test.VersionFor(variant)

	r.audit(ctx, skillSlug, variant, version, userID)

	return Decision{
		InABTest: true,
		TestID:   test.ID,
		Variant:  variant,
		Version:  version,
	}
}

// RecordOutcome registra el resultado (exito/fallo) de una invocacion enrutada,
// incrementando los contadores de la variante. Best-effort: un fallo aqui no
// debe tumbar la request (lo loguea).
func (r *Router) RecordOutcome(ctx context.Context, d Decision, success bool) {
	if !d.InABTest {
		return
	}
	if err := r.repo.IncrementResult(ctx, d.TestID, d.Variant, success); err != nil {
		r.logger.Warn("ab-router: increment result fallo",
			slog.String("test_id", d.TestID.String()),
			slog.String("variant", string(d.Variant)),
			slog.Any("err", err))
	}
}

// audit loguea la asignacion de variante. NUNCA loguea el input del skill: solo
// (skill_slug, version, user_id) en NewValues. Best-effort (RecordOrLog).
func (r *Router) audit(ctx context.Context, skillSlug string, v Variant, version int, userID uuid.UUID) {
	if r.recorder == nil {
		return
	}
	actor := userID
	audit.RecordOrLog(ctx, r.recorder, audit.Event{
		ActorID:    &actor,
		ActorType:  audit.ActorUser,
		Action:     "skill.ab_test.routed",
		EntityType: "skill_ab_test",
		NewValues: map[string]any{
			"skill_slug": skillSlug,
			"variant":    string(v),
			"version":    version,
			"user_id":    userID.String(),
		},
	})
}
