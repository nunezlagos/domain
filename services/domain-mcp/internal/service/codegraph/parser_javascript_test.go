//go:build treesitter

package codegraph

import "testing"

func TestParseJavaScript(t *testing.T) {
	src := []byte(`import { x } from 'mod'

class Repo extends Base {
  find() {
    load()
  }
}

function top() {
  helper()
}
`)
	lp, ok := parserForPath("repo.js")
	if !ok {
		t.Fatal("no parser registered for .js")
	}
	pf, err := lp.Parse("repo.js", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pf.Language != "javascript" {
		t.Fatalf("language = %q, want javascript", pf.Language)
	}
	if _, ok := findNode(pf.Nodes, KindType, "Repo"); !ok {
		t.Error("missing class Repo as type node")
	}
	if _, ok := findNode(pf.Nodes, KindMethod, "find"); !ok {
		t.Error("missing method find")
	}
	if _, ok := findNode(pf.Nodes, KindFunc, "top"); !ok {
		t.Error("missing func top")
	}
	if !hasEdge(pf.Edges, "javascript.Repo.find", "javascript.Repo", EdgeMethodOf) {
		t.Error("missing method_of find -> Repo")
	}
	if !hasEdge(pf.Edges, "repo.js", "mod", EdgeImports) {
		t.Error("missing import mod")
	}
	if !hasEdge(pf.Edges, "javascript.top", "helper", EdgeCalls) {
		t.Error("missing call top -> helper")
	}
}
