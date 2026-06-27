// Package codegraph — fase 2: grafo de CÓDIGO del repo (Go-only v1). Las tablas
// code_nodes / code_edges / code_index_files (mig 000176) materializan el grafo;
// el aislamiento es single-tenant por project_id.
//
// Este archivo (parser.go) es la capa de PARSING PURA: dado el path y los bytes
// de UN archivo .go, produce un *ParsedFile con nodos (file/func/method/type/
// interface/const/var) y aristas intra-archivo (method_of/defined_in/calls/
// imports). No toca la DB ni la red — la persistencia y la incrementalidad por
// content_hash viven en la capa de servicio. Mantener este parser DETERMINISTA
// y testeable en aislamiento.
package codegraph

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// Kinds de nodo soportados. Replican el CHECK de code_nodes.kind (mig 000176).
const (
	KindFile      = "file"
	KindPackage   = "package"
	KindFunc      = "func"
	KindMethod    = "method"
	KindType      = "type"
	KindInterface = "interface"
	KindConst     = "const"
	KindVar       = "var"
)

// Tipos de arista soportados. Replican el CHECK de code_edges.edge_type.
const (
	EdgeCalls      = "calls"
	EdgeImports    = "imports"
	EdgeImplements = "implements"
	EdgeReferences = "references"
	EdgeDefinedIn  = "defined_in"
	EdgeMethodOf   = "method_of"
)

// ParsedNode es un nodo de código extraído del AST. QualifiedName (QN) sigue la
// convención: para símbolos top-level "paquete.Nombre"; para métodos
// "paquete.Tipo.Metodo"; para el nodo file el QN es el file_path.
type ParsedNode struct {
	Kind          string
	Name          string
	QualifiedName string
	FilePath      string
	LineStart     int
	LineEnd       int
	Signature     string
	Doc           string
}

// ParsedEdge es una arista intra-archivo entre dos QN. Para calls no resueltos
// dentro del paquete/archivo, TargetQN queda con el nombre crudo del callee.
type ParsedEdge struct {
	SourceQN string
	TargetQN string
	EdgeType string
}

// ParsedFile es el resultado de parsear un archivo .go.
type ParsedFile struct {
	FilePath    string
	ContentHash []byte
	Nodes       []ParsedNode
	Edges       []ParsedEdge
	ImportPaths []string
}

// PackageName devuelve el nombre de paquete declarado en src, parseando solo el
// package clause (modo rápido, sin cuerpos ni comentarios).
func PackageName(src []byte) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.PackageClauseOnly)
	if err != nil {
		return "", fmt.Errorf("parse package clause: %w", err)
	}
	if f.Name == nil {
		return "", fmt.Errorf("no package clause found")
	}
	return f.Name.Name, nil
}

// ParseFile parsea UN archivo .go y devuelve su grafo intra-archivo.
//
// Nodos extraídos:
//   - file: el archivo en sí (QN = filePath).
//   - func: funciones top-level sin receiver (QN = pkg.Nombre).
//   - method: funciones con receiver (QN = pkg.Tipo.Metodo).
//   - type: type specs que NO son interface (QN = pkg.Nombre).
//   - interface: type specs cuyo underlying es *ast.InterfaceType.
//   - const / var: identificadores de GenDecl top-level (QN = pkg.Nombre).
//
// Aristas extraídas:
//   - defined_in: cada símbolo top-level (y cada método) -> file.
//   - method_of: method -> su type receiver (si el type está en el mismo archivo).
//   - imports: file -> cada import path.
//   - calls: func/method -> callee. Se resuelve best-effort dentro del mismo
//     paquete/archivo (a un QN existente); si no resuelve, TargetQN guarda el
//     nombre crudo del callee.
func ParseFile(filePath string, src []byte) (*ParsedFile, error) {
	sum := sha256.Sum256(src)

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}

	pkg := ""
	if astFile.Name != nil {
		pkg = astFile.Name.Name
	}

	pf := &ParsedFile{
		FilePath:    filePath,
		ContentHash: sum[:],
	}

	lineOf := func(p token.Pos) int {
		if !p.IsValid() {
			return 0
		}
		return fset.Position(p).Line
	}

	fileNode := collectFileNode(astFile, filePath, lineOf)
	pf.Nodes = append(pf.Nodes, fileNode)

	// set de QN locales para resolución best-effort de calls
	localQNs := map[string]string{}
	collectTopLevelDecls(pf, astFile, pkg, filePath, lineOf, localQNs)
	collectDefinedInEdges(pf, fileNode.QualifiedName)
	collectImportEdges(pf, astFile, fileNode.QualifiedName)
	collectCallEdges(pf, astFile, pkg, localQNs)

	return pf, nil
}

// collectFileNode arma el nodo file. QN = filePath para que defined_in/imports
// tengan un source estable.
func collectFileNode(astFile *ast.File, filePath string, lineOf func(token.Pos) int) ParsedNode {
	return ParsedNode{
		Kind:          KindFile,
		Name:          baseName(filePath),
		QualifiedName: filePath,
		FilePath:      filePath,
		LineStart:     1,
		LineEnd:       lineOf(astFile.End()),
		Doc:           docText(astFile.Doc),
	}
}

// collectTopLevelDecls es la pasada 1: emite nodos func/method/type/const/var y
// las aristas method_of, poblando localQNs para la resolución de calls.
func collectTopLevelDecls(
	pf *ParsedFile,
	astFile *ast.File,
	pkg, filePath string,
	lineOf func(token.Pos) int,
	localQNs map[string]string,
) {
	addNode := func(n ParsedNode) {
		pf.Nodes = append(pf.Nodes, n)
	}
	for _, decl := range astFile.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			collectFuncDecl(pf, d, pkg, filePath, lineOf, addNode, localQNs)
		case *ast.GenDecl:
			collectGenDecl(d, pkg, filePath, lineOf, addNode, localQNs)
		}
	}
}

// collectFuncDecl emite el nodo func o method y, para métodos, la arista
// method_of hacia su type receiver.
func collectFuncDecl(
	pf *ParsedFile,
	d *ast.FuncDecl,
	pkg, filePath string,
	lineOf func(token.Pos) int,
	addNode func(ParsedNode),
	localQNs map[string]string,
) {
	name := d.Name.Name
	doc := docText(d.Doc)
	sig := renderFuncSig(d)
	start, end := lineOf(d.Pos()), lineOf(d.End())

	if d.Recv != nil && len(d.Recv.List) > 0 {
		recv := receiverTypeName(d.Recv.List[0].Type)
		qn := joinQN(pkg, recv, name)
		addNode(ParsedNode{
			Kind:          KindMethod,
			Name:          name,
			QualifiedName: qn,
			FilePath:      filePath,
			LineStart:     start,
			LineEnd:       end,
			Signature:     sig,
			Doc:           doc,
		})
		localQNs[name] = qn
		if recv != "" {
			pf.Edges = append(pf.Edges, ParsedEdge{
				SourceQN: qn,
				TargetQN: joinQN(pkg, recv),
				EdgeType: EdgeMethodOf,
			})
		}
		return
	}

	qn := joinQN(pkg, name)
	addNode(ParsedNode{
		Kind:          KindFunc,
		Name:          name,
		QualifiedName: qn,
		FilePath:      filePath,
		LineStart:     start,
		LineEnd:       end,
		Signature:     sig,
		Doc:           doc,
	})
	localQNs[name] = qn
}

// collectDefinedInEdges agrega una arista defined_in de cada símbolo no-file
// hacia el nodo file.
func collectDefinedInEdges(pf *ParsedFile, fileQN string) {
	for _, n := range pf.Nodes {
		if n.Kind == KindFile {
			continue
		}
		pf.Edges = append(pf.Edges, ParsedEdge{
			SourceQN: n.QualifiedName,
			TargetQN: fileQN,
			EdgeType: EdgeDefinedIn,
		})
	}
}

// collectImportEdges de-duplica y ordena los import paths y agrega una arista
// imports file -> path por cada uno.
func collectImportEdges(pf *ParsedFile, astFile *ast.File, fileQN string) {
	importSet := map[string]struct{}{}
	for _, imp := range astFile.Imports {
		if imp.Path == nil {
			continue
		}
		path := strings.Trim(imp.Path.Value, `"`)
		if path == "" {
			continue
		}
		importSet[path] = struct{}{}
	}
	for path := range importSet {
		pf.ImportPaths = append(pf.ImportPaths, path)
	}
	sort.Strings(pf.ImportPaths)
	for _, path := range pf.ImportPaths {
		pf.Edges = append(pf.Edges, ParsedEdge{
			SourceQN: fileQN,
			TargetQN: path,
			EdgeType: EdgeImports,
		})
	}
}

// collectCallEdges es la pasada 2: recorre el cuerpo de cada func/method y
// agrega una arista calls por cada callee resuelto.
func collectCallEdges(pf *ParsedFile, astFile *ast.File, pkg string, localQNs map[string]string) {
	for _, decl := range astFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		var srcQN string
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			srcQN = joinQN(pkg, receiverTypeName(fn.Recv.List[0].Type), fn.Name.Name)
		} else {
			srcQN = joinQN(pkg, fn.Name.Name)
		}
		callees := collectCalls(fn.Body, localQNs)
		for _, target := range callees {
			pf.Edges = append(pf.Edges, ParsedEdge{
				SourceQN: srcQN,
				TargetQN: target,
				EdgeType: EdgeCalls,
			})
		}
	}
}

// collectGenDecl extrae nodos type/interface/const/var de un *ast.GenDecl
// top-level. El doc de cada spec cae al doc del decl si el spec no tiene propio.
func collectGenDecl(
	d *ast.GenDecl,
	pkg, filePath string,
	lineOf func(token.Pos) int,
	addNode func(ParsedNode),
	localQNs map[string]string,
) {
	switch d.Tok {
	case token.TYPE:
		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			collectTypeSpec(ts, d, pkg, filePath, lineOf, addNode, localQNs)
		}
	case token.CONST, token.VAR:
		kind := KindConst
		if d.Tok == token.VAR {
			kind = KindVar
		}
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			collectValueSpec(vs, d, kind, pkg, filePath, lineOf, addNode, localQNs)
		}
	}
}

// collectTypeSpec emite un nodo type/interface a partir de un TypeSpec.
func collectTypeSpec(
	ts *ast.TypeSpec,
	d *ast.GenDecl,
	pkg, filePath string,
	lineOf func(token.Pos) int,
	addNode func(ParsedNode),
	localQNs map[string]string,
) {
	kind := KindType
	if _, isIface := ts.Type.(*ast.InterfaceType); isIface {
		kind = KindInterface
	}
	doc := docText(ts.Doc)
	if doc == "" {
		doc = docText(d.Doc)
	}
	qn := joinQN(pkg, ts.Name.Name)
	addNode(ParsedNode{
		Kind:          kind,
		Name:          ts.Name.Name,
		QualifiedName: qn,
		FilePath:      filePath,
		LineStart:     lineOf(ts.Pos()),
		LineEnd:       lineOf(ts.End()),
		Doc:           doc,
	})
	localQNs[ts.Name.Name] = qn
}

// collectValueSpec emite un nodo const/var por cada nombre de un ValueSpec
// (omite el blank identifier).
func collectValueSpec(
	vs *ast.ValueSpec,
	d *ast.GenDecl,
	kind, pkg, filePath string,
	lineOf func(token.Pos) int,
	addNode func(ParsedNode),
	localQNs map[string]string,
) {
	doc := docText(vs.Doc)
	if doc == "" {
		doc = docText(d.Doc)
	}
	for _, name := range vs.Names {
		if name.Name == "_" {
			continue
		}
		qn := joinQN(pkg, name.Name)
		addNode(ParsedNode{
			Kind:          kind,
			Name:          name.Name,
			QualifiedName: qn,
			FilePath:      filePath,
			LineStart:     lineOf(name.Pos()),
			LineEnd:       lineOf(name.End()),
			Doc:           doc,
		})
		localQNs[name.Name] = qn
	}
}

// collectCalls recorre un cuerpo de función y devuelve los QN de los callees,
// de-duplicados y ordenados (determinismo). Resolución best-effort:
//   - foo(...)      -> si foo es local, su QN; si no, "foo" crudo.
//   - x.Method(...) -> si Method es local, su QN; si no, "x.Method" crudo.
func collectCalls(body *ast.BlockStmt, localQNs map[string]string) []string {
	seen := map[string]struct{}{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if qn, found := localQNs[fun.Name]; found {
				seen[qn] = struct{}{}
			} else {
				seen[fun.Name] = struct{}{}
			}
		case *ast.SelectorExpr:
			method := fun.Sel.Name
			if qn, found := localQNs[method]; found {
				seen[qn] = struct{}{}
			} else {
				seen[exprName(fun.X)+"."+method] = struct{}{}
			}
		}
		return true
	})
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// joinQN une partes no vacías con ".". Ej: joinQN("pkg","Tipo","M") => "pkg.Tipo.M".
func joinQN(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, ".")
}

// receiverTypeName extrae el nombre del tipo receiver, desreferenciando punteros.
// Ej: *T => "T", T => "T", T[X] (genérico) => "T".
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr: // receiver genérico: T[P]
		return receiverTypeName(t.X)
	case *ast.IndexListExpr: // receiver genérico multi-param: T[P, Q]
		return receiverTypeName(t.X)
	}
	return ""
}

// exprName renderiza una expresión a un nombre dotted best-effort, para callees
// no resueltos. Ej: pkg.Sub => "pkg.Sub", x => "x", (*T) => "T".
func exprName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprName(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return exprName(t.X)
	case *ast.ParenExpr:
		return exprName(t.X)
	case *ast.CallExpr:
		return exprName(t.Fun)
	case *ast.IndexExpr:
		return exprName(t.X)
	}
	return "?"
}

// renderFuncSig produce una firma legible y determinista de una FuncDecl:
// "func (r *T) Name(params) results". Omite el receiver si no hay.
func renderFuncSig(fn *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("func ")
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		b.WriteString("(")
		b.WriteString(renderField(fn.Recv.List[0]))
		b.WriteString(") ")
	}
	b.WriteString(fn.Name.Name)
	b.WriteString(renderFieldList(fn.Type.Params, true))
	if res := renderResults(fn.Type.Results); res != "" {
		b.WriteString(" ")
		b.WriteString(res)
	}
	return b.String()
}

// renderResults renderiza la lista de resultados, con paréntesis solo si hay
// más de un resultado o el único resultado está nombrado.
func renderResults(results *ast.FieldList) string {
	if results == nil || len(results.List) == 0 {
		return ""
	}
	if len(results.List) == 1 && len(results.List[0].Names) == 0 {
		return renderExpr(results.List[0].Type)
	}
	return renderFieldList(results, true)
}

// renderFieldList renderiza params o results entre paréntesis (si withParens).
func renderFieldList(fl *ast.FieldList, withParens bool) string {
	var b strings.Builder
	if withParens {
		b.WriteString("(")
	}
	if fl != nil {
		for i, f := range fl.List {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(renderField(f))
		}
	}
	if withParens {
		b.WriteString(")")
	}
	return b.String()
}

// renderField renderiza un campo "name1, name2 Type" (o solo "Type" si anónimo).
func renderField(f *ast.Field) string {
	names := make([]string, 0, len(f.Names))
	for _, n := range f.Names {
		names = append(names, n.Name)
	}
	typeStr := renderExpr(f.Type)
	if len(names) == 0 {
		return typeStr
	}
	return strings.Join(names, ", ") + " " + typeStr
}

// renderExpr renderiza un tipo a texto determinista. Cubre los casos comunes;
// los no cubiertos caen a "_" para mantener la firma estable y sin pánico.
func renderExpr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return renderExpr(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + renderExpr(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + renderExpr(t.Elt)
		}
		return "[" + renderExpr(t.Len) + "]" + renderExpr(t.Elt)
	case *ast.MapType:
		return "map[" + renderExpr(t.Key) + "]" + renderExpr(t.Value)
	case *ast.ChanType:
		switch t.Dir {
		case ast.SEND:
			return "chan<- " + renderExpr(t.Value)
		case ast.RECV:
			return "<-chan " + renderExpr(t.Value)
		default:
			return "chan " + renderExpr(t.Value)
		}
	case *ast.Ellipsis:
		return "..." + renderExpr(t.Elt)
	case *ast.FuncType:
		return "func" + renderFieldList(t.Params, true) + renderResultsSuffix(t.Results)
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.StructType:
		return "struct{...}"
	case *ast.IndexExpr:
		return renderExpr(t.X) + "[" + renderExpr(t.Index) + "]"
	case *ast.IndexListExpr:
		parts := make([]string, 0, len(t.Indices))
		for _, idx := range t.Indices {
			parts = append(parts, renderExpr(idx))
		}
		return renderExpr(t.X) + "[" + strings.Join(parts, ", ") + "]"
	case *ast.BasicLit:
		return t.Value
	}
	return "_"
}

// renderResultsSuffix renderiza los resultados de un func type embebido (con
// espacio inicial cuando aplica).
func renderResultsSuffix(results *ast.FieldList) string {
	res := renderResults(results)
	if res == "" {
		return ""
	}
	return " " + res
}

// docText normaliza un comment group a una sola línea trim-eada. Devuelve "" si
// no hay doc, para que el caller mapee a NULL/empty sin ambigüedad.
func docText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	return strings.TrimSpace(cg.Text())
}

// baseName extrae el último segmento de un path con separador "/".
func baseName(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
