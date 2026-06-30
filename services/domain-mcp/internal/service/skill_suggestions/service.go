package skill_suggestions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/skill_suggestions/skillsuggestionsdb"
	"nunezlagos/domain/internal/store/txctx"
)

// Service expone el ciclo de vida de las sugerencias del judge.
//
//   - Create: persiste una sugerencia pending (cron o manual). Audit ActorSystem.
//   - Get / List: lectura para la UI/CLI.
//   - Approve / Reject: transicion pending -> approved/rejected (accion humana).
//   - Apply: ejecuta la mutacion sobre `skills` (split/merge/refine/archive).
//     SOLO se invoca por accion humana (handler POST approve+apply o CLI con
//     actor). Transaccional + rollback: si falla, la sugerencia queda 'approved'
//     con applied_at=NULL (reintentable).
//
// Refiner es opcional: lo necesita REFINE/SPLIT cuando el payload no trae el
// content ya resuelto y hay que generarlo via LLM. Si es nil y el payload no
// alcanza, Apply devuelve ErrApplyUnavailable (degradacion limpia, regla 7).
type Service struct {
	Pool     *pgxpool.Pool
	Audit    audit.Recorder
	Refiner  Refiner         // opcional (LLM para refine/split)
	Versions VersionRecorder // opcional (snapshots de skill_versions en refine)
}

// Refiner genera content via LLM para REFINE/SPLIT cuando el payload no lo trae.
// Lo implementa llm_judge.go (LLMJudge). Interface para testear Apply sin LLM.
type Refiner interface {
	// RefineContent devuelve el nuevo content para un skill dado su content
	// actual + la instruccion del payload. Error si el LLM no esta disponible.
	RefineContent(ctx context.Context, skillSlug, currentContent, instruction string) (string, error)
}

// VersionRecorder crea un snapshot inmutable en skill_versions (REFINE lo usa
// para que el cambio sea reversible) y devuelve el numero de version creado.
// Lo satisface un adapter sobre skill.VersionStore (ver wireup). Honra la
// tx-context: si se llama dentro de la tx del Apply, el snapshot es atomico.
type VersionRecorder interface {
	RecordVersion(ctx context.Context, skillID uuid.UUID, content *string, changelog *string, createdBy *uuid.UUID) (int, error)
}

// q resuelve el Queries contra la tx-context si existe, o el pool.
func (s *Service) q(ctx context.Context) *skillsuggestionsdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return skillsuggestionsdb.New(tx)
	}
	return skillsuggestionsdb.New(s.Pool)
}

// Create persiste una sugerencia pending. Dedup via UNIQUE parcial: si ya hay
// una pendiente (skill_slug, kind), la query devuelve 0 filas y aca lo tratamos
// como "ya existe" (no es error: nil, nil). Audit ActorSystem (lo genera el
// judge/cron). El payload_hash (SHA-256) va en el audit para trazabilidad.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Suggestion, error) {
	if !validKind(in.Kind) {
		return nil, ErrInvalidKind
	}
	payload := in.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}

	row, err := s.q(ctx).SuggestionCreate(ctx, skillsuggestionsdb.SuggestionCreateParams{
		SkillSlug:     in.SkillSlug,
		Kind:          in.Kind,
		Payload:       payload,
		Rationale:     in.Rationale,
		LlmModel:      in.LLMModel,
		LlmConfidence: floatPtrToNumeric(in.Confidence),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Ya existe una pendiente identica (dedup). No es error.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("create suggestion: %w", err)
	}

	sug := suggestionFromCreate(row)
	s.audit(ctx, audit.Event{
		ActorType:  audit.ActorSystem,
		Action:     "skill_suggestion.generated",
		EntityType: "skill_suggestion",
		EntityID:   &sug.ID,
		NewValues:  auditValues(sug, payload),
	})
	return sug, nil
}

// Get devuelve una sugerencia por id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Suggestion, error) {
	row, err := s.q(ctx).SuggestionGet(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get suggestion: %w", err)
	}
	return suggestionFromGet(row), nil
}

// List devuelve sugerencias filtradas por skill_slug/kind/status (vacios se
// ignoran), ordenadas por created_at DESC, paginadas.
func (s *Service) List(ctx context.Context, f ListFilter) ([]*Suggestion, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	if f.Kind != "" && !validKind(f.Kind) {
		return nil, ErrInvalidKind
	}

	rows, err := s.q(ctx).SuggestionList(ctx, skillsuggestionsdb.SuggestionListParams{
		SkillSlug:    f.SkillSlug,
		Kind:         f.Kind,
		Status:       f.Status,
		ResultLimit:  int32(limit),
		ResultOffset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list suggestions: %w", err)
	}
	out := make([]*Suggestion, len(rows))
	for i, r := range rows {
		out[i] = suggestionFromList(r)
	}
	return out, nil
}

// CountPending devuelve cuantas sugerencias estan pending (badge de la UI).
func (s *Service) CountPending(ctx context.Context) (int64, error) {
	n, err := s.q(ctx).SuggestionCountPending(ctx)
	if err != nil {
		return 0, fmt.Errorf("count pending: %w", err)
	}
	return n, nil
}

// Approve transiciona pending -> approved (accion humana). Guard optimista: si
// ya no esta pending devuelve ErrNotPending (concurrencia / ya revisada).
func (s *Service) Approve(ctx context.Context, id uuid.UUID, reviewer *uuid.UUID) (*Suggestion, error) {
	row, err := s.q(ctx).SuggestionApprove(ctx, skillsuggestionsdb.SuggestionApproveParams{
		ID:         id,
		ReviewedBy: reviewer,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return s.notPendingErr(ctx, id)
	}
	if err != nil {
		return nil, fmt.Errorf("approve: %w", err)
	}
	sug := suggestionFromApprove(row)
	s.audit(ctx, audit.Event{
		ActorType:  audit.ActorUser,
		ActorID:    reviewer,
		Action:     "skill_suggestion.approved",
		EntityType: "skill_suggestion",
		EntityID:   &sug.ID,
		OldValues:  map[string]any{"status": StatusPending},
		NewValues:  auditValues(sug, sug.Payload),
	})
	return sug, nil
}

// Reject transiciona pending -> rejected (accion humana). Mismo guard optimista.
func (s *Service) Reject(ctx context.Context, id uuid.UUID, reviewer *uuid.UUID) (*Suggestion, error) {
	row, err := s.q(ctx).SuggestionReject(ctx, skillsuggestionsdb.SuggestionRejectParams{
		ID:         id,
		ReviewedBy: reviewer,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return s.notPendingErr(ctx, id)
	}
	if err != nil {
		return nil, fmt.Errorf("reject: %w", err)
	}
	sug := suggestionFromReject(row)
	s.audit(ctx, audit.Event{
		ActorType:  audit.ActorUser,
		ActorID:    reviewer,
		Action:     "skill_suggestion.rejected",
		EntityType: "skill_suggestion",
		EntityID:   &sug.ID,
		OldValues:  map[string]any{"status": StatusPending},
		NewValues:  auditValues(sug, sug.Payload),
	})
	return sug, nil
}

// notPendingErr distingue "no existe" de "ya revisada" cuando un guard
// optimista devuelve 0 filas.
func (s *Service) notPendingErr(ctx context.Context, id uuid.UUID) (*Suggestion, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err // ErrNotFound u otro
	}
	return nil, ErrNotPending
}

// audit es best-effort (no bloquea el flujo); RecordOrLog loguea si falla.
func (s *Service) audit(ctx context.Context, e audit.Event) {
	audit.RecordOrLog(ctx, s.Audit, e)
}

// ---- conversion helpers ----

func suggestionFromCreate(r skillsuggestionsdb.SuggestionCreateRow) *Suggestion {
	return &Suggestion{
		ID: r.ID, SkillSlug: r.SkillSlug, Kind: r.Kind, Payload: r.Payload,
		Rationale: r.Rationale, LLMModel: r.LlmModel, LLMConfidence: confPtr(r.LlmConfidence),
		Status: r.Status, ReviewedBy: r.ReviewedBy, ReviewedAt: tsPtr(r.ReviewedAt),
		AppliedAt: tsPtr(r.AppliedAt), AppliedChanges: r.AppliedChanges, CreatedAt: r.CreatedAt,
	}
}

func suggestionFromGet(r skillsuggestionsdb.SuggestionGetRow) *Suggestion {
	return &Suggestion{
		ID: r.ID, SkillSlug: r.SkillSlug, Kind: r.Kind, Payload: r.Payload,
		Rationale: r.Rationale, LLMModel: r.LlmModel, LLMConfidence: confPtr(r.LlmConfidence),
		Status: r.Status, ReviewedBy: r.ReviewedBy, ReviewedAt: tsPtr(r.ReviewedAt),
		AppliedAt: tsPtr(r.AppliedAt), AppliedChanges: r.AppliedChanges, CreatedAt: r.CreatedAt,
	}
}

func suggestionFromList(r skillsuggestionsdb.SuggestionListRow) *Suggestion {
	return &Suggestion{
		ID: r.ID, SkillSlug: r.SkillSlug, Kind: r.Kind, Payload: r.Payload,
		Rationale: r.Rationale, LLMModel: r.LlmModel, LLMConfidence: confPtr(r.LlmConfidence),
		Status: r.Status, ReviewedBy: r.ReviewedBy, ReviewedAt: tsPtr(r.ReviewedAt),
		AppliedAt: tsPtr(r.AppliedAt), AppliedChanges: r.AppliedChanges, CreatedAt: r.CreatedAt,
	}
}

func suggestionFromApprove(r skillsuggestionsdb.SuggestionApproveRow) *Suggestion {
	return &Suggestion{
		ID: r.ID, SkillSlug: r.SkillSlug, Kind: r.Kind, Payload: r.Payload,
		Rationale: r.Rationale, LLMModel: r.LlmModel, LLMConfidence: confPtr(r.LlmConfidence),
		Status: r.Status, ReviewedBy: r.ReviewedBy, ReviewedAt: tsPtr(r.ReviewedAt),
		AppliedAt: tsPtr(r.AppliedAt), AppliedChanges: r.AppliedChanges, CreatedAt: r.CreatedAt,
	}
}

func suggestionFromReject(r skillsuggestionsdb.SuggestionRejectRow) *Suggestion {
	return &Suggestion{
		ID: r.ID, SkillSlug: r.SkillSlug, Kind: r.Kind, Payload: r.Payload,
		Rationale: r.Rationale, LLMModel: r.LlmModel, LLMConfidence: confPtr(r.LlmConfidence),
		Status: r.Status, ReviewedBy: r.ReviewedBy, ReviewedAt: tsPtr(r.ReviewedAt),
		AppliedAt: tsPtr(r.AppliedAt), AppliedChanges: r.AppliedChanges, CreatedAt: r.CreatedAt,
	}
}

func suggestionFromMarkApplied(r skillsuggestionsdb.SuggestionMarkAppliedRow) *Suggestion {
	return &Suggestion{
		ID: r.ID, SkillSlug: r.SkillSlug, Kind: r.Kind, Payload: r.Payload,
		Rationale: r.Rationale, LLMModel: r.LlmModel, LLMConfidence: confPtr(r.LlmConfidence),
		Status: r.Status, ReviewedBy: r.ReviewedBy, ReviewedAt: tsPtr(r.ReviewedAt),
		AppliedAt: tsPtr(r.AppliedAt), AppliedChanges: r.AppliedChanges, CreatedAt: r.CreatedAt,
	}
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// confPtr: las queries devuelven llm_confidence::float8 (NOT NULL en pgx -> 0
// para NULL). Distinguimos 0 real de NULL no es posible aca; tratamos 0 como
// "sin confianza" (el judge nunca persiste 0, descarta <0.6).
func confPtr(v float64) *float64 {
	if v == 0 {
		return nil
	}
	out := v
	return &out
}

// floatPtrToNumeric: *float64 -> pgtype.Numeric (Valid=false si nil = NULL).
func floatPtrToNumeric(p *float64) pgtype.Numeric {
	if p == nil {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.2f", *p))
	return n
}

// payloadHash computa SHA-256 hex del payload JSON canonicalizado (claves
// ordenadas) para trazabilidad inmutable en el audit_log.
func payloadHash(payload []byte) string {
	canon := canonicalJSON(payload)
	sum := sha256.Sum256(canon)
	return hex.EncodeToString(sum[:])
}

// canonicalJSON re-serializa el JSON con claves ordenadas (Go marshal de
// map[string]any ordena las claves) para que el hash sea estable. Si no parsea,
// hashea los bytes crudos.
func canonicalJSON(payload []byte) []byte {
	var v any
	if err := json.Unmarshal(payload, &v); err != nil {
		return payload
	}
	out, err := json.Marshal(v)
	if err != nil {
		return payload
	}
	return out
}

// auditValues arma el map que va en audit_log.new_values: metadata del modelo +
// payload_hash. NUNCA el payload crudo (puede ser grande / contener PII en
// rationale/content). slug + kind + model + confidence + hash bastan para
// trazar que sugirio el modelo vs que se aplico.
func auditValues(s *Suggestion, payload []byte) map[string]any {
	conf := 0.0
	if s.LLMConfidence != nil {
		conf = math.Round(*s.LLMConfidence*100) / 100
	}
	model := ""
	if s.LLMModel != nil {
		model = *s.LLMModel
	}
	return map[string]any{
		"skill_slug":     s.SkillSlug,
		"kind":           s.Kind,
		"status":         s.Status,
		"llm_model":      model,
		"llm_confidence": conf,
		"payload_hash":   payloadHash(payload),
	}
}
