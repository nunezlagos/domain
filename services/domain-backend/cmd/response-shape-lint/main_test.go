package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parseFixture parsea un archivo testdata/*.go.txt como si fuera Go source.
// Los fixtures llevan extensión .go.txt para no entrar al build del package
// (Go ignora todo lo que no termine en .go fuera de testdata, y testdata mismo
// está excluido del build).
func parseFixture(t *testing.T, rel string) (*token.FileSet, string, []Violation, int) {
	t.Helper()
	path := filepath.Join("testdata", rel)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	violations, scanned := lintFile(fset, path, f)
	return fset, path, violations, scanned
}

func TestLint_OK_WriteData(t *testing.T) {
	_, _, violations, scanned := parseFixture(t, "ok_writedata.go.txt")
	if scanned != 1 {
		t.Fatalf("expected 1 handler scanned, got %d", scanned)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestLint_OK_NoContent_Delete(t *testing.T) {
	_, _, violations, scanned := parseFixture(t, "ok_no_content.go.txt")
	if scanned != 1 {
		t.Fatalf("expected 1 handler scanned, got %d", scanned)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations (StatusNoContent allowed), got %d: %v", len(violations), violations)
	}
}

func TestLint_OK_NotModified_ETag(t *testing.T) {
	_, _, violations, scanned := parseFixture(t, "ok_not_modified.go.txt")
	if scanned != 1 {
		t.Fatalf("expected 1 handler scanned, got %d", scanned)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations (StatusNotModified allowed), got %d: %v", len(violations), violations)
	}
}

func TestLint_BAD_RawWrite(t *testing.T) {
	_, _, violations, scanned := parseFixture(t, "bad_raw_write.go.txt")
	if scanned != 1 {
		t.Fatalf("expected 1 handler scanned, got %d", scanned)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Reason, "raw w.Write") {
		t.Fatalf("expected reason to mention raw w.Write, got %q", violations[0].Reason)
	}
}

func TestLint_BAD_JSONEncoder(t *testing.T) {
	_, _, violations, scanned := parseFixture(t, "bad_json_encoder.go.txt")
	if scanned != 1 {
		t.Fatalf("expected 1 handler scanned, got %d", scanned)
	}
	// Esperamos al menos una violation por NewEncoder y otra por WriteHeader
	// con status no-allowed (StatusAccepted).
	if len(violations) < 1 {
		t.Fatalf("expected >=1 violations, got %d: %v", len(violations), violations)
	}
	foundEncoder := false
	for _, v := range violations {
		if strings.Contains(v.Reason, "json.NewEncoder") {
			foundEncoder = true
		}
	}
	if !foundEncoder {
		t.Fatalf("expected json.NewEncoder violation, got %v", violations)
	}
}

func TestLint_BAD_WriteHeader_NonAllowed(t *testing.T) {
	_, _, violations, scanned := parseFixture(t, "bad_writeheader_ok.go.txt")
	if scanned != 1 {
		t.Fatalf("expected 1 handler scanned, got %d", scanned)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation (WriteHeader(StatusOK)), got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Reason, "WriteHeader") {
		t.Fatalf("expected reason to mention WriteHeader, got %q", violations[0].Reason)
	}
}

func TestLint_BAD_FmtFprintf(t *testing.T) {
	_, _, violations, scanned := parseFixture(t, "bad_fmt_fprintf.go.txt")
	if scanned != 1 {
		t.Fatalf("expected 1 handler scanned, got %d", scanned)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0].Reason, "fmt.Fprintf") {
		t.Fatalf("expected reason to mention fmt.Fprintf, got %q", violations[0].Reason)
	}
}

func TestLint_SkipsNonHandlers(t *testing.T) {
	_, _, violations, scanned := parseFixture(t, "non_handler.go.txt")
	if scanned != 0 {
		t.Fatalf("expected 0 handlers scanned (no API receiver), got %d", scanned)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestApiHandlerWriterName_Negative(t *testing.T) {
	cases := []string{
		"non_handler.go.txt",
	}
	for _, c := range cases {
		_, _, _, scanned := parseFixture(t, c)
		if scanned != 0 {
			t.Errorf("%s: expected 0 handlers, got %d", c, scanned)
		}
	}
}
