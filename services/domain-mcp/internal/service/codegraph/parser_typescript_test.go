//go:build treesitter

package codegraph

import "testing"

func TestParseTypeScript(t *testing.T) {
	src := []byte(`import { x } from 'mod'

interface Reader { read(): void }

type ID = number

class Repo {
  find(): void {
    load()
  }
}

function top(): void {
  helper()
}
`)
	lp, ok := parserForPath("repo.ts")
	if !ok {
		t.Fatal("no parser registered for .ts")
	}
	pf, err := lp.Parse("repo.ts", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pf.Language != "typescript" {
		t.Fatalf("language = %q, want typescript", pf.Language)
	}
	if _, ok := findNode(pf.Nodes, KindInterface, "Reader"); !ok {
		t.Error("missing interface Reader")
	}
	if _, ok := findNode(pf.Nodes, KindType, "ID"); !ok {
		t.Error("missing type alias ID")
	}
	if _, ok := findNode(pf.Nodes, KindType, "Repo"); !ok {
		t.Error("missing class Repo as type node")
	}
	if _, ok := findNode(pf.Nodes, KindMethod, "find"); !ok {
		t.Error("missing method find")
	}
	if !hasEdge(pf.Edges, "typescript.Repo.find", "typescript.Repo", EdgeMethodOf) {
		t.Error("missing method_of find -> Repo")
	}
	if !hasEdge(pf.Edges, "typescript.top", "helper", EdgeCalls) {
		t.Error("missing call top -> helper")
	}
}

func TestParseTSX(t *testing.T) {
	src := []byte(`import { x } from 'mod'

class Button {
  render() {
    draw()
  }
}

const C = () => <div/>
`)
	lp, ok := parserForPath("Button.tsx")
	if !ok {
		t.Fatal("no parser registered for .tsx")
	}
	pf, err := lp.Parse("Button.tsx", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pf.Language != "tsx" {
		t.Fatalf("language = %q, want tsx", pf.Language)
	}
	if _, ok := findNode(pf.Nodes, KindType, "Button"); !ok {
		t.Error("missing class Button as type node")
	}
	if _, ok := findNode(pf.Nodes, KindMethod, "render"); !ok {
		t.Error("missing method render")
	}
	if !hasEdge(pf.Edges, "tsx.Button.render", "draw", EdgeCalls) {
		t.Error("missing call render -> draw")
	}
}
