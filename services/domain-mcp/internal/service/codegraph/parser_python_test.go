//go:build treesitter

package codegraph

import "testing"

func TestParsePython(t *testing.T) {
	src := []byte(`import os
from a.b import c

class Repo(Base):
    def find(self):
        load()

def top():
    helper()
`)
	lp, ok := parserForPath("repo.py")
	if !ok {
		t.Fatal("no parser registered for .py")
	}
	pf, err := lp.Parse("repo.py", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pf.Language != "python" {
		t.Fatalf("language = %q, want python", pf.Language)
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
	if !hasEdge(pf.Edges, "python.Repo.find", "python.Repo", EdgeMethodOf) {
		t.Error("missing method_of find -> Repo")
	}
	if !hasEdge(pf.Edges, "python.top", "repo.py", EdgeDefinedIn) {
		t.Error("missing defined_in top -> file")
	}
	if !hasEdge(pf.Edges, "repo.py", "os", EdgeImports) {
		t.Error("missing import os")
	}
	if !hasEdge(pf.Edges, "python.top", "helper", EdgeCalls) {
		t.Error("missing call top -> helper")
	}
}
