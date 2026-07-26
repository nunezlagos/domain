// Package observation — issue-03.1 CRUD de observations + búsqueda híbrida.
//
// Observations son la unidad central de memoria. Cada una vive en un project
// dentro de una organization. Búsqueda híbrida combina:
//   - BM25 (ts_rank con índice GIN sobre content_tsv en español)
//   - cosine (operador <=> de pgvector sobre embedding)
// fusionados con Reciprocal Rank Fusion (RRF): score = sum(1 / (k + rank_i)).
//
// Si el embedding es vector zero (NopEmbedder), search degrada a tsvector-only.
//
// HU-28.1: Service depende de Repository (interfaz) en vez de *pgxpool.Pool.
// El campo Pool se mantiene público como deprecated para Strangler Fig
// (callers que aún construyen &Service{Pool: ...} siguen funcionando — el
// helper repository() inicializa pgRepository on-demand desde Pool).
package observation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/memory/dedup"
	"nunezlagos/domain/internal/memory/privacy"
)

var (
	ErrNotFound          = errors.New("observation not found")
	ErrContentRequired   = errors.New("content required")
	ErrProjectMismatch   = errors.New("project does not belong to organization")
	ErrDuplicate         = errors.New("duplicate observation (content_hash already exists)")
)

type Observation struct {
	ID              uuid.UUID
	ProjectID       uuid.UUID
	CreatedBy       *uuid.UUID
	SessionID       *uuid.UUID
	Content         string
	ObservationType string
	Tags            []string
	Metadata        map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SaveInput struct {
	OrganizationID  uuid.UUID
	ProjectID       uuid.UUID
	CreatedBy       *uuid.UUID
	SessionID       *uuid.UUID
	Content         string
	ObservationType string // default "note"
	Tags            []string
	Metadata        map[string]any
}

type SearchResult struct {
	Observation
	Score      float64 // RRF combined score
	BM25Rank   int     // 0 si no apareció en BM25
	VectorRank int     // 0 si no apareció en vector
}

// EventEmitter publica eventos de dominio hacia webhooks outbound
// (issue-10.4 ow-002). Opcional.
type EventEmitter interface {
	EmitEntityEvent(ctx context.Context, orgID uuid.UUID, eventType string, data map[string]any)
}

type Service struct {



	Pool     *pgxpool.Pool
	Audit    audit.Recorder
	Embedder llm.Embedder
	Events   EventEmitter // nil = sin webhooks

	// LLM es el Factory de providers para la inferencia de inference.go (RoleInfer),
	// que corre FUERA del camino de búsqueda. Opcional: si es nil, esa inferencia
	// degrada sin error. Se inyecta post-construcción porque el Factory se arma
	// después del Service en el wiring (ver cmd/*).
	//
	// El rerank de búsqueda se retiró en DOMAINSERV-116: mem_search no puede
	// depender de inferencia porque siempre tiene a alguien esperando.
	LLM *llm.Factory

	repo Repository
}

// NewService construye el Service con dependencias explícitas. Si repo es nil,
// se construye un pgRepository wrappeando pool (back-compat).
func NewService(pool *pgxpool.Pool, audit audit.Recorder, embedder llm.Embedder, events EventEmitter, repo Repository) *Service {
	if repo == nil && pool != nil {
		repo = NewPgRepository(pool)
	}
	return &Service{
		Pool:     pool,
		Audit:    audit,
		Embedder: embedder,
		Events:   events,
		repo:     repo,
	}
}

// repository retorna la Repository inyectada o crea una pgRepository on-demand
// desde Pool (compat con construcción por struct literal).
func (s *Service) repository() Repository {
	if s.repo != nil {
		return s.repo
	}
	s.repo = NewPgRepository(s.Pool)
	return s.repo
}

// Save crea observation con embedding generado en línea.
// Aplica privacy stripping (<private>...</private>) y dedup hash check
// (UNIQUE constraint en DB = defense in depth contra bypass de la app).
func (s *Service) Save(ctx context.Context, in SaveInput) (*Observation, error) {
	if strings.TrimSpace(in.Content) == "" {
		return nil, ErrContentRequired
	}
	if in.ObservationType == "" {
		in.ObservationType = "note"
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}


	cleanContent, redactedCount := privacy.Strip(in.Content)
	if redactedCount > 0 {
		in.Metadata["privacy_redacted_blocks"] = redactedCount
	}
	if strings.TrimSpace(cleanContent) == "" {
		return nil, ErrContentRequired
	}

	metaJSON, _ := json.Marshal(in.Metadata)


	hash := dedup.Hash(dedup.FingerprintInput{
		ProjectID:       in.ProjectID,
		ObservationType: in.ObservationType,
		Title:           "",
		Content:         cleanContent,
	})


	vec, err := s.Embedder.Embed(ctx, cleanContent)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	embedLit := vectorLiteral(vec)

	o, err := s.repository().Insert(ctx, InsertParams{
		OrganizationID:  in.OrganizationID,
		ProjectID:       in.ProjectID,
		CreatedBy:       in.CreatedBy,
		SessionID:       in.SessionID,
		Content:         cleanContent,
		EmbeddingLit:    embedLit,
		ObservationType: in.ObservationType,
		Tags:            in.Tags,
		MetadataJSON:    metaJSON,
		ContentHash:     hash,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "knowledge_observations_dedup_hash_uniq" {
				return nil, ErrDuplicate
			}
			if pgErr.Code == pgerrcode.ForeignKeyViolation && strings.Contains(pgErr.ConstraintName, "project") {
				return nil, ErrProjectMismatch
			}
		}
		return nil, fmt.Errorf("insert observation: %w", err)
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        in.CreatedBy,
			ActorType:      audit.ActorUser,
			Action:         "observation.saved",
			EntityType:     "observation",
			EntityID:       &o.ID,
			NewValues:      map[string]any{"redacted": redactedCount},
		})
	}

	if s.Events != nil {
		s.Events.EmitEntityEvent(ctx, in.OrganizationID, "observation.created", map[string]any{
			"observation_id": o.ID,
			"project_id":     in.ProjectID,
			"type":           o.ObservationType,
		})
	}
	return o, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Observation, error) {
	return s.repository().Get(ctx, id)
}

// List lista observations del project, más recientes primero.
func (s *Service) List(ctx context.Context, projectID uuid.UUID, limit int) ([]Observation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repository().List(ctx, projectID, limit)
}

// ListPageInput describe filtros + paginación cursor-based (issue-13.6).
type ListPageInput struct {
	ProjectID      uuid.UUID
	Limit          int
	SortDesc       bool       // true = DESC (default), false = ASC
	CursorTime     *time.Time // si != nil, paginar desde este punto
	CursorID       *uuid.UUID // tie-breaker para estabilidad
}

// ListPaginated implementa keyset pagination estable por (created_at, id).
// Devuelve hasta limit+1 rows internamente para detectar has_more sin extra query.
func (s *Service) ListPaginated(ctx context.Context, in ListPageInput) ([]Observation, bool, error) {
	if in.Limit <= 0 || in.Limit > 200 {
		in.Limit = 50
	}
	return s.repository().ListPaginated(ctx, in)
}

// SoftDelete marca deleted_at.
func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	if err := s.repository().SoftDelete(ctx, id); err != nil {
		return err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:    &actorID,
			ActorType:  audit.ActorUser,
			Action:     "observation.deleted",
			EntityType: "observation",
			EntityID:   &id,
		})
	}
	return nil
}

// SearchHybrid combina BM25 y cosine con RRF (Reciprocal Rank Fusion).
// k=60 es el default standard en literatura RRF.
// Si embedder es Nop (zero vector), search degrada a tsvector-only.
func (s *Service) SearchHybrid(ctx context.Context, orgID uuid.UUID, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	vec, err := s.Embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	useVector := !llm.IsZero(vec)

	const rrfK = 60
	const candidates = 100 // por cada modalidad

	return s.repository().SearchHybrid(ctx, SearchInput{
		OrgID:        orgID,
		Query:        query,
		EmbeddingLit: vectorLiteral(vec),
		UseVector:    useVector,
		Limit:        limit,
		Candidates:   candidates,
		RRFK:         rrfK,
	})
}

// vectorLiteral convierte []float32 a literal '[v1,v2,...]' para pgvector.
// vectorLiteral serializa el embedding para el INSERT. Devuelve "[]" cuando no
// hay embedding útil, y el CASE de sql/query.sql:7 lo traduce a NULL.
//
// DOMAINSERV-157: el vector CERO cuenta como "no hay". El NopEmbedder devuelve
// un slice de N ceros —no vacío—, así que sin este guard se serializaba
// "[0,0,...]" y quedaba persistido como si fuera un embedding legítimo. En prod
// eso dejó 44 observaciones con embedding NO NULL y norma 0.
//
// No basta con que hoy la búsqueda degrade limpio: ese guard (useVector) mira el
// vector de la QUERY, mientras que el CTE `vec` filtra por `embedding IS NOT
// NULL` sin mirar la norma. Los ceros almacenados se activan como ruido recién
// cuando el embedder vuelve a funcionar.
func vectorLiteral(v []float32) string {
	if len(v) == 0 || llm.IsZero(v) {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", f)
	}
	sb.WriteByte(']')
	return sb.String()
}

// maxUsageIDsPerTurn acota el payload del reporte de consumo. El hook inyecta
// un puñado de memorias por turno, así que una lista mucho más larga es señal
// de un cliente mal implementado, no de uso real. Se recorta en vez de fallar:
// un reporte parcial sigue siendo útil.
const maxUsageIDsPerTurn = 20

// RecordUsage persiste qué observaciones se le mostraron al cliente en un turno
// y cuáles dice haber usado (DOMAINSERV-145).
//
// La ausencia de reporte significa "sin dato", NUNCA "no sirvió": por eso un
// reporte con usadas vacías SÍ se registra —"se mostraron y no sirvieron" es
// información— y un turno que nunca reporta simplemente no deja filas.
//
// Devuelve cuántas se registraron y cuántas se descartaron.
func (s *Service) RecordUsage(ctx context.Context, promptID uuid.UUID, candidateIDs, usedIDs []uuid.UUID) (int, int, error) {
	if promptID == uuid.Nil {
		return 0, 0, fmt.Errorf("prompt_id requerido")
	}
	// used ⊆ candidates: una usada no declarada se suma al denominador en vez
	// de descartarse, o la tasa usados/candidatos podría superar 1 y dejaría de
	// ser interpretable.
	candidatos, descartados := normalizarIDs(append(append([]uuid.UUID{}, candidateIDs...), usedIDs...))
	usadas, _ := normalizarIDs(usedIDs)
	if len(candidatos) == 0 {
		return 0, descartados, nil
	}

	existe, err := s.repository().PromptExists(ctx, promptID)
	if err != nil {
		return 0, 0, err
	}
	// Un prompt desconocido es esperable —el cliente reporta al cerrar el turno
	// y puede equivocarse—, no un error del sistema: se informa y no se escribe.
	if !existe {
		return 0, len(candidatos), nil
	}

	n, err := s.repository().RecordUsage(ctx, RecordUsageParams{
		PromptID: promptID, CandidateIDs: candidatos, UsedIDs: usadas,
	})
	if err != nil {
		return 0, 0, err
	}
	return int(n), descartados, nil
}

// normalizarIDs deduplica preservando el orden, descarta uuid.Nil y recorta al
// cap. Devuelve los ids válidos y cuántos quedaron afuera.
func normalizarIDs(in []uuid.UUID) ([]uuid.UUID, int) {
	visto := make(map[uuid.UUID]bool, len(in))
	out := make([]uuid.UUID, 0, len(in))
	descartados := 0
	for _, id := range in {
		if id == uuid.Nil || visto[id] {
			continue
		}
		visto[id] = true
		if len(out) >= maxUsageIDsPerTurn {
			descartados++
			continue
		}
		out = append(out, id)
	}
	return out, descartados
}
