package codegraph

import (
	"testing"
)

// findNode busca el primer nodo con (kind, name) dados.
func findNode(nodes []ParsedNode, kind, name string) (ParsedNode, bool) {
	for _, n := range nodes {
		if n.Kind == kind && n.Name == name {
			return n, true
		}
	}
	return ParsedNode{}, false
}

// hasEdge reporta si existe una arista (source, target, type).
func hasEdge(edges []ParsedEdge, src, tgt, typ string) bool {
	for _, e := range edges {
		if e.SourceQN == src && e.TargetQN == tgt && e.EdgeType == typ {
			return true
		}
	}
	return false
}

func TestParseFile_FuncAndImports(t *testing.T) {
	src := []byte(`// Package demo hace demo.
package demo

import (
	"fmt"
	"strings"
)

// Greet saluda.
func Greet(name string) string {
	return fmt.Sprintf("hola %s", strings.ToUpper(name))
}
`)
	pf, err := ParseFile("demo/greet.go", src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	// Nodo file.
	fileNode, ok := findNode(pf.Nodes, KindFile, "greet.go")
	if !ok {
		t.Fatalf("falta nodo file; nodes=%+v", pf.Nodes)
	}
	if fileNode.QualifiedName != "demo/greet.go" {
		t.Errorf("file QN = %q, quiero %q", fileNode.QualifiedName, "demo/greet.go")
	}

	// Nodo func.
	fn, ok := findNode(pf.Nodes, KindFunc, "Greet")
	if !ok {
		t.Fatalf("falta nodo func Greet; nodes=%+v", pf.Nodes)
	}
	if fn.QualifiedName != "demo.Greet" {
		t.Errorf("func QN = %q, quiero %q", fn.QualifiedName, "demo.Greet")
	}
	if fn.Signature != "func Greet(name string) string" {
		t.Errorf("signature = %q", fn.Signature)
	}
	if fn.Doc != "Greet saluda." {
		t.Errorf("doc = %q", fn.Doc)
	}
	if fn.LineStart == 0 || fn.LineEnd < fn.LineStart {
		t.Errorf("líneas inválidas: start=%d end=%d", fn.LineStart, fn.LineEnd)
	}

	// Imports de-duplicados y ordenados.
	if len(pf.ImportPaths) != 2 || pf.ImportPaths[0] != "fmt" || pf.ImportPaths[1] != "strings" {
		t.Errorf("imports = %v", pf.ImportPaths)
	}

	// Edges: imports (file -> path), defined_in (func -> file).
	if !hasEdge(pf.Edges, "demo/greet.go", "fmt", EdgeImports) {
		t.Errorf("falta edge imports file->fmt; edges=%+v", pf.Edges)
	}
	if !hasEdge(pf.Edges, "demo.Greet", "demo/greet.go", EdgeDefinedIn) {
		t.Errorf("falta edge defined_in Greet->file; edges=%+v", pf.Edges)
	}

	// content_hash presente (sha256 = 32 bytes).
	if len(pf.ContentHash) != 32 {
		t.Errorf("content_hash len = %d, quiero 32", len(pf.ContentHash))
	}
}

func TestParseFile_MethodOfAndCalls(t *testing.T) {
	src := []byte(`package demo

type Repo struct{}

func (r *Repo) Save() error {
	return r.validate()
}

func (r *Repo) validate() error {
	return nil
}
`)
	pf, err := ParseFile("demo/repo.go", src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	// Nodo type Repo.
	if _, ok := findNode(pf.Nodes, KindType, "Repo"); !ok {
		t.Fatalf("falta nodo type Repo; nodes=%+v", pf.Nodes)
	}

	// Nodo method Save con QN pkg.Tipo.Metodo.
	save, ok := findNode(pf.Nodes, KindMethod, "Save")
	if !ok {
		t.Fatalf("falta nodo method Save; nodes=%+v", pf.Nodes)
	}
	if save.QualifiedName != "demo.Repo.Save" {
		t.Errorf("method QN = %q, quiero %q", save.QualifiedName, "demo.Repo.Save")
	}
	if save.Signature != "func (r *Repo) Save() error" {
		t.Errorf("signature = %q", save.Signature)
	}

	// method_of: method -> type.
	if !hasEdge(pf.Edges, "demo.Repo.Save", "demo.Repo", EdgeMethodOf) {
		t.Errorf("falta edge method_of Save->Repo; edges=%+v", pf.Edges)
	}

	// calls: Save -> validate (resuelto al QN local).
	if !hasEdge(pf.Edges, "demo.Repo.Save", "demo.Repo.validate", EdgeCalls) {
		t.Errorf("falta edge calls Save->validate; edges=%+v", pf.Edges)
	}
}

func TestParseFile_TypesConstsVarsInterface(t *testing.T) {
	src := []byte(`package demo

const MaxRetries = 3

var defaultName = "x"

type Stringer interface {
	String() string
}

type Config struct {
	Name string
}
`)
	pf, err := ParseFile("demo/types.go", src)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if c, ok := findNode(pf.Nodes, KindConst, "MaxRetries"); !ok || c.QualifiedName != "demo.MaxRetries" {
		t.Errorf("const MaxRetries faltante o QN malo: %+v", c)
	}
	if v, ok := findNode(pf.Nodes, KindVar, "defaultName"); !ok || v.QualifiedName != "demo.defaultName" {
		t.Errorf("var defaultName faltante o QN malo: %+v", v)
	}
	if _, ok := findNode(pf.Nodes, KindInterface, "Stringer"); !ok {
		t.Errorf("interface Stringer faltante; nodes=%+v", pf.Nodes)
	}
	if _, ok := findNode(pf.Nodes, KindType, "Config"); !ok {
		t.Errorf("type Config faltante; nodes=%+v", pf.Nodes)
	}
}

func TestPackageName(t *testing.T) {
	name, err := PackageName([]byte("package foo\n\nfunc Bar() {}\n"))
	if err != nil {
		t.Fatalf("PackageName: %v", err)
	}
	if name != "foo" {
		t.Errorf("PackageName = %q, quiero %q", name, "foo")
	}
}
