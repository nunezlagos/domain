// Package codegraph — fase 2c: CodegraphService, la capa de servicio que
// materializa el grafo de CÓDIGO (tablas code_nodes / code_edges /
// code_index_files, mig 000178) usando el parser PURO de la fase 2b
// (parser.go) y las queries generadas por sqlc (codegraphdb).
//
// Aislamiento single-tenant: TODO se acota por project_id. NADA de
// organization_id ni REFERENCES organizations (regla dura del repo).
//
// Incrementalidad: code_index_files guarda content_hash + git_head por archivo.
// Build saltea archivos cuyo hash no cambió y re-parsea solo los modificados o
// nuevos. Los archivos que desaparecieron del disco se soft-deletean.
//
// Honra la tx-context vía q(ctx) (idéntico al patrón de observation/edge.go):
// si el ctx trae una pgx.Tx inyectada (txctx), las queries corren en ella; si
// no, corren contra el pool. Build abre UNA tx por archivo modificado para que
// el re-parseo de ese archivo sea atómico (limpiar versión previa + reinsertar).
package codegraph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/codegraph/codegraphdb"
	"nunezlagos/domain/internal/store/txctx"
)

// Errores de dominio del grafo de código. El handler MCP los mapea a respuestas
// claras.
var (
	// ErrNodeNotFound: no existe nodo activo que matchee el símbolo pedido.
	ErrNodeNotFound = errors.New("code node not found")
	// ErrSymbolAmbiguous: el símbolo resolvió a más de un nodo y la operación
	// requiere uno solo (Path). El caller debe desambiguar por qualified_name.
	ErrSymbolAmbiguous = errors.New("symbol is ambiguous; use a qualified name")
)

// defaultPathMaxDepth: profundidad máxima por defecto del BFS de Path.
const defaultPathMaxDepth = 8

// defaultSearchLimit: tope de candidatos al resolver un símbolo por nombre.
const defaultSearchLimit = 50

// defaultLanguage: lenguaje por defecto si el parser no fijó uno (no debería
// pasar; los parsers siempre setean ParsedFile.Language).
const defaultLanguage = "go"

// languageOf devuelve el lenguaje del archivo parseado, cayendo a
// defaultLanguage si el parser no lo fijó (defensa; no esperado).
func languageOf(parsed *ParsedFile) string {
	if parsed.Language != "" {
		return parsed.Language
	}
	return defaultLanguage
}

// isTestFile reporta si name es un archivo de test según convención por
// lenguaje: Go (*_test.go) y JS/TS/PHP/Python (*.test.*, *.spec.*).
func isTestFile(name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return true
	}
	lower := strings.ToLower(name)
	ext := extOf(lower)
	stem := strings.TrimSuffix(lower, ext)
	return strings.HasSuffix(stem, ".test") || strings.HasSuffix(stem, ".spec")
}

// excludedDirs: directorios que NUNCA se recorren al construir el grafo.
var excludedDirs = map[string]struct{}{
	"vendor":       {},
	"testdata":     {},
	".git":         {},
	"node_modules": {},
}

// CodegraphService encapsula la lógica de negocio del grafo de código. Solo
// necesita el Pool; las queries se obtienen vía q(ctx) honrando tx-context.
type CodegraphService struct {
	Pool *pgxpool.Pool
}

// NewCodegraphService construye un CodegraphService con dependencias explícitas.
func NewCodegraphService(pool *pgxpool.Pool) *CodegraphService {
	return &CodegraphService{Pool: pool}
}

// q retorna un *codegraphdb.Queries atado a la tx del context si existe, o al
// pool en su defecto (mismo patrón que observation/edge.go q).
func (s *CodegraphService) q(ctx context.Context) *codegraphdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return codegraphdb.New(tx)
	}
	return codegraphdb.New(s.Pool)
}

// withTx ejecuta fn dentro de una tx: reusa la del ctx si existe (no anida) o
// abre una propia sobre el pool con rollback/commit.
func (s *CodegraphService) withTx(ctx context.Context, fn func(context.Context) error) error {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return fn(ctx)
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(txctx.WithTxContext(ctx, tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// BuildInput describe una corrida de construcción/actualización del grafo.
//
// NOTA: el aislamiento es SOLO por project_id. RootPath es la raíz del repo a
// recorrer (filesystem local del binario stdio). GitHead es opcional y se guarda
// como metadato de incrementalidad en code_index_files.
type BuildInput struct {
	ProjectID uuid.UUID
	RootPath  string
	GitHead   string
	// IncludeTests: si true, NO excluye archivos *_test.go. Por defecto false
	// (los tests se excluyen).
	IncludeTests bool
}

// BuildStats resume el resultado de una corrida de Build.
type BuildStats struct {
	FilesScanned  int // archivos .go elegibles encontrados en disco
	FilesParsed   int // archivos nuevos o modificados re-parseados
	FilesSkipped  int // archivos sin cambios (hash igual) salteados
	FilesRemoved  int // archivos indexados que ya no existen en disco
	NodesUpserted int // nodos insertados o actualizados
	EdgesCreated  int // aristas nuevas creadas (resueltas)
}

// Build recorre RootPath, parsea los archivos .go elegibles y materializa el
// grafo de código del project de forma INCREMENTAL.
//
// Selección de archivos: solo .go; excluye vendor/, testdata/, .git/,
// node_modules/, directorios ocultos (prefijo ".") y, salvo IncludeTests,
// archivos *_test.go.
//
// Incrementalidad: por archivo se calcula sha256 y se compara con el
// content_hash guardado en code_index_files. Si coincide -> SKIP. Si es nuevo o
// cambió -> ParseFile.
//
// Build en DOS FASES (decoupling de aristas vs. orden de WalkDir):
//
//	FASE 1 (nodos): por cada archivo cambiado, en UNA tx propia, se UPSERTEAN
//	  sus nodos. El UPSERT reusa el id existente (ON CONFLICT ... DO UPDATE) por
//	  lo que la IDENTIDAD del nodo es ESTABLE entre re-parseos: las aristas
//	  entrantes y los vínculos memoria/código siguen apuntando al mismo node_id.
//	  Los nodos del archivo que ya NO existen se soft-deletean (excepto los ids
//	  retenidos) y se borran (hard) sus aristas entrantes y salientes para no
//	  dejar huérfanos hacia nodos muertos. Las aristas salientes de los nodos
//	  RETENIDOS también se borran porque se re-resuelven en la fase 2.
//
//	FASE 2 (aristas): con TODOS los nodos cambiados ya persistidos, se resuelven
//	  e insertan las aristas pendientes contra el conjunto COMPLETO de nodos del
//	  project. Esto evita que una arista A->B se pierda para siempre cuando A se
//	  parsea antes que B y, en builds siguientes, A se saltea por hash.
//
// Borrados: los archivos presentes en code_index_files pero ausentes del disco
// se eliminan en removeFile: soft-delete de sus nodos + hard-delete de aristas
// entrantes y salientes de esos nodos + DeleteIndexFile.
//
// Las rutas se guardan RELATIVAS a RootPath para que el grafo sea portable
// entre clones del repo (el content_hash es lo que da identidad, no el path
// absoluto de la máquina).
func (s *CodegraphService) Build(ctx context.Context, in BuildInput) (*BuildStats, error) {
	if in.ProjectID == uuid.Nil {
		return nil, fmt.Errorf("build: project_id required")
	}
	root, err := filepath.Abs(in.RootPath)
	if err != nil {
		return nil, fmt.Errorf("build: resolve root path: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("build: stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("build: root path is not a directory: %s", root)
	}

	stats := &BuildStats{}

	// Conjunto de paths (relativos) vistos en disco para el barrido de borrados.
	seen := make(map[string]struct{})

	// pendingEdges acumula las aristas de TODOS los archivos cambiados para
	// resolverlas en la fase 2, cuando ya están todos los nodos persistidos.
	// El source siempre se resolvió a un node_id en la fase 1.
	var pendingEdges []pendingEdge

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			// La raíz nunca se excluye aunque su nombre empiece con "." o matchee.
			if path == root {
				return nil
			}
			if _, excluded := excludedDirs[name]; excluded {
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		// Despacho por extensión vía el registry de lenguajes. Si no hay parser
		// registrado para la extensión, el archivo no es elegible y se ignora.
		lp, ok := parserForPath(name)
		if !ok {
			return nil
		}
		// Tests: Go usa el sufijo *_test.go; el resto, *.test.<ext> o
		// *.spec.<ext> (convención JS/TS/PHP/Python comunes).
		if !in.IncludeTests && isTestFile(name) {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("rel path %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)
		seen[rel] = struct{}{}
		stats.FilesScanned++

		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}

		// Incrementalidad: comparar hash con el índice.
		parsed, err := lp.Parse(rel, src)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}

		existing, err := s.q(ctx).GetIndexFile(ctx, codegraphdb.GetIndexFileParams{
			ProjectID: in.ProjectID,
			FilePath:  rel,
		})
		if err == nil && bytes.Equal(existing.ContentHash, parsed.ContentHash) {
			stats.FilesSkipped++
			return nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("get index file %s: %w", rel, err)
		}

		// FASE 1: upsert de nodos del archivo (identidad estable). Las aristas se
		// acumulan en pendingEdges para resolverse en la fase 2.
		nodes, edges, perr := s.indexFileNodes(ctx, in, rel, parsed)
		if perr != nil {
			return fmt.Errorf("index %s: %w", rel, perr)
		}
		pendingEdges = append(pendingEdges, edges...)
		stats.FilesParsed++
		stats.NodesUpserted += nodes
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("build walk: %w", walkErr)
	}

	// Barrido de borrados: archivos indexados ausentes del disco. Se hace ANTES
	// de la fase 2 para que las aristas no se resuelvan contra nodos que van a
	// desaparecer.
	indexed, err := s.q(ctx).ListIndexFiles(ctx, in.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("build list index files: %w", err)
	}
	for _, idx := range indexed {
		if _, ok := seen[idx.FilePath]; ok {
			continue
		}
		if err := s.removeFile(ctx, in.ProjectID, idx.FilePath); err != nil {
			return nil, fmt.Errorf("remove %s: %w", idx.FilePath, err)
		}
		stats.FilesRemoved++
	}

	// FASE 2: resolver e insertar todas las aristas pendientes contra el conjunto
	// COMPLETO de nodos ya persistido. Independiente del orden de WalkDir.
	created, err := s.resolveAndInsertEdges(ctx, in.ProjectID, pendingEdges)
	if err != nil {
		return nil, fmt.Errorf("build resolve edges: %w", err)
	}
	stats.EdgesCreated = created

	return stats, nil
}

// pendingEdge es una arista cuyo source ya se resolvió a un node_id (fase 1) y
// cuyo target se resolverá por qualified_name en la fase 2.
type pendingEdge struct {
	sourceID uuid.UUID
	targetQN string
	edgeType string
}

// indexFileNodes es la fase 1 del build para un archivo: upsert de sus nodos en
// una tx (identidad estable); devuelve (nodosUpserted, aristasPendientes) para
// resolver en la fase 2
func (s *CodegraphService) indexFileNodes(ctx context.Context, in BuildInput, rel string, parsed *ParsedFile) (int, []pendingEdge, error) {
	var nodesUpserted int
	var pending []pendingEdge

	work := func(ctx context.Context) error {
		qx := s.q(ctx)

		// 1) Upsert de nodos. El UPSERT reusa el id existente (DO UPDATE), por lo
		//    que los ids son ESTABLES entre re-parseos. Mapa QN -> node_id para
		//    resolver el source de las aristas. Un mismo QN puede repetirse en
		//    kinds distintos; para el source basta el primer node_id por QN.
		keepIDs := make([]uuid.UUID, 0, len(parsed.Nodes))
		qnToID := make(map[string]uuid.UUID, len(parsed.Nodes))
		for _, n := range parsed.Nodes {
			meta, _ := json.Marshal(map[string]any{})
			row, err := qx.UpsertNode(ctx, codegraphdb.UpsertNodeParams{
				ProjectID:     in.ProjectID,
				Kind:          n.Kind,
				Name:          strPtr(n.Name),
				QualifiedName: strPtr(n.QualifiedName),
				FilePath:      strPtr(n.FilePath),
				LineStart:     int32Ptr(n.LineStart),
				LineEnd:       int32Ptr(n.LineEnd),
				Signature:     strPtr(n.Signature),
				Doc:           strPtr(n.Doc),
				Language:      languageOf(parsed),
				ContentHash:   parsed.ContentHash,
				Metadata:      meta,
			})
			if err != nil {
				return fmt.Errorf("upsert node %s: %w", n.QualifiedName, err)
			}
			nodesUpserted++
			keepIDs = append(keepIDs, row.ID)
			if _, ok := qnToID[n.QualifiedName]; !ok {
				qnToID[n.QualifiedName] = row.ID
			}
		}

		// 2) Soft-delete de los nodos del archivo que YA NO existen (id no
		//    retenido). RETURNING ids para borrar sus aristas entrantes/salientes
		//    y no dejar huérfanos hacia nodos muertos.
		removed, err := qx.SoftDeleteNodesByFileExcept(ctx, codegraphdb.SoftDeleteNodesByFileExceptParams{
			ProjectID:   in.ProjectID,
			FilePath:    &rel,
			KeepNodeIds: keepIDs,
		})
		if err != nil {
			return fmt.Errorf("soft delete vanished nodes: %w", err)
		}
		if len(removed) > 0 {
			if _, err := qx.DeleteEdgesBySourceNodes(ctx, codegraphdb.DeleteEdgesBySourceNodesParams{
				ProjectID:     in.ProjectID,
				SourceNodeIds: removed,
			}); err != nil {
				return fmt.Errorf("delete removed-node out edges: %w", err)
			}
			if _, err := qx.DeleteEdgesByTargetNodes(ctx, codegraphdb.DeleteEdgesByTargetNodesParams{
				ProjectID:     in.ProjectID,
				TargetNodeIds: removed,
			}); err != nil {
				return fmt.Errorf("delete removed-node in edges: %w", err)
			}
		}

		// 3) Borrar las aristas SALIENTES de los nodos retenidos: se re-resuelven
		//    e insertan en la fase 2 (las entrantes desde otros archivos NO se
		//    tocan: la identidad estable las mantiene válidas).
		if len(keepIDs) > 0 {
			if _, err := qx.DeleteEdgesBySourceNodes(ctx, codegraphdb.DeleteEdgesBySourceNodesParams{
				ProjectID:     in.ProjectID,
				SourceNodeIds: keepIDs,
			}); err != nil {
				return fmt.Errorf("delete kept-node out edges: %w", err)
			}
		}

		// 4) Acumular las aristas para la fase 2. El source se resuelve ya (es un
		//    QN local emitido por el parser); el target se resuelve después.
		for _, e := range parsed.Edges {
			srcID, ok := qnToID[e.SourceQN]
			if !ok {
				continue // source no materializado (no debería ocurrir).
			}
			pending = append(pending, pendingEdge{
				sourceID: srcID,
				targetQN: e.TargetQN,
				edgeType: e.EdgeType,
			})
		}

		// 5) Actualizar el índice (content_hash + git_head + node_count).
		nodeCount := int32(len(parsed.Nodes))
		if _, err := qx.UpsertIndexFile(ctx, codegraphdb.UpsertIndexFileParams{
			ProjectID:   in.ProjectID,
			FilePath:    rel,
			ContentHash: parsed.ContentHash,
			GitHead:     gitHeadPtr(in.GitHead),
			NodeCount:   &nodeCount,
		}); err != nil {
			return fmt.Errorf("upsert index file: %w", err)
		}
		return nil
	}

	// Si ya hay tx en el ctx, reusarla (no anidar). Si no, abrir una propia.
	if tx := txctx.TxFromContext(ctx); tx != nil {
		if err := work(ctx); err != nil {
			return 0, nil, err
		}
		return nodesUpserted, pending, nil
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := work(txctx.WithTxContext(ctx, tx)); err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, fmt.Errorf("commit tx: %w", err)
	}
	return nodesUpserted, pending, nil
}

// resolveAndInsertEdges es la FASE 2 del build: resuelve el target de cada arista
// pendiente contra el conjunto COMPLETO de nodos (ya persistido) e inserta las
// aristas resueltas. Es independiente del orden de WalkDir: una arista A->B se
// crea aunque A se haya parseado antes que B, porque la resolución corre cuando
// AMBOS ya existen. Los targets que no resuelven a ningún nodo activo se OMITEN
// (cross-package/externo: v1 no materializa nodos fantasma). Devuelve la cantidad
// de aristas nuevas creadas.
func (s *CodegraphService) resolveAndInsertEdges(ctx context.Context, projectID uuid.UUID, edges []pendingEdge) (int, error) {
	if len(edges) == 0 {
		return 0, nil
	}
	var created int

	work := func(ctx context.Context) error {
		qx := s.q(ctx)
		// cache QN -> node_id para no reconsultar el mismo target repetido
		resolved := make(map[string]uuid.UUID)
		for _, e := range edges {
			tgtID, found, err := s.resolveTargetCached(ctx, projectID, e.targetQN, resolved)
			if err != nil {
				return err
			}
			if !found {
				continue // target no resuelto: se omite (externo/cross-package).
			}
			if e.sourceID == tgtID {
				continue // la mig 000178 prohíbe source == target.
			}
			meta, _ := json.Marshal(map[string]any{})
			_, err = qx.InsertEdgeIfAbsent(ctx, codegraphdb.InsertEdgeIfAbsentParams{
				ProjectID:    projectID,
				SourceNodeID: e.sourceID,
				TargetNodeID: tgtID,
				EdgeType:     e.edgeType,
				Metadata:     meta,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				continue // ON CONFLICT DO NOTHING: ya existía.
			}
			if err != nil {
				return fmt.Errorf("insert edge ->%s: %w", e.targetQN, err)
			}
			created++
		}
		return nil
	}

	if err := s.withTx(ctx, work); err != nil {
		return 0, err
	}
	return created, nil
}

// resolveTargetCached resuelve un targetQN a node_id usando (y poblando) el
// cache local resolved. Devuelve (id, found, err).
func (s *CodegraphService) resolveTargetCached(ctx context.Context, projectID uuid.UUID, targetQN string, resolved map[string]uuid.UUID) (uuid.UUID, bool, error) {
	if id, ok := resolved[targetQN]; ok {
		return id, true, nil
	}
	id, found, err := s.resolveTarget(ctx, projectID, targetQN)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("resolve target %s: %w", targetQN, err)
	}
	if !found {
		return uuid.Nil, false, nil
	}
	resolved[targetQN] = id
	return id, true, nil
}

// resolveTarget mapea un TargetQN de una arista a un node_id buscando un nodo
// ACTIVO cuyo qualified_name == targetQN entre los kinds que pueden ser destino
// de las aristas que emite el parser (calls/method_of/defined_in). Devuelve
// (id, true, nil) si resolvió, (nil, false, nil) si no hay nodo activo, y
// propaga cualquier error de DB NO-ErrNoRows para que el caller aborte la tx en
// vez de tragarse el error como "arista faltante".
func (s *CodegraphService) resolveTarget(ctx context.Context, projectID uuid.UUID, targetQN string) (uuid.UUID, bool, error) {
	for _, kind := range []string{KindFunc, KindMethod, KindType, KindInterface, KindFile} {
		row, err := s.q(ctx).GetNodeByQualified(ctx, codegraphdb.GetNodeByQualifiedParams{
			ProjectID:     projectID,
			QualifiedName: &targetQN,
			Kind:          kind,
		})
		if err == nil {
			return row.ID, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, err
		}
	}
	return uuid.Nil, false, nil
}

// removeFile elimina un archivo ausente del disco: soft-deletea sus nodos y
// hard-deletea las aristas ENTRANTES y SALIENTES de esos nodos (soft-delete es
// UPDATE y no dispara el ON DELETE CASCADE, así que las aristas quedarían
// colgando hacia nodos muertos). Finalmente lo quita del índice.
func (s *CodegraphService) removeFile(ctx context.Context, projectID uuid.UUID, rel string) error {
	work := func(ctx context.Context) error {
		qx := s.q(ctx)
		// Recuperar los ids de los nodos del archivo ANTES de soft-deletearlos
		// para poder borrar sus aristas.
		nodes, err := qx.ListNodesByFile(ctx, codegraphdb.ListNodesByFileParams{
			ProjectID: projectID,
			FilePath:  &rel,
		})
		if err != nil {
			return fmt.Errorf("list nodes: %w", err)
		}
		if len(nodes) > 0 {
			ids := make([]uuid.UUID, 0, len(nodes))
			for _, n := range nodes {
				ids = append(ids, n.ID)
			}
			if _, err := qx.DeleteEdgesBySourceNodes(ctx, codegraphdb.DeleteEdgesBySourceNodesParams{
				ProjectID:     projectID,
				SourceNodeIds: ids,
			}); err != nil {
				return fmt.Errorf("delete out edges: %w", err)
			}
			if _, err := qx.DeleteEdgesByTargetNodes(ctx, codegraphdb.DeleteEdgesByTargetNodesParams{
				ProjectID:     projectID,
				TargetNodeIds: ids,
			}); err != nil {
				return fmt.Errorf("delete in edges: %w", err)
			}
		}
		if _, err := qx.SoftDeleteNodesByFile(ctx, codegraphdb.SoftDeleteNodesByFileParams{
			ProjectID: projectID,
			FilePath:  &rel,
		}); err != nil {
			return fmt.Errorf("soft delete nodes: %w", err)
		}
		if _, err := qx.DeleteIndexFile(ctx, codegraphdb.DeleteIndexFileParams{
			ProjectID: projectID,
			FilePath:  rel,
		}); err != nil {
			return fmt.Errorf("delete index file: %w", err)
		}
		return nil
	}

	return s.withTx(ctx, work)
}

// CodeNode es la representación de dominio de un nodo de código.
type CodeNode struct {
	ID            uuid.UUID
	ProjectID     uuid.UUID
	Kind          string
	Name          string
	QualifiedName string
	FilePath      string
	LineStart     int
	LineEnd       int
	Signature     string
	Doc           string
	Language      string
}

// CodeEdge es la representación de dominio de una arista de código.
type CodeEdge struct {
	ID           uuid.UUID
	ProjectID    uuid.UUID
	SourceNodeID uuid.UUID
	TargetNodeID uuid.UUID
	EdgeType     string
	Metadata     map[string]any
}

// ExploreResult es el blast-radius de un símbolo: el/los nodo(s) que matchean,
// sus callers (aristas calls entrantes) y sus callees (aristas calls salientes).
type ExploreResult struct {
	// Matches: nodos que resolvieron al símbolo pedido (puede ser >1 si se pasó
	// un nombre simple que matchea varios).
	Matches []CodeNode
	// Callers: aristas calls cuyo target es el símbolo (quién lo llama).
	Callers []CodeEdge
	// Callees: aristas calls cuyo source es el símbolo (a quién llama).
	Callees []CodeEdge
}

// Explore resuelve uno o más nodos por símbolo (qualified_name exacto o nombre
// vía SearchNodesByName) y devuelve su blast-radius: callers (ListEdgesByTarget
// type=calls) y callees (ListEdgesBySource type=calls). Incluye signature/doc/
// file:line en cada Match.
//
// El símbolo se resuelve primero como qualified_name exacto contra los kinds
// invocables (func/method); si no hay match exacto, se cae a búsqueda por
// nombre con ILIKE.
func (s *CodegraphService) Explore(ctx context.Context, projectID uuid.UUID, symbol string) (*ExploreResult, error) {
	if projectID == uuid.Nil {
		return nil, fmt.Errorf("explore: project_id required")
	}
	if symbol == "" {
		return nil, fmt.Errorf("explore: symbol required")
	}

	matches, err := s.resolveNodes(ctx, projectID, symbol)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, ErrNodeNotFound
	}

	callsType := EdgeCalls
	res := &ExploreResult{Matches: matches}
	for _, m := range matches {
		callers, err := s.q(ctx).ListEdgesByTarget(ctx, codegraphdb.ListEdgesByTargetParams{
			ProjectID:    projectID,
			TargetNodeID: m.ID,
			EdgeType:     &callsType,
		})
		if err != nil {
			return nil, fmt.Errorf("explore callers: %w", err)
		}
		for _, e := range callers {
			res.Callers = append(res.Callers, edgeFromDB(e))
		}
		callees, err := s.q(ctx).ListEdgesBySource(ctx, codegraphdb.ListEdgesBySourceParams{
			ProjectID:    projectID,
			SourceNodeID: m.ID,
			EdgeType:     &callsType,
		})
		if err != nil {
			return nil, fmt.Errorf("explore callees: %w", err)
		}
		for _, e := range callees {
			res.Callees = append(res.Callees, edgeFromDB(e))
		}
	}
	return res, nil
}

// resolveNodes resuelve un símbolo a nodos de dominio. Estrategia:
//  1. qualified_name exacto contra kinds invocables/destacados.
//  2. fallback: SearchNodesByName con patrón exacto (sin comodines) sobre
//     name/qualified_name; si tampoco hay, patrón %symbol% (substring).
func (s *CodegraphService) resolveNodes(ctx context.Context, projectID uuid.UUID, symbol string) ([]CodeNode, error) {
	if node, found, err := s.resolveExact(ctx, projectID, symbol); err != nil {
		return nil, err
	} else if found {
		return []CodeNode{node}, nil
	}
	return s.resolveByName(ctx, projectID, symbol)
}

// resolveExact intenta resolver el símbolo por qualified_name exacto sobre los
// kinds destacados. Devuelve (node, found, err).
func (s *CodegraphService) resolveExact(ctx context.Context, projectID uuid.UUID, symbol string) (CodeNode, bool, error) {
	for _, kind := range []string{KindFunc, KindMethod, KindType, KindInterface, KindConst, KindVar, KindFile} {
		row, err := s.q(ctx).GetNodeByQualified(ctx, codegraphdb.GetNodeByQualifiedParams{
			ProjectID:     projectID,
			QualifiedName: &symbol,
			Kind:          kind,
		})
		if err == nil {
			return nodeFromGet(row), true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return CodeNode{}, false, fmt.Errorf("resolve qualified: %w", err)
		}
	}
	return CodeNode{}, false, nil
}

// resolveByName busca el símbolo por nombre: primero patrón exacto y, si no hay
// match, substring (%symbol%) sobre name/qualified_name.
func (s *CodegraphService) resolveByName(ctx context.Context, projectID uuid.UUID, symbol string) ([]CodeNode, error) {
	exact := symbol
	rows, err := s.q(ctx).SearchNodesByName(ctx, codegraphdb.SearchNodesByNameParams{
		ProjectID:   projectID,
		Pattern:     &exact,
		ResultLimit: defaultSearchLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("search by name: %w", err)
	}
	if len(rows) == 0 {
		like := "%" + symbol + "%"
		rows, err = s.q(ctx).SearchNodesByName(ctx, codegraphdb.SearchNodesByNameParams{
			ProjectID:   projectID,
			Pattern:     &like,
			ResultLimit: defaultSearchLimit,
		})
		if err != nil {
			return nil, fmt.Errorf("search by name (substring): %w", err)
		}
	}
	out := make([]CodeNode, 0, len(rows))
	for _, r := range rows {
		out = append(out, nodeFromSearch(r))
	}
	return out, nil
}

// Path busca el camino más corto (en número de aristas) desde fromSymbol hasta
// toSymbol recorriendo aristas del project en dirección source->target.
//
// Resuelve ambos símbolos a un único nodo (qualified_name exacto o nombre); si
// alguno resuelve a >1 nodo devuelve ErrSymbolAmbiguous (el caller debe usar un
// qualified_name). Carga el grafo del project en memoria (aristas entre nodos
// ACTIVOS vía ListEdgesByProjectActive, para no atravesar nodos muertos),
// construye la adyacencia y hace BFS. maxDepth default defaultPathMaxDepth.
//
// Devuelve la cadena de aristas (en orden) o nil si no hay camino dentro de
// maxDepth. Mismo patrón que observation/edge.go Path.
func (s *CodegraphService) Path(ctx context.Context, projectID uuid.UUID, fromSymbol, toSymbol string, maxDepth int) ([]CodeEdge, error) {
	if projectID == uuid.Nil {
		return nil, fmt.Errorf("path: project_id required")
	}
	if maxDepth <= 0 {
		maxDepth = defaultPathMaxDepth
	}

	fromID, err := s.resolveSingle(ctx, projectID, fromSymbol)
	if err != nil {
		return nil, fmt.Errorf("path from: %w", err)
	}
	toID, err := s.resolveSingle(ctx, projectID, toSymbol)
	if err != nil {
		return nil, fmt.Errorf("path to: %w", err)
	}
	if fromID == toID {
		return []CodeEdge{}, nil
	}

	// Solo aristas entre nodos ACTIVOS: el BFS no debe atravesar nodos muertos
	// a través de aristas huérfanas que pudieran sobrevivir.
	rows, err := s.q(ctx).ListEdgesByProjectActive(ctx, codegraphdb.ListEdgesByProjectActiveParams{
		ProjectID: projectID,
		EdgeType:  nil,
	})
	if err != nil {
		return nil, fmt.Errorf("path load edges: %w", err)
	}

	// adyacencia source -> aristas salientes.
	adj := make(map[uuid.UUID][]CodeEdge, len(rows))
	for _, r := range rows {
		e := edgeFromDB(r)
		adj[e.SourceNodeID] = append(adj[e.SourceNodeID], e)
	}

	// BFS guardando la arista predecesora de cada nodo para reconstruir.
	type state struct {
		node  uuid.UUID
		depth int
	}
	visited := map[uuid.UUID]bool{fromID: true}
	prevEdge := map[uuid.UUID]CodeEdge{}
	queue := []state{{node: fromID, depth: 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= maxDepth {
			continue
		}
		for _, e := range adj[cur.node] {
			if visited[e.TargetNodeID] {
				continue
			}
			visited[e.TargetNodeID] = true
			prevEdge[e.TargetNodeID] = e
			if e.TargetNodeID == toID {
				return reconstructPath(prevEdge, fromID, toID), nil
			}
			queue = append(queue, state{node: e.TargetNodeID, depth: cur.depth + 1})
		}
	}
	return nil, nil
}

// resolveSingle resuelve un símbolo a EXACTAMENTE un node_id. Error tipado si no
// existe (ErrNodeNotFound) o si es ambiguo (ErrSymbolAmbiguous).
func (s *CodegraphService) resolveSingle(ctx context.Context, projectID uuid.UUID, symbol string) (uuid.UUID, error) {
	nodes, err := s.resolveNodes(ctx, projectID, symbol)
	if err != nil {
		return uuid.Nil, err
	}
	switch len(nodes) {
	case 0:
		return uuid.Nil, ErrNodeNotFound
	case 1:
		return nodes[0].ID, nil
	default:
		return uuid.Nil, ErrSymbolAmbiguous
	}
}

// reconstructPath reconstruye la cadena de aristas from -> to siguiendo prevEdge
// hacia atrás y luego invirtiendo.
func reconstructPath(prevEdge map[uuid.UUID]CodeEdge, fromID, toID uuid.UUID) []CodeEdge {
	var rev []CodeEdge
	cur := toID
	for cur != fromID {
		e, ok := prevEdge[cur]
		if !ok {
			return nil
		}
		rev = append(rev, e)
		cur = e.SourceNodeID
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

// KindCount es el conteo de nodos activos de un kind dado.
type KindCount struct {
	Kind  string
	Count int
}

// GodNode es un nodo con alto grado total (entrantes + salientes), candidato a
// hotspot/acoplamiento alto.
type GodNode struct {
	Node      CodeNode
	InDegree  int
	OutDegree int
	Degree    int // in + out
}

// Overview resume el grafo del project: conteos por kind y los nodos con mayor
// grado total (god-nodes).
type Overview struct {
	TotalNodes int
	TotalEdges int
	ByKind     []KindCount
	GodNodes   []GodNode
}

// defaultGodNodesTop: cuántos god-nodes devolver en Overview.
const defaultGodNodesTop = 10

// Overview computa conteos por kind y top god-nodes por grado de aristas
// (entrantes + salientes). Carga nodos y aristas del project en memoria.
func (s *CodegraphService) Overview(ctx context.Context, projectID uuid.UUID) (*Overview, error) {
	if projectID == uuid.Nil {
		return nil, fmt.Errorf("overview: project_id required")
	}

	nodes, err := s.q(ctx).ListNodesByProject(ctx, codegraphdb.ListNodesByProjectParams{
		ProjectID: projectID,
		Kind:      nil,
	})
	if err != nil {
		return nil, fmt.Errorf("overview list nodes: %w", err)
	}
	edges, err := s.q(ctx).ListEdgesByProject(ctx, codegraphdb.ListEdgesByProjectParams{
		ProjectID: projectID,
		EdgeType:  nil,
	})
	if err != nil {
		return nil, fmt.Errorf("overview list edges: %w", err)
	}

	ov := &Overview{TotalNodes: len(nodes), TotalEdges: len(edges)}

	nodeByID := make(map[uuid.UUID]CodeNode, len(nodes))
	for _, n := range nodes {
		nodeByID[n.ID] = nodeFromList(n)
	}
	ov.ByKind = countByKind(nodes)
	ov.GodNodes = computeGodNodes(nodeByID, edges)
	return ov, nil
}

// countByKind cuenta los nodos por kind y los devuelve ordenados por kind asc
// (determinista).
func countByKind(nodes []codegraphdb.ListNodesByProjectRow) []KindCount {
	kindCounts := map[string]int{}
	for _, n := range nodes {
		kindCounts[n.Kind]++
	}
	kinds := make([]string, 0, len(kindCounts))
	for k := range kindCounts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	out := make([]KindCount, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, KindCount{Kind: k, Count: kindCounts[k]})
	}
	return out
}

// computeGodNodes calcula el grado in/out de cada nodo activo (aristas a nodos
// borrados se ignoran), ordena por grado desc + qualified_name asc y trunca al
// top.
func computeGodNodes(nodeByID map[uuid.UUID]CodeNode, edges []codegraphdb.CodeEdge) []GodNode {
	inDeg := map[uuid.UUID]int{}
	outDeg := map[uuid.UUID]int{}
	for _, e := range edges {
		if _, ok := nodeByID[e.SourceNodeID]; ok {
			outDeg[e.SourceNodeID]++
		}
		if _, ok := nodeByID[e.TargetNodeID]; ok {
			inDeg[e.TargetNodeID]++
		}
	}

	gods := make([]GodNode, 0, len(nodeByID))
	for id, n := range nodeByID {
		in := inDeg[id]
		out := outDeg[id]
		deg := in + out
		if deg == 0 {
			continue
		}
		gods = append(gods, GodNode{Node: n, InDegree: in, OutDegree: out, Degree: deg})
	}
	sort.Slice(gods, func(i, j int) bool {
		if gods[i].Degree != gods[j].Degree {
			return gods[i].Degree > gods[j].Degree
		}
		return gods[i].Node.QualifiedName < gods[j].Node.QualifiedName
	})
	if len(gods) > defaultGodNodesTop {
		gods = gods[:defaultGodNodesTop]
	}
	return gods
}

// mappers + helpers

func edgeFromDB(r codegraphdb.CodeEdge) CodeEdge {
	return CodeEdge{
		ID:           r.ID,
		ProjectID:    r.ProjectID,
		SourceNodeID: r.SourceNodeID,
		TargetNodeID: r.TargetNodeID,
		EdgeType:     r.EdgeType,
		Metadata:     metaMap(r.Metadata),
	}
}

func nodeFromGet(r codegraphdb.GetNodeByQualifiedRow) CodeNode {
	return CodeNode{
		ID: r.ID, ProjectID: r.ProjectID, Kind: r.Kind,
		Name: deref(r.Name), QualifiedName: deref(r.QualifiedName),
		FilePath: deref(r.FilePath), LineStart: int32deref(r.LineStart),
		LineEnd: int32deref(r.LineEnd), Signature: deref(r.Signature),
		Doc: deref(r.Doc), Language: r.Language,
	}
}

func nodeFromSearch(r codegraphdb.SearchNodesByNameRow) CodeNode {
	return CodeNode{
		ID: r.ID, ProjectID: r.ProjectID, Kind: r.Kind,
		Name: deref(r.Name), QualifiedName: deref(r.QualifiedName),
		FilePath: deref(r.FilePath), LineStart: int32deref(r.LineStart),
		LineEnd: int32deref(r.LineEnd), Signature: deref(r.Signature),
		Doc: deref(r.Doc), Language: r.Language,
	}
}

func nodeFromList(r codegraphdb.ListNodesByProjectRow) CodeNode {
	return CodeNode{
		ID: r.ID, ProjectID: r.ProjectID, Kind: r.Kind,
		Name: deref(r.Name), QualifiedName: deref(r.QualifiedName),
		FilePath: deref(r.FilePath), LineStart: int32deref(r.LineStart),
		LineEnd: int32deref(r.LineEnd), Signature: deref(r.Signature),
		Doc: deref(r.Doc), Language: r.Language,
	}
}

func metaMap(b []byte) map[string]any {
	if b == nil {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// strPtr devuelve *string para un valor; "" -> nil (NULL en DB).
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// gitHeadPtr devuelve *string para el git head; "" -> nil.
func gitHeadPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// int32Ptr devuelve *int32 para una línea; 0 -> nil (línea inválida).
func int32Ptr(i int) *int32 {
	if i == 0 {
		return nil
	}
	v := int32(i)
	return &v
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func int32deref(i *int32) int {
	if i == nil {
		return 0
	}
	return int(*i)
}
