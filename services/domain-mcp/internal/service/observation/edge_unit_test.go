// Tests unitarios SIN DB del EdgeService (memory graph). Cubren:
//   - funciones puras: reconstructPath (reconstrucción del BFS de Path),
//     validToPtr y metaMap (mappers row -> Edge).
//   - validaciones que ocurren ANTES de tocar la DB: Link rechaza source==target
//     (ErrEdgeSelf) y Path con from==to devuelve camino vacío; ambas retornan
//     antes de construir Queries, por lo que son invocables con Pool == nil.
//
// La lógica que SOLO se puede testear con DB real (InsertEdge / UniqueViolation
// -> ErrEdgeExists, InsertEdgeIfAbsent -> ErrNoRows = ya existía, BFS completo
// sobre aristas del project, supersedes idempotente vía índice único parcial)
// queda cubierta por los tests con build-tag 'integration' (testcontainers +
// Postgres): el EdgeService usa *pgxpool.Pool concreto (no una interfaz), así que
// no es mockeable a nivel de servicio sin una conexión real.
package observation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// --- reconstructPath: reconstrucción del camino BFS del grafo de memoria ---

// TestReconstructPath_Chain arma prevEdge para A->B->C y verifica orden.
func TestReconstructPath_Chain(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	eAB := Edge{ID: uuid.New(), SourceID: a, TargetID: b, EdgeType: "relates_to"}
	eBC := Edge{ID: uuid.New(), SourceID: b, TargetID: c, EdgeType: "derived_from"}
	prev := map[uuid.UUID]Edge{b: eAB, c: eBC}

	path := reconstructPath(prev, a, c)
	if len(path) != 2 {
		t.Fatalf("len(path) = %d, want 2", len(path))
	}
	if path[0].ID != eAB.ID || path[1].ID != eBC.ID {
		t.Fatalf("orden incorrecto: got [%v %v], want [%v %v]", path[0].ID, path[1].ID, eAB.ID, eBC.ID)
	}
}

// TestReconstructPath_Single: un solo salto A->B.
func TestReconstructPath_Single(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	eAB := Edge{ID: uuid.New(), SourceID: a, TargetID: b}
	path := reconstructPath(map[uuid.UUID]Edge{b: eAB}, a, b)
	if len(path) != 1 || path[0].ID != eAB.ID {
		t.Fatalf("path = %+v, want [%v]", path, eAB.ID)
	}
}

// TestReconstructPath_Broken: cadena que no llega a fromID devuelve nil.
func TestReconstructPath_Broken(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	prev := map[uuid.UUID]Edge{c: {SourceID: b, TargetID: c}} // b sin predecesor
	if got := reconstructPath(prev, a, c); got != nil {
		t.Fatalf("got %v, want nil (cadena rota)", got)
	}
}

// --- mappers row -> Edge ---

func TestValidToPtr(t *testing.T) {
	if validToPtr(pgtype.Timestamptz{Valid: false}) != nil {
		t.Error("valid_to inválido (NULL) debe mapear a nil (vigente)")
	}
	now := time.Now()
	got := validToPtr(pgtype.Timestamptz{Time: now, Valid: true})
	if got == nil || !got.Equal(now) {
		t.Errorf("validToPtr = %v, want %v", got, now)
	}
}

func TestMetaMap(t *testing.T) {
	if metaMap(nil) != nil {
		t.Error("metaMap(nil) debe ser nil")
	}
	m := metaMap([]byte(`{"inferred_score":0.9}`))
	if m == nil || m["inferred_score"].(float64) != 0.9 {
		t.Errorf("metaMap = %v, want inferred_score 0.9", m)
	}
	// JSON inválido: Unmarshal falla y deja el mapa nil sin panic.
	if got := metaMap([]byte("not json")); got != nil {
		t.Errorf("metaMap(invalid) = %v, want nil", got)
	}
}

// --- validaciones pre-DB (Pool == nil; retornan antes de construir Queries) ---

// TestLink_SelfEdge: source == target devuelve ErrEdgeSelf sin tocar la DB.
func TestLink_SelfEdge(t *testing.T) {
	s := &EdgeService{Pool: nil}
	id := uuid.New()
	_, err := s.Link(context.Background(), LinkInput{SourceID: id, TargetID: id, EdgeType: "relates_to"})
	if err != ErrEdgeSelf {
		t.Fatalf("err = %v, want ErrEdgeSelf", err)
	}
}

// TestPath_SameNode: from == to devuelve camino vacío (sin error, sin DB).
func TestPath_SameNode(t *testing.T) {
	s := &EdgeService{Pool: nil}
	id := uuid.New()
	path, err := s.Path(context.Background(), id, id, 0)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if path == nil || len(path) != 0 {
		t.Fatalf("path = %v, want [] (mismo nodo)", path)
	}
}

// TestEdgeDomainErrors documenta que los errores de dominio del grafo son
// distintos entre sí (el handler MCP los mapea a respuestas distintas).
func TestEdgeDomainErrors(t *testing.T) {
	errs := []error{ErrEdgeNotFound, ErrEdgeExists, ErrEdgeSelf, ErrEdgeCrossProject}
	for i := range errs {
		for j := i + 1; j < len(errs); j++ {
			if errs[i] == errs[j] {
				t.Errorf("errores de dominio duplicados en %d y %d", i, j)
			}
		}
	}
}
