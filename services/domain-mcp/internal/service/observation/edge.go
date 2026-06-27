// Package observation — fase 1 memory graph: aristas tipadas y bi-temporales
// entre knowledge_observations (tabla knowledge_observation_edges, mig 000175).
//
// Una arista es dirigida (source -> target), tiene un tipo semántico
// (supersedes/contradicts/derived_from/depends_on/relates_to) y es bi-temporal:
//   - valid_from/valid_to: valid time (valid_to NULL = vigente en el dominio).
//   - created_at: transaction time (cuándo el sistema la registró).
//
// Aislamiento: single-tenant por project_id. Las queries de grafo se acotan
// por project_id; EdgeService valida que source y target pertenezcan al mismo
// project antes de crear la arista. El project se DERIVA del source resuelto
// por id (GetObservation), nunca se confía en un project provisto por el caller.
package observation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/service/observation/observationdb"
	"nunezlagos/domain/internal/store/txctx"
)

// Errores de dominio del grafo. El handler MCP los mapea a respuestas claras.
var (
	ErrEdgeNotFound     = errors.New("edge not found")
	ErrEdgeExists       = errors.New("active edge already exists (same source, target, type)")
	ErrEdgeSelf         = errors.New("edge source and target must differ")
	ErrEdgeCrossProject = errors.New("edge source and target must belong to the same project")
)

// activeEdgeUniqueIndex es el nombre del índice único parcial de la mig 000175.
// pgx reporta este nombre en pgconn.PgError.ConstraintName cuando se viola.
const activeEdgeUniqueIndex = "knowledge_observation_edges_active_uniq"

// defaultInferThreshold: similitud coseno mínima para crear una arista inferida.
const defaultInferThreshold = 0.85

// defaultInferTopK: cuántos candidatos pedir a pgvector por observation.
const defaultInferTopK = 10

// defaultPathMaxDepth: profundidad máxima por defecto del BFS de Path.
const defaultPathMaxDepth = 6

// Edge es la representación de dominio de una arista del grafo de memoria.
type Edge struct {
	ID         uuid.UUID
	ProjectID  uuid.UUID
	SourceID   uuid.UUID
	TargetID   uuid.UUID
	EdgeType   string
	Origin     string
	Confidence float32
	ValidFrom  time.Time
	ValidTo    *time.Time // nil = vigente
	Note       *string
	Metadata   map[string]any
	CreatedBy  *uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// EdgeService encapsula la lógica de negocio del grafo de memoria. Comparte
// las mismas dependencias que Service (Pool, Embedder, Audit) y honra la
// tx-context vía q(ctx).
type EdgeService struct {
	Pool     *pgxpool.Pool
	Embedder llm.Embedder
	Audit    audit.Recorder
}

// NewEdgeService construye un EdgeService con dependencias explícitas.
func NewEdgeService(pool *pgxpool.Pool, embedder llm.Embedder, audit audit.Recorder) *EdgeService {
	return &EdgeService{Pool: pool, Embedder: embedder, Audit: audit}
}

// q retorna un *observationdb.Queries atado a la tx del context si existe,
// o al pool en su defecto (mismo patrón que pgRepository.q).
func (s *EdgeService) q(ctx context.Context) *observationdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return observationdb.New(tx)
	}
	return observationdb.New(s.Pool)
}

// LinkInput describe la creación de una arista manual.
//
// NOTA: NO incluye ProjectID. El project se DERIVA del source ya validado dentro
// de Link (vía GetObservation por id), nunca se confía en un project provisto
// por el caller.
type LinkInput struct {
	SourceID   uuid.UUID
	TargetID   uuid.UUID
	EdgeType   string
	Confidence float32 // 0..1; si 0 se asume 1.0 (manual)
	Note       *string
	Metadata   map[string]any
	CreatedBy  *uuid.UUID
}

// InferInput describe una corrida de inferencia de aristas relates_to por
// similitud de embedding.
type InferInput struct {
	ProjectID uuid.UUID
	// ObservationID opcional: si está, infiere solo desde esa observation.
	// Si es nil, infiere desde todas las observations vigentes del project.
	ObservationID *uuid.UUID
	// Threshold mínimo de score (coseno). Si <= 0 usa defaultInferThreshold.
	Threshold float64
	// TopK candidatos por observation. Si <= 0 usa defaultInferTopK.
	TopK      int
	CreatedBy *uuid.UUID
}

// Link crea una arista manual source -> target.
//
// Resuelve source y target con GetObservation (por id). El project_id de la
// arista se DERIVA del source ya validado (no se confía en el caller).
//
// Validaciones: source != target y ambos del mismo project. Si edge_type es
// 'supersedes', re-afirmar la MISMA arista A->B vigente es no-op idempotente
// (devuelve ErrEdgeExists como el resto de tipos, sin churn bi-temporal). Una
// violación del índice único activo se traduce a ErrEdgeExists.
func (s *EdgeService) Link(ctx context.Context, in LinkInput) (*Edge, error) {
	if in.SourceID == in.TargetID {
		return nil, ErrEdgeSelf
	}

	projectID, err := s.loadAndValidateEndpoints(ctx, in)
	if err != nil {
		return nil, err
	}

	confidence := in.Confidence
	if confidence <= 0 {
		confidence = 1.0
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}
	metaJSON, _ := json.Marshal(in.Metadata)

	// supersedes: NO hay tratamiento especial. Re-afirmar la MISMA arista A->B
	// supersedes vigente es no-op idempotente: el índice único parcial activo
	// (project_id, source_id, target_id, edge_type) garantiza a lo sumo una
	// activa y devuelve ErrEdgeExists, igual que el resto de edge_types. No se
	// cierra ni reinserta la vigente (evita churn y versiones espurias en el
	// historial bi-temporal).

	row, err := s.q(ctx).InsertEdge(ctx, observationdb.InsertEdgeParams{
		ProjectID:  projectID,
		SourceID:   in.SourceID,
		TargetID:   in.TargetID,
		EdgeType:   in.EdgeType,
		Origin:     "manual",
		Confidence: confidence,
		ValidFrom:  pgtype.Timestamptz{}, // NULL -> COALESCE(NOW())
		Note:       in.Note,
		Metadata:   metaJSON,
		CreatedBy:  in.CreatedBy,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == activeEdgeUniqueIndex {
				return nil, ErrEdgeExists
			}
		}
		return nil, fmt.Errorf("insert edge: %w", err)
	}

	e := edgeFromInsert(row)
	s.recordEdgeLinked(ctx, in, e)
	return &e, nil
}

// loadAndValidateEndpoints carga source y target por id, valida que existan y
// que compartan project, y devuelve el project_id derivado del source validado
// (no se confía en un project del caller).
func (s *EdgeService) loadAndValidateEndpoints(ctx context.Context, in LinkInput) (uuid.UUID, error) {
	src, err := s.q(ctx).GetObservation(ctx, in.SourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("source: %w", ErrNotFound)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get source: %w", err)
	}
	tgt, err := s.q(ctx).GetObservation(ctx, in.TargetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("target: %w", ErrNotFound)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get target: %w", err)
	}
	if src.ProjectID != tgt.ProjectID {
		return uuid.Nil, ErrEdgeCrossProject
	}
	return src.ProjectID, nil
}

// recordEdgeLinked registra el evento de auditoría de una arista creada (no-op
// si no hay Recorder).
func (s *EdgeService) recordEdgeLinked(ctx context.Context, in LinkInput, e Edge) {
	if s.Audit == nil {
		return
	}
	audit.RecordOrLog(ctx, s.Audit, audit.Event{
		ActorID:    in.CreatedBy,
		ActorType:  audit.ActorUser,
		Action:     "observation.edge.linked",
		EntityType: "observation_edge",
		EntityID:   &e.ID,
		NewValues: map[string]any{
			"source_id": e.SourceID,
			"target_id": e.TargetID,
			"edge_type": e.EdgeType,
		},
	})
}

// Unlink hace soft-delete de la arista por id.
//
// Carga la arista (GetEdge) antes de borrar para poblar el audit con
// source/target/edge_type. SoftDeleteEdge devuelve 0 filas si la arista no
// existe o ya fue borrada -> ErrEdgeNotFound (mismo patrón que el repo).
func (s *EdgeService) Unlink(ctx context.Context, edgeID uuid.UUID) error {
	edge, err := s.q(ctx).GetEdge(ctx, edgeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEdgeNotFound
	}
	if err != nil {
		return fmt.Errorf("get edge: %w", err)
	}

	n, err := s.q(ctx).SoftDeleteEdge(ctx, edgeID)
	if err != nil {
		return fmt.Errorf("soft delete edge: %w", err)
	}
	if n == 0 {
		return ErrEdgeNotFound
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorUser,
			Action:     "observation.edge.unlinked",
			EntityType: "observation_edge",
			EntityID:   &edgeID,
			OldValues: map[string]any{
				"source_id": edge.SourceID,
				"target_id": edge.TargetID,
				"edge_type": edge.EdgeType,
			},
		})
	}
	return nil
}

// Neighbors devuelve las aristas vigentes incidentes a observationID.
//
// Resuelve la observation ancla con GetObservation (por id) y DERIVA su
// project_id (no se confía en un project pasado por el caller).
//
// direction: "forward" (salientes source=obs), "backward" (entrantes
// target=obs) o "both" (ambas). edgeType opcional filtra por tipo.
func (s *EdgeService) Neighbors(ctx context.Context, observationID uuid.UUID, direction string, edgeType *string) ([]Edge, error) {
	if direction == "" {
		direction = "both"
	}
	obs, err := s.q(ctx).GetObservation(ctx, observationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolve observation: %w", err)
	}
	projectID := obs.ProjectID
	var out []Edge

	if direction == "forward" || direction == "both" {
		rows, err := s.q(ctx).ListEdgesBySource(ctx, observationdb.ListEdgesBySourceParams{
			ProjectID: projectID,
			SourceID:  observationID,
			EdgeType:  edgeType,
		})
		if err != nil {
			return nil, fmt.Errorf("neighbors forward: %w", err)
		}
		for _, r := range rows {
			out = append(out, edgeFromListSource(r))
		}
	}
	if direction == "backward" || direction == "both" {
		rows, err := s.q(ctx).ListEdgesByTarget(ctx, observationdb.ListEdgesByTargetParams{
			ProjectID: projectID,
			TargetID:  observationID,
			EdgeType:  edgeType,
		})
		if err != nil {
			return nil, fmt.Errorf("neighbors backward: %w", err)
		}
		for _, r := range rows {
			out = append(out, edgeFromListTarget(r))
		}
	}
	return out, nil
}

// Subgraph devuelve todas las aristas vigentes del project (opcionalmente
// filtradas por tipo) junto con el conjunto de nodos (observation IDs) que
// participan en ellas. Útil para visualizar o exportar el grafo.
func (s *EdgeService) Subgraph(ctx context.Context, projectID uuid.UUID, edgeType *string) (nodes []uuid.UUID, edges []Edge, err error) {
	rows, err := s.q(ctx).ListEdgesByProject(ctx, observationdb.ListEdgesByProjectParams{
		ProjectID: projectID,
		EdgeType:  edgeType,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("subgraph: %w", err)
	}
	seen := make(map[uuid.UUID]struct{})
	edges = make([]Edge, 0, len(rows))
	for _, r := range rows {
		e := edgeFromListProject(r)
		edges = append(edges, e)
		if _, ok := seen[e.SourceID]; !ok {
			seen[e.SourceID] = struct{}{}
			nodes = append(nodes, e.SourceID)
		}
		if _, ok := seen[e.TargetID]; !ok {
			seen[e.TargetID] = struct{}{}
			nodes = append(nodes, e.TargetID)
		}
	}
	return nodes, edges, nil
}

// Path busca el camino más corto (en número de aristas) desde fromID hasta
// toID recorriendo aristas vigentes del project en dirección source->target.
//
// Resuelve la observation origen con GetObservation (por id) y DERIVA el
// project_id (no se confía en un project del caller).
//
// Carga el subgrafo del project en memoria, construye la adyacencia y hace BFS.
// Devuelve la cadena de aristas (en orden) o nil si no hay camino dentro de
// maxDepth (default defaultPathMaxDepth).
func (s *EdgeService) Path(ctx context.Context, fromID, toID uuid.UUID, maxDepth int) ([]Edge, error) {
	if maxDepth <= 0 {
		maxDepth = defaultPathMaxDepth
	}
	if fromID == toID {
		return []Edge{}, nil
	}

	from, err := s.q(ctx).GetObservation(ctx, fromID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolve from observation: %w", err)
	}
	projectID := from.ProjectID

	rows, err := s.q(ctx).ListEdgesByProject(ctx, observationdb.ListEdgesByProjectParams{
		ProjectID: projectID,
		EdgeType:  nil,
	})
	if err != nil {
		return nil, fmt.Errorf("path load edges: %w", err)
	}

	// adyacencia source -> aristas salientes.
	adj := make(map[uuid.UUID][]Edge, len(rows))
	for _, r := range rows {
		e := edgeFromListProject(r)
		adj[e.SourceID] = append(adj[e.SourceID], e)
	}

	// BFS guardando la arista predecesora de cada nodo para reconstruir.
	type state struct {
		node  uuid.UUID
		depth int
	}
	visited := map[uuid.UUID]bool{fromID: true}
	prevEdge := map[uuid.UUID]Edge{}
	queue := []state{{node: fromID, depth: 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= maxDepth {
			continue
		}
		for _, e := range adj[cur.node] {
			if visited[e.TargetID] {
				continue
			}
			visited[e.TargetID] = true
			prevEdge[e.TargetID] = e
			if e.TargetID == toID {
				return reconstructPath(prevEdge, fromID, toID), nil
			}
			queue = append(queue, state{node: e.TargetID, depth: cur.depth + 1})
		}
	}
	return nil, nil
}

// reconstructPath reconstruye la cadena de aristas from -> to siguiendo
// prevEdge hacia atrás y luego invirtiendo.
func reconstructPath(prevEdge map[uuid.UUID]Edge, fromID, toID uuid.UUID) []Edge {
	var rev []Edge
	cur := toID
	for cur != fromID {
		e, ok := prevEdge[cur]
		if !ok {
			return nil
		}
		rev = append(rev, e)
		cur = e.SourceID
	}
	// invertir.
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

// InferEdges genera aristas relates_to (origin='inferred') por similitud de
// embedding. Para cada observation fuente (la indicada o todas las del project)
// recomputa su embedding desde el content, pide los TopK candidatos por coseno
// y crea una arista por cada candidato con score >= threshold.
//
// Idempotencia SEGURA en tx: usa InsertEdgeIfAbsent (INSERT ... ON CONFLICT DO
// NOTHING contra el índice único activo). A diferencia de capturar la
// UniqueViolation, ON CONFLICT NO lanza excepción, por lo que NO aborta la
// transacción (que en PostgreSQL haría fallar toda sentencia posterior con
// 25P02 'current transaction is aborted'). Una arista ya existente => RETURNING
// vacío (pgx.ErrNoRows) => no se cuenta como created y el loop sigue.
//
// Cuando se pasa una observation fuente, se resuelve con GetObservation (por id)
// y se valida que pertenezca a in.ProjectID. El barrido por project usa el
// in.ProjectID ya resuelto por el caller (vía GetBySlug).
//
// Devuelve (created, candidates, error): aristas nuevas creadas y candidatos
// totales por encima del threshold evaluados.
func (s *EdgeService) InferEdges(ctx context.Context, in InferInput) (created int, candidates int, err error) {
	threshold := in.Threshold
	if threshold <= 0 {
		threshold = defaultInferThreshold
	}
	topK := in.TopK
	if topK <= 0 {
		topK = defaultInferTopK
	}

	sources, err := s.resolveInferSources(ctx, in)
	if err != nil {
		return 0, 0, err
	}

	for _, src := range sources {
		c, cand, err := s.inferEdgesForSource(ctx, in, src, threshold, topK)
		if err != nil {
			return created, candidates, err
		}
		created += c
		candidates += cand
	}
	return created, candidates, nil
}

// resolveInferSources resuelve las observations fuente de la inferencia: la
// indicada (validando que sea del project) o todas las del project.
func (s *EdgeService) resolveInferSources(ctx context.Context, in InferInput) ([]observationdb.ListObservationsRow, error) {
	if in.ObservationID != nil {
		o, err := s.q(ctx).GetObservation(ctx, *in.ObservationID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("get source observation: %w", err)
		}
		if o.ProjectID != in.ProjectID {
			return nil, ErrEdgeCrossProject
		}
		return []observationdb.ListObservationsRow{{
			ID: o.ID, ProjectID: o.ProjectID, Content: o.Content,
		}}, nil
	}
	rows, err := s.q(ctx).ListObservations(ctx, observationdb.ListObservationsParams{
		ProjectID:   in.ProjectID,
		ResultLimit: 200,
	})
	if err != nil {
		return nil, fmt.Errorf("list source observations: %w", err)
	}
	return rows, nil
}

// inferEdgesForSource embebe el content de src, pide los topK candidatos por
// coseno y crea una arista relates_to por candidato con score >= threshold.
// Devuelve (created, candidates) para esa fuente.
func (s *EdgeService) inferEdgesForSource(ctx context.Context, in InferInput, src observationdb.ListObservationsRow, threshold float64, topK int) (created int, candidates int, err error) {
	vec, err := s.Embedder.Embed(ctx, src.Content)
	if err != nil {
		return 0, 0, fmt.Errorf("embed source %s: %w", src.ID, err)
	}
	if llm.IsZero(vec) {
		return 0, 0, nil // sin embedder real no hay similitud útil
	}
	cands, err := s.q(ctx).FindEdgeCandidatesByEmbedding(ctx, observationdb.FindEdgeCandidatesByEmbeddingParams{
		Embedding:   vectorLiteral(vec),
		ProjectID:   in.ProjectID,
		SourceID:    src.ID,
		ResultLimit: int32(topK),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("find candidates for %s: %w", src.ID, err)
	}
	for _, c := range cands {
		if c.Score < threshold {
			continue
		}
		candidates++
		confidence := float32(c.Score)
		if confidence > 1 {
			confidence = 1
		}
		meta, _ := json.Marshal(map[string]any{"inferred_score": c.Score})
		_, err := s.q(ctx).InsertEdgeIfAbsent(ctx, observationdb.InsertEdgeIfAbsentParams{
			ProjectID:  in.ProjectID,
			SourceID:   src.ID,
			TargetID:   c.ID,
			EdgeType:   "relates_to",
			Origin:     "inferred",
			Confidence: confidence,
			ValidFrom:  pgtype.Timestamptz{},
			Note:       nil,
			Metadata:   meta,
			CreatedBy:  in.CreatedBy,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT DO NOTHING: la arista ya existía (RETURNING vacío).
			// Idempotente y sin abortar la tx. No se cuenta como created.
			continue
		}
		if err != nil {
			return created, candidates, fmt.Errorf("insert inferred edge %s->%s: %w", src.ID, c.ID, err)
		}
		created++
	}
	return created, candidates, nil
}

// mappers row -> Edge

func validToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func metaMap(b []byte) map[string]any {
	if b == nil {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func edgeFromInsert(r observationdb.InsertEdgeRow) Edge {
	return Edge{
		ID: r.ID, ProjectID: r.ProjectID, SourceID: r.SourceID, TargetID: r.TargetID,
		EdgeType: r.EdgeType, Origin: r.Origin, Confidence: r.Confidence,
		ValidFrom: r.ValidFrom, ValidTo: validToPtr(r.ValidTo), Note: r.Note,
		Metadata: metaMap(r.Metadata), CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func edgeFromListSource(r observationdb.ListEdgesBySourceRow) Edge {
	return Edge{
		ID: r.ID, ProjectID: r.ProjectID, SourceID: r.SourceID, TargetID: r.TargetID,
		EdgeType: r.EdgeType, Origin: r.Origin, Confidence: r.Confidence,
		ValidFrom: r.ValidFrom, ValidTo: validToPtr(r.ValidTo), Note: r.Note,
		Metadata: metaMap(r.Metadata), CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func edgeFromListTarget(r observationdb.ListEdgesByTargetRow) Edge {
	return Edge{
		ID: r.ID, ProjectID: r.ProjectID, SourceID: r.SourceID, TargetID: r.TargetID,
		EdgeType: r.EdgeType, Origin: r.Origin, Confidence: r.Confidence,
		ValidFrom: r.ValidFrom, ValidTo: validToPtr(r.ValidTo), Note: r.Note,
		Metadata: metaMap(r.Metadata), CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func edgeFromListProject(r observationdb.ListEdgesByProjectRow) Edge {
	return Edge{
		ID: r.ID, ProjectID: r.ProjectID, SourceID: r.SourceID, TargetID: r.TargetID,
		EdgeType: r.EdgeType, Origin: r.Origin, Confidence: r.Confidence,
		ValidFrom: r.ValidFrom, ValidTo: validToPtr(r.ValidTo), Note: r.Note,
		Metadata: metaMap(r.Metadata), CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
