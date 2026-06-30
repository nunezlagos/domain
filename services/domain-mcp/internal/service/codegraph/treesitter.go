//go:build treesitter

// treesitter.go — backend tree-sitter para los lenguajes no-Go (Python, PHP,
// JavaScript, TypeScript/TSX). Solo se compila con -tags treesitter.
//
// POR QUÉ DETRÁS DE UN TAG: el binding tree-sitter con cobertura de gramáticas
// (alexaandru/go-tree-sitter-bare + go-sitter-forest) usa CGO. El release del
// repo es CGO_ENABLED=0 y cross-compila a linux/darwin x amd64/arm64; meter CGO
// en el build por defecto lo rompería. Con el tag el operador opta explícitamente
// a un binario con tree-sitter (que NO cross-compila sin toolchains C), y el
// build por defecto sigue intacto y portable.
//
// DISEÑO: un walker genérico (tsParser.Parse) recorre el árbol tree-sitter y, vía
// una config por lenguaje (langSpec) con los nombres de nodo de cada gramática,
// extrae nodos (file/func/method/type/interface/const/var) y aristas
// (defined_in, imports, calls best-effort, method_of). El mapeo es determinista:
// recorrido en orden de hijos nombrados y de-dup/orden en las aristas calls.
//
// Las clases se mapean a kind 'type' (la mig 000178 no tiene kind 'class'). Las
// firmas/QN siguen la convención del parser Go: top-level "lang.Nombre",
// método "lang.Tipo.Metodo".
//
// NOTA API: go-tree-sitter-bare devuelve Node POR VALOR; el "nodo nulo" se
// detecta con IsNull(), no con nil.
package codegraph

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	sitter "github.com/alexaandru/go-tree-sitter-bare"
)

// langSpec parametriza el walker genérico con los nombres de nodo de UNA
// gramática tree-sitter. Cada campo lista los node-types relevantes; un set vacío
// significa "este lenguaje no tiene ese concepto".
type langSpec struct {
	language   string
	extensions []string
	// langFn devuelve el *sitter.Language de la gramática.
	langFn func() *sitter.Language

	// nodos
	funcDecl   map[string]struct{} // funciones top-level -> kind func
	methodDecl map[string]struct{} // métodos dentro de clase -> kind method
	classDecl  map[string]struct{} // clases -> kind type
	ifaceDecl  map[string]struct{} // interfaces -> kind interface
	typeDecl   map[string]struct{} // type aliases -> kind type
	// classBody: nodos cuyo cuerpo agrupa miembros de clase (para method_of).
	classBody map[string]struct{}

	// imports
	importDecl map[string]struct{}

	// calls
	callExpr map[string]struct{} // nodos de llamada -> arista calls

	// nameFields: campos que contienen el identificador de un decl.
	nameFields []string
	// identTypes: node-types que cuentan como "nombre".
	identTypes map[string]struct{}
	// calleeFields: campos del call que contienen el callee (ej "function").
	calleeFields []string
}

func set(items ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, it := range items {
		m[it] = struct{}{}
	}
	return m
}

// tsParser implementa LanguageParser sobre una langSpec.
type tsParser struct {
	spec langSpec
}

func (p tsParser) Language() string     { return p.spec.language }
func (p tsParser) Extensions() []string { return p.spec.extensions }

// Parse construye el ParsedFile recorriendo el árbol tree-sitter de src.
func (p tsParser) Parse(filePath string, src []byte) (*ParsedFile, error) {
	sum := sha256.Sum256(src)
	lang := p.spec.langFn()
	root, err := sitter.Parse(context.Background(), src, lang)
	if err != nil {
		return nil, fmt.Errorf("treesitter parse %s: %w", filePath, err)
	}

	pf := &ParsedFile{
		FilePath:    filePath,
		ContentHash: sum[:],
		Language:    p.spec.language,
	}

	// Nodo file (QN = filePath), igual que el parser Go.
	pf.Nodes = append(pf.Nodes, ParsedNode{
		Kind:          KindFile,
		Name:          baseName(filePath),
		QualifiedName: filePath,
		FilePath:      filePath,
		LineStart:     1,
		LineEnd:       int(root.EndPoint().Row) + 1,
	})

	w := &tsWalk{
		p:        p,
		src:      src,
		filePath: filePath,
		fileQN:   filePath,
		localQNs: map[string]string{},
		pf:       pf,
	}

	// Pasada 1: declaraciones top-level (y miembros de clase) -> nodos + edges
	// method_of/defined_in. Pobla localQNs para resolver calls en la pasada 2.
	w.walkDecls(root, "")
	// imports (file -> path)
	w.collectImports(root)
	// Pasada 2: calls dentro de cada func/method.
	w.collectCalls(root)

	return pf, nil
}

// tsWalk lleva el estado de un recorrido.
type tsWalk struct {
	p        tsParser
	src      []byte
	filePath string
	fileQN   string
	localQNs map[string]string
	pf       *ParsedFile
}

func (w *tsWalk) nodeText(n sitter.Node) string { return n.Content(w.src) }

// lineRange devuelve (startLine, endLine) 1-based.
func (w *tsWalk) lineRange(n sitter.Node) (int, int) {
	return int(n.StartPoint().Row) + 1, int(n.EndPoint().Row) + 1
}

// nameOf extrae el identificador de un decl: primero por campos nameFields,
// luego por el primer hijo nombrado cuyo tipo esté en identTypes.
func (w *tsWalk) nameOf(n sitter.Node) string {
	for _, f := range w.p.spec.nameFields {
		if c := n.ChildByFieldName(f); !c.IsNull() {
			return w.nodeText(c)
		}
	}
	cnt := n.NamedChildCount()
	for i := uint32(0); i < cnt; i++ {
		c := n.NamedChild(i)
		if c.IsNull() {
			continue
		}
		if has(w.p.spec.identTypes, c.Type()) {
			return w.nodeText(c)
		}
	}
	return ""
}

// addNode agrega un nodo y la arista defined_in -> file, y registra el QN local.
func (w *tsWalk) addNode(kind, name, qn string, start, end int) {
	w.pf.Nodes = append(w.pf.Nodes, ParsedNode{
		Kind:          kind,
		Name:          name,
		QualifiedName: qn,
		FilePath:      w.filePath,
		LineStart:     start,
		LineEnd:       end,
	})
	w.pf.Edges = append(w.pf.Edges, ParsedEdge{
		SourceQN: qn,
		TargetQN: w.fileQN,
		EdgeType: EdgeDefinedIn,
	})
	if _, ok := w.localQNs[name]; !ok {
		w.localQNs[name] = qn
	}
}

// walkDecls recorre los hijos nombrados buscando decls top-level / miembros.
func (w *tsWalk) walkDecls(n sitter.Node, classCtx string) {
	cnt := n.NamedChildCount()
	for i := uint32(0); i < cnt; i++ {
		c := n.NamedChild(i)
		if c.IsNull() {
			continue
		}
		w.classifyDecl(c, classCtx)
	}
}

// classifyDecl despacha un nodo según su tipo en la langSpec.
func (w *tsWalk) classifyDecl(c sitter.Node, classCtx string) {
	t := c.Type()
	spec := w.p.spec
	switch {
	case has(spec.classDecl, t), has(spec.ifaceDecl, t):
		w.handleClass(c, t)
	case has(spec.typeDecl, t):
		w.handleType(c)
	case has(spec.methodDecl, t) && classCtx != "":
		w.handleMethod(c, classCtx)
	case has(spec.funcDecl, t):
		w.handleFunc(c)
	default:
		// Descender en contenedores genéricos (export_statement, blocks, etc.)
		// para no perder decls anidados en wrappers de la gramática.
		w.walkDecls(c, classCtx)
	}
}

// handleClass emite un nodo type/interface y recorre su cuerpo por métodos.
func (w *tsWalk) handleClass(c sitter.Node, t string) {
	kind := KindType
	if has(w.p.spec.ifaceDecl, t) {
		kind = KindInterface
	}
	name := w.nameOf(c)
	if name == "" {
		return
	}
	qn := joinQN(w.p.spec.language, name)
	start, end := w.lineRange(c)
	w.addNode(kind, name, qn, start, end)

	body, ok := w.classBodyOf(c)
	if !ok {
		return
	}
	bcnt := body.NamedChildCount()
	for i := uint32(0); i < bcnt; i++ {
		m := body.NamedChild(i)
		if m.IsNull() {
			continue
		}
		if has(w.p.spec.methodDecl, m.Type()) {
			w.handleMethod(m, name)
		}
	}
}

// classBodyOf localiza el nodo cuerpo de una clase (campo "body" o classBody).
func (w *tsWalk) classBodyOf(c sitter.Node) (sitter.Node, bool) {
	if b := c.ChildByFieldName("body"); !b.IsNull() {
		return b, true
	}
	cnt := c.NamedChildCount()
	for i := uint32(0); i < cnt; i++ {
		ch := c.NamedChild(i)
		if ch.IsNull() {
			continue
		}
		if has(w.p.spec.classBody, ch.Type()) {
			return ch, true
		}
	}
	return sitter.Node{}, false
}

// handleMethod emite un nodo method con QN lang.Clase.Metodo y arista method_of.
func (w *tsWalk) handleMethod(m sitter.Node, className string) {
	name := w.nameOf(m)
	if name == "" {
		return
	}
	qn := joinQN(w.p.spec.language, className, name)
	start, end := w.lineRange(m)
	w.pf.Nodes = append(w.pf.Nodes, ParsedNode{
		Kind:          KindMethod,
		Name:          name,
		QualifiedName: qn,
		FilePath:      w.filePath,
		LineStart:     start,
		LineEnd:       end,
	})
	w.pf.Edges = append(w.pf.Edges,
		ParsedEdge{SourceQN: qn, TargetQN: w.fileQN, EdgeType: EdgeDefinedIn},
		ParsedEdge{SourceQN: qn, TargetQN: joinQN(w.p.spec.language, className), EdgeType: EdgeMethodOf},
	)
	if _, ok := w.localQNs[name]; !ok {
		w.localQNs[name] = qn
	}
}

// handleType emite un nodo type (type alias).
func (w *tsWalk) handleType(c sitter.Node) {
	name := w.nameOf(c)
	if name == "" {
		return
	}
	qn := joinQN(w.p.spec.language, name)
	start, end := w.lineRange(c)
	w.addNode(KindType, name, qn, start, end)
}

// handleFunc emite un nodo func top-level.
func (w *tsWalk) handleFunc(c sitter.Node) {
	name := w.nameOf(c)
	if name == "" {
		return
	}
	qn := joinQN(w.p.spec.language, name)
	start, end := w.lineRange(c)
	w.addNode(KindFunc, name, qn, start, end)
}

// collectImports agrega aristas imports file -> path (de-dup + orden).
func (w *tsWalk) collectImports(root sitter.Node) {
	importSet := map[string]struct{}{}
	w.eachDescendant(root, func(n sitter.Node) {
		if !has(w.p.spec.importDecl, n.Type()) {
			return
		}
		for _, path := range w.importPaths(n) {
			if path != "" {
				importSet[path] = struct{}{}
			}
		}
	})
	for path := range importSet {
		w.pf.ImportPaths = append(w.pf.ImportPaths, path)
	}
	sort.Strings(w.pf.ImportPaths)
	for _, path := range w.pf.ImportPaths {
		w.pf.Edges = append(w.pf.Edges, ParsedEdge{
			SourceQN: w.fileQN,
			TargetQN: path,
			EdgeType: EdgeImports,
		})
	}
}

// importPaths extrae los módulos de un nodo import (campo source/module_name o
// el primer string/dotted_name/qualified_name descendiente).
func (w *tsWalk) importPaths(n sitter.Node) []string {
	if s := n.ChildByFieldName("source"); !s.IsNull() {
		return []string{w.trimQuotes(w.nodeText(s))}
	}
	if mn := n.ChildByFieldName("module_name"); !mn.IsNull() {
		return []string{w.nodeText(mn)}
	}
	var out []string
	w.eachDescendant(n, func(c sitter.Node) {
		switch c.Type() {
		case "string", "string_fragment", "dotted_name", "qualified_name", "namespace_name":
			if t := w.trimQuotes(w.nodeText(c)); t != "" {
				out = append(out, t)
			}
		}
	})
	if len(out) > 1 {
		out = out[:1]
	}
	return out
}

func (w *tsWalk) trimQuotes(s string) string {
	return strings.Trim(strings.TrimSpace(s), "\"'`")
}

// collectCalls agrega aristas calls de cada func/method hacia sus callees.
func (w *tsWalk) collectCalls(root sitter.Node) {
	spec := w.p.spec
	w.eachDescendant(root, func(n sitter.Node) {
		t := n.Type()
		if !has(spec.funcDecl, t) && !has(spec.methodDecl, t) {
			return
		}
		srcQN, ok := w.enclosingQN(n)
		if !ok {
			return
		}
		for _, target := range w.callsIn(n) {
			w.pf.Edges = append(w.pf.Edges, ParsedEdge{
				SourceQN: srcQN,
				TargetQN: target,
				EdgeType: EdgeCalls,
			})
		}
	})
}

// enclosingQN reconstruye el QN de un func/method a partir de su nombre y, si es
// método, de su clase contenedora.
func (w *tsWalk) enclosingQN(n sitter.Node) (string, bool) {
	name := w.nameOf(n)
	if name == "" {
		return "", false
	}
	if has(w.p.spec.methodDecl, n.Type()) {
		if cls := w.enclosingClassName(n); cls != "" {
			return joinQN(w.p.spec.language, cls, name), true
		}
	}
	return joinQN(w.p.spec.language, name), true
}

// enclosingClassName sube por los padres buscando una clase contenedora.
func (w *tsWalk) enclosingClassName(n sitter.Node) string {
	for p := n.Parent(); !p.IsNull(); p = p.Parent() {
		if has(w.p.spec.classDecl, p.Type()) || has(w.p.spec.ifaceDecl, p.Type()) {
			return w.nameOf(p)
		}
	}
	return ""
}

// callsIn recorre el cuerpo de una función y devuelve los QN de callees,
// de-duplicados y ordenados.
func (w *tsWalk) callsIn(fn sitter.Node) []string {
	seen := map[string]struct{}{}
	w.eachDescendant(fn, func(n sitter.Node) {
		if !has(w.p.spec.callExpr, n.Type()) {
			return
		}
		callee := w.calleeName(n)
		if callee == "" {
			return
		}
		if qn, ok := w.localQNs[lastIdent(callee)]; ok {
			seen[qn] = struct{}{}
		} else {
			seen[callee] = struct{}{}
		}
	})
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// calleeName extrae el nombre del callee de un nodo de llamada.
func (w *tsWalk) calleeName(call sitter.Node) string {
	for _, f := range w.p.spec.calleeFields {
		if c := call.ChildByFieldName(f); !c.IsNull() {
			return w.lastSegment(c)
		}
	}
	if c := call.NamedChild(0); !c.IsNull() {
		return w.lastSegment(c)
	}
	return ""
}

// lastSegment renderiza un callee a un nombre best-effort.
func (w *tsWalk) lastSegment(n sitter.Node) string {
	if has(w.p.spec.identTypes, n.Type()) {
		return w.nodeText(n)
	}
	for _, f := range []string{"property", "name", "function", "constructor"} {
		if c := n.ChildByFieldName(f); !c.IsNull() {
			return w.nodeText(c)
		}
	}
	return strings.TrimSpace(w.nodeText(n))
}

// eachDescendant aplica fn a cada nodo nombrado del subárbol (incluyendo n).
func (w *tsWalk) eachDescendant(n sitter.Node, fn func(sitter.Node)) {
	fn(n)
	cnt := n.NamedChildCount()
	for i := uint32(0); i < cnt; i++ {
		c := n.NamedChild(i)
		if c.IsNull() {
			continue
		}
		w.eachDescendant(c, fn)
	}
}

// has reporta pertenencia a un set (nil-safe).
func has(m map[string]struct{}, k string) bool {
	if m == nil {
		return false
	}
	_, ok := m[k]
	return ok
}

// lastIdent devuelve el último segmento de un nombre dotted ("a.b.c" -> "c").
func lastIdent(s string) string {
	if i := strings.LastIndexAny(s, ".:>-"); i >= 0 {
		return strings.TrimLeft(s[i+1:], ">")
	}
	return s
}
