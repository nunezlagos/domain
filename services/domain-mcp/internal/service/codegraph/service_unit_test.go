// Tests unitarios SIN DB del CodegraphService. Cubren:
//   - funciones puras: reconstructPath (BFS), countByKind, computeGodNodes,
//     isTestFile, languageOf y los helpers de punteros.
//   - validaciones que ocurren ANTES de tocar la DB (guards de project_id /
//     symbol / from==to), invocables con Pool == nil porque el método retorna
//     antes de construir Queries.
//
// Lo que SOLO se puede testear con DB real (resolución de símbolos vía sqlc,
// Build incremental sobre disco+tx, resolveTarget) queda cubierto por los tests
// con build-tag 'integration' (testcontainers + Postgres).
package codegraph

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/codegraph/codegraphdb"
)

// --- reconstructPath: reconstrucción del camino BFS ---

// TestReconstructPath_Chain arma prevEdge para A->B->C y verifica que el camino
// se reconstruye en orden (A->B, B->C).
func TestReconstructPath_Chain(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	eAB := CodeEdge{ID: uuid.New(), SourceNodeID: a, TargetNodeID: b, EdgeType: EdgeCalls}
	eBC := CodeEdge{ID: uuid.New(), SourceNodeID: b, TargetNodeID: c, EdgeType: EdgeCalls}
	prev := map[uuid.UUID]CodeEdge{b: eAB, c: eBC}

	path := reconstructPath(prev, a, c)
	if len(path) != 2 {
		t.Fatalf("len(path) = %d, want 2", len(path))
	}
	if path[0].ID != eAB.ID || path[1].ID != eBC.ID {
		t.Fatalf("orden incorrecto: got [%v %v], want [%v %v]", path[0].ID, path[1].ID, eAB.ID, eBC.ID)
	}
}

// TestReconstructPath_Broken: si la cadena prevEdge no llega a fromID, devuelve nil.
func TestReconstructPath_Broken(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	// Solo hay arista hacia c desde b, pero b no tiene predecesor hacia a.
	prev := map[uuid.UUID]CodeEdge{c: {SourceNodeID: b, TargetNodeID: c}}
	if got := reconstructPath(prev, a, c); got != nil {
		t.Fatalf("got %v, want nil (cadena rota)", got)
	}
}

// --- countByKind: conteo determinista por kind ---

func node(kind string) codegraphdb.ListNodesByProjectRow {
	return codegraphdb.ListNodesByProjectRow{ID: uuid.New(), Kind: kind}
}

func TestCountByKind_OrderedAndCounted(t *testing.T) {
	nodes := []codegraphdb.ListNodesByProjectRow{
		node(KindFunc), node(KindFunc), node(KindType), node(KindMethod), node(KindFunc),
	}
	got := countByKind(nodes)
	// Orden alfabético por kind: func, method, type.
	want := []KindCount{
		{Kind: KindFunc, Count: 3},
		{Kind: KindMethod, Count: 1},
		{Kind: KindType, Count: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCountByKind_Empty(t *testing.T) {
	if got := countByKind(nil); len(got) != 0 {
		t.Fatalf("got %v, want vacío", got)
	}
}

// --- computeGodNodes: grado in/out, orden, top-N, e ignorar aristas a nodos muertos ---

func TestComputeGodNodes_DegreeAndOrder(t *testing.T) {
	hub, leaf, other := uuid.New(), uuid.New(), uuid.New()
	nodeByID := map[uuid.UUID]CodeNode{
		hub:   {ID: hub, QualifiedName: "pkg.Hub"},
		leaf:  {ID: leaf, QualifiedName: "pkg.Leaf"},
		other: {ID: other, QualifiedName: "pkg.Other"},
	}
	edges := []codegraphdb.CodeEdge{
		{SourceNodeID: leaf, TargetNodeID: hub},  // hub +in
		{SourceNodeID: other, TargetNodeID: hub}, // hub +in
		{SourceNodeID: hub, TargetNodeID: leaf},  // hub +out, leaf +in
	}
	gods := computeGodNodes(nodeByID, edges)

	// hub: in=2 out=1 deg=3; leaf: in=1 out=1 deg=2; other: out=1 deg=1.
	if len(gods) != 3 {
		t.Fatalf("len(gods) = %d, want 3 (%+v)", len(gods), gods)
	}
	if gods[0].Node.ID != hub || gods[0].Degree != 3 || gods[0].InDegree != 2 || gods[0].OutDegree != 1 {
		t.Fatalf("god[0] = %+v, want hub deg3 in2 out1", gods[0])
	}
}

// TestComputeGodNodes_IgnoresDeadTargets: una arista a un nodo que no está en
// nodeByID (soft-deleteado) no suma grado.
func TestComputeGodNodes_IgnoresDeadTargets(t *testing.T) {
	live, dead := uuid.New(), uuid.New()
	nodeByID := map[uuid.UUID]CodeNode{live: {ID: live, QualifiedName: "pkg.Live"}}
	edges := []codegraphdb.CodeEdge{
		{SourceNodeID: live, TargetNodeID: dead}, // target muerto: no cuenta el in de dead
		{SourceNodeID: dead, TargetNodeID: live}, // source muerto: SÍ cuenta el in de live
	}
	gods := computeGodNodes(nodeByID, edges)
	// live: out=1 (hacia dead) + in=1 (desde dead) = deg 2. dead no aparece.
	if len(gods) != 1 {
		t.Fatalf("len(gods) = %d, want 1 (dead se ignora)", len(gods))
	}
	if gods[0].Node.ID != live || gods[0].Degree != 2 {
		t.Fatalf("god = %+v, want live deg2", gods[0])
	}
}

// TestComputeGodNodes_SkipsZeroDegree: nodos aislados (deg 0) no se incluyen.
func TestComputeGodNodes_SkipsZeroDegree(t *testing.T) {
	a, iso := uuid.New(), uuid.New()
	nodeByID := map[uuid.UUID]CodeNode{
		a:   {ID: a, QualifiedName: "pkg.A"},
		iso: {ID: iso, QualifiedName: "pkg.Isolated"},
	}
	edges := []codegraphdb.CodeEdge{{SourceNodeID: a, TargetNodeID: a}}
	// self-edge: out a + in a => a deg2; iso deg0 omitido.
	gods := computeGodNodes(nodeByID, edges)
	if len(gods) != 1 || gods[0].Node.ID != a {
		t.Fatalf("gods = %+v, want solo a", gods)
	}
}

// TestComputeGodNodes_TopTruncation: con más de defaultGodNodesTop nodos con
// grado, se trunca al top.
func TestComputeGodNodes_TopTruncation(t *testing.T) {
	nodeByID := map[uuid.UUID]CodeNode{}
	var edges []codegraphdb.CodeEdge
	for i := 0; i < defaultGodNodesTop+5; i++ {
		id := uuid.New()
		nodeByID[id] = CodeNode{ID: id, QualifiedName: uuid.NewString()}
		edges = append(edges, codegraphdb.CodeEdge{SourceNodeID: id, TargetNodeID: id})
	}
	gods := computeGodNodes(nodeByID, edges)
	if len(gods) != defaultGodNodesTop {
		t.Fatalf("len(gods) = %d, want %d (truncado)", len(gods), defaultGodNodesTop)
	}
}

// --- isTestFile: convención de test por lenguaje ---

func TestIsTestFile(t *testing.T) {
	cases := map[string]bool{
		"service_test.go":  true,
		"service.go":       false,
		"foo.test.ts":      true,
		"foo.spec.js":      true,
		"Foo.Test.TSX":     true, // case-insensitive
		"foo.ts":           false,
		"foo.spec.php":     true,
		"component.test.tsx": true,
		"bar.py":           false,
		"bar.spec.py":      true,
	}
	for name, want := range cases {
		if got := isTestFile(name); got != want {
			t.Errorf("isTestFile(%q) = %v, want %v", name, got, want)
		}
	}
}

// --- languageOf: fallback a defaultLanguage ---

func TestLanguageOf(t *testing.T) {
	if got := languageOf(&ParsedFile{Language: "python"}); got != "python" {
		t.Fatalf("got %q, want python", got)
	}
	if got := languageOf(&ParsedFile{}); got != defaultLanguage {
		t.Fatalf("got %q, want %q (fallback)", got, defaultLanguage)
	}
}

// --- helpers de punteros ---

func TestPtrHelpers(t *testing.T) {
	if strPtr("") != nil {
		t.Error("strPtr(\"\") debe ser nil")
	}
	if v := strPtr("x"); v == nil || *v != "x" {
		t.Errorf("strPtr(\"x\") = %v, want *\"x\"", v)
	}
	if int32Ptr(0) != nil {
		t.Error("int32Ptr(0) debe ser nil (línea inválida)")
	}
	if v := int32Ptr(7); v == nil || *v != 7 {
		t.Errorf("int32Ptr(7) = %v, want *7", v)
	}
	if deref(nil) != "" {
		t.Error("deref(nil) debe ser \"\"")
	}
	if int32deref(nil) != 0 {
		t.Error("int32deref(nil) debe ser 0")
	}
}

// --- validaciones pre-DB (Pool == nil; retornan antes de construir Queries) ---

func TestExplore_GuardsBeforeDB(t *testing.T) {
	s := &CodegraphService{Pool: nil}
	if _, err := s.Explore(context.Background(), uuid.Nil, "X"); err == nil {
		t.Fatal("Explore con project_id nil debe fallar antes de la DB")
	}
	if _, err := s.Explore(context.Background(), uuid.New(), ""); err == nil {
		t.Fatal("Explore con symbol vacío debe fallar antes de la DB")
	}
}

func TestOverview_GuardBeforeDB(t *testing.T) {
	s := &CodegraphService{Pool: nil}
	if _, err := s.Overview(context.Background(), uuid.Nil); err == nil {
		t.Fatal("Overview con project_id nil debe fallar antes de la DB")
	}
}

func TestPath_GuardsBeforeDB(t *testing.T) {
	s := &CodegraphService{Pool: nil}
	if _, err := s.Path(context.Background(), uuid.Nil, "a", "b", 0); err == nil {
		t.Fatal("Path con project_id nil debe fallar antes de la DB")
	}
}

func TestLinkObservationToCode_GuardsBeforeDB(t *testing.T) {
	s := &CodegraphService{Pool: nil}
	pid := uuid.New()

	if _, err := s.LinkObservationToCode(context.Background(), LinkInput{ProjectID: uuid.Nil}); err == nil {
		t.Fatal("project_id nil debe fallar antes de la DB")
	}
	if _, err := s.LinkObservationToCode(context.Background(), LinkInput{ProjectID: pid, ObservationID: uuid.Nil}); err == nil {
		t.Fatal("observation_id nil debe fallar antes de la DB")
	}
	// link_type inválido: validado contra el CHECK de la mig 000177 antes de la DB.
	_, err := s.LinkObservationToCode(context.Background(), LinkInput{
		ProjectID:     pid,
		ObservationID: uuid.New(),
		LinkType:      "no_existe",
	})
	if err != ErrLinkBadType {
		t.Fatalf("err = %v, want ErrLinkBadType", err)
	}
}

func TestUnlinkObservationFromCode_GuardsBeforeDB(t *testing.T) {
	s := &CodegraphService{Pool: nil}
	pid := uuid.New()

	if err := s.UnlinkObservationFromCode(context.Background(), UnlinkInput{ProjectID: uuid.Nil}); err == nil {
		t.Fatal("project_id nil debe fallar antes de la DB")
	}
	if err := s.UnlinkObservationFromCode(context.Background(), UnlinkInput{ProjectID: pid, ObservationID: uuid.Nil}); err == nil {
		t.Fatal("observation_id nil debe fallar antes de la DB")
	}
	err := s.UnlinkObservationFromCode(context.Background(), UnlinkInput{
		ProjectID:     pid,
		ObservationID: uuid.New(),
		LinkType:      "bad",
	})
	if err != ErrLinkBadType {
		t.Fatalf("err = %v, want ErrLinkBadType", err)
	}
}

// TestValidLinkTypes documenta el set permitido por la mig 000177.
func TestValidLinkTypes(t *testing.T) {
	for _, lt := range []string{"affects", "decided_in", "references", "implements"} {
		if !validLinkTypes[lt] {
			t.Errorf("link_type %q debería ser válido", lt)
		}
	}
	if validLinkTypes["supersedes"] {
		t.Error("supersedes NO es un link_type de cruce")
	}
}
