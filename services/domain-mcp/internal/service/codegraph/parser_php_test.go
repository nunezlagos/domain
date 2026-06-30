//go:build treesitter

package codegraph

import "testing"

func TestParsePHP(t *testing.T) {
	src := []byte(`<?php
use App\Service;

class Repo extends Base {
    public function find() {
        load();
    }
}

function top() {
    helper();
}
`)
	lp, ok := parserForPath("Repo.php")
	if !ok {
		t.Fatal("no parser registered for .php")
	}
	pf, err := lp.Parse("Repo.php", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pf.Language != "php" {
		t.Fatalf("language = %q, want php", pf.Language)
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
	if !hasEdge(pf.Edges, "php.Repo.find", "php.Repo", EdgeMethodOf) {
		t.Error("missing method_of find -> Repo")
	}
	if !hasEdge(pf.Edges, "php.top", "helper", EdgeCalls) {
		t.Error("missing call top -> helper")
	}
}
