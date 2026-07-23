// Command size-lint — DOMAINSERV-87. Enforza el límite de tamaño de FUNCIONES
// (≤ 50 líneas por default) sobre código Go nuevo. El límite de archivo es
// advisory (no lo chequea este linter, por decisión del ticket).
//
// Exenciones (no se chequean): *_test.go, *.sql.go generados, internal/seeds/
// (catálogos de datos), migrate/migrations/, y wiring/DI (main.go,
// server_services.go, server_runners.go). Escape-hatch: comentario
// `// size-lint:allow <razón>` en la línea previa a la func.
//
// Baseline: -baseline <file> ignora "relpath:func" ya existentes (deuda
// congelada); CI falla solo ante funciones NUEVAS. -dump imprime los
// violadores actuales (para generar el baseline).
//
// Uso:
//
//	size-lint [-max 50] [-dir .] [-baseline .size-lint-baseline] [-dump]
//
// Exit 1 si hay violaciones no-baseline.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	max := flag.Int("max", 50, "máximo de líneas por función")
	dir := flag.String("dir", ".", "directorio raíz a recorrer")
	baselinePath := flag.String("baseline", ".size-lint-baseline", "archivo de baseline (relpath:func por línea)")
	dump := flag.Bool("dump", false, "imprime los violadores actuales y sale (para generar baseline)")
	flag.Parse()

	baseline := loadBaseline(*baselinePath)
	violations := scan(*dir, *max)

	if *dump {
		for _, v := range violations {
			fmt.Println(v.key())
		}
		return
	}
	var fresh []violation
	for _, v := range violations {
		if _, ok := baseline[v.key()]; !ok {
			fresh = append(fresh, v)
		}
	}
	if len(fresh) > 0 {
		fmt.Fprintf(os.Stderr, "❌ size-lint: %d función(es) nueva(s) > %d líneas (baseline: %d congeladas)\n",
			len(fresh), *max, len(baseline))
		for _, v := range fresh {
			fmt.Fprintf(os.Stderr, "  %s (%d líneas)\n", v.key(), v.lines)
		}
		os.Exit(1)
	}
	fmt.Printf("✅ size-lint OK: 0 funciones nuevas > %d líneas (%d congeladas en baseline)\n", *max, len(baseline))
}

type violation struct {
	path  string
	fn    string
	lines int
}

func (v violation) key() string { return v.path + ":" + v.fn }

func scan(root string, max int) []violation {
	var out []violation
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || exempt(path) {
			return nil
		}
		out = append(out, scanFile(path, max)...)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].key() < out[j].key() })
	return out
}

func scanFile(path string, max int) []violation {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil
	}
	rel := relPath(path)
	var out []violation
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if hasAllow(fn, fset, f) {
			continue
		}
		n := fset.Position(fn.Body.End()).Line - fset.Position(fn.Body.Pos()).Line - 1
		if n > max {
			out = append(out, violation{path: rel, fn: fn.Name.Name, lines: n})
		}
	}
	return out
}

// hasAllow detecta `// size-lint:allow` en el doc-comment de la func.
func hasAllow(fn *ast.FuncDecl, _ *token.FileSet, _ *ast.File) bool {
	if fn.Doc == nil {
		return false
	}
	for _, c := range fn.Doc.List {
		if strings.Contains(c.Text, "size-lint:allow") {
			return true
		}
	}
	return false
}

func exempt(path string) bool {
	p := filepath.ToSlash(path)
	if strings.HasSuffix(p, "_test.go") || strings.HasSuffix(p, ".sql.go") {
		return true
	}
	for _, seg := range []string{"/internal/seeds/", "/migrate/migrations/", "/vendor/"} {
		if strings.Contains(p, seg) {
			return true
		}
	}
	base := filepath.Base(p)
	return base == "main.go" || base == "server_services.go" || base == "server_runners.go"
}

func relPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		if wd, err := os.Getwd(); err == nil {
			if r, err := filepath.Rel(wd, abs); err == nil {
				return filepath.ToSlash(r)
			}
		}
	}
	return filepath.ToSlash(path)
}

func loadBaseline(path string) map[string]struct{} {
	m := map[string]struct{}{}
	f, err := os.Open(path)
	if err != nil {
		return m
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			m[line] = struct{}{}
		}
	}
	return m
}
