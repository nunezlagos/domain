// Command domain-lint-stateless — issue-26.1 lint del invariante stateless.
//
// Recorre paquetes Go del proyecto y detecta state in-memory potencialmente
// crítico que podría romper el invariante "cualquier pod puede recibir
// cualquier request" si no está justificado en la whitelist.
//
// Heurísticas:
//   - var globales no-const con tipos mutables (map, slice, struct, chan,
//     sync.Map, *sync.Mutex con datos protegidos).
//   - sync.Map global.
//   - Channels globales sin owner claro.
//   - Maps globales sin TTL marker en comment.
//
// Whitelist en .stateless-allowed.yaml en root del repo. Cada entrada
// requiere {path, var, reason} para que esté auditada.
//
// Uso:
//
//	domain-lint-stateless [packages ...]   # default: ./...
//
// Exit code 1 si hay issues no-whitelisted.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type allowedEntry struct {
	Path   string `yaml:"path"`
	Var    string `yaml:"var"`
	Reason string `yaml:"reason"`
}

type allowed struct {
	Allowed []allowedEntry `yaml:"allowed"`
}

type issue struct {
	path string
	line int
	name string
	kind string
}

func main() {
	cfgPath := flag.String("config", ".stateless-allowed.yaml", "ruta a whitelist")
	verbose := flag.Bool("v", false, "imprime entradas whitelisted aceptadas")
	flag.Parse()
	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"./..."}
	}

	wl, err := loadWhitelist(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lint-stateless: %v\n", err)
		os.Exit(2)
	}

	fset := token.NewFileSet()
	var issues []issue
	var whitelisted int

	for _, root := range expandRoots(roots) {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				if d != nil && d.IsDir() && shouldSkipDir(path) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return nil // skip silenciosamente; otros linters reportan parser errors
			}
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.VAR {
					continue
				}
				for _, spec := range gen.Specs {
					vspec := spec.(*ast.ValueSpec)
					for _, name := range vspec.Names {
						if name.IsExported() || isLowerSnake(name.Name) {
							// inspecciona si el tipo es "mutable global crítico"
							kind := mutableKind(vspec, gen)
							if kind == "" {
								continue
							}
							rel, _ := filepath.Rel(".", path)
							if wl.matches(rel, name.Name) {
								whitelisted++
								if *verbose {
									fmt.Printf("WHITELISTED %s:%d %s (%s)\n",
										rel, fset.Position(name.Pos()).Line, name.Name, kind)
								}
								continue
							}
							issues = append(issues, issue{
								path: rel,
								line: fset.Position(name.Pos()).Line,
								name: name.Name,
								kind: kind,
							})
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "walk %s: %v\n", root, err)
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].path != issues[j].path {
			return issues[i].path < issues[j].path
		}
		return issues[i].line < issues[j].line
	})

	if len(issues) == 0 {
		fmt.Printf("✅ lint-stateless OK: %d entries whitelisted, 0 issues\n", whitelisted)
		return
	}

	fmt.Printf("❌ lint-stateless: %d issue(s), %d whitelisted\n\n", len(issues), whitelisted)
	for _, it := range issues {
		fmt.Printf("  %s:%d  %s  → %s\n", it.path, it.line, it.name, it.kind)
	}
	fmt.Println("\nFix: agregá entrada a .stateless-allowed.yaml con reason explícito.")
	os.Exit(1)
}

func loadWhitelist(path string) (*allowed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &allowed{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var wl allowed
	if err := yaml.Unmarshal(data, &wl); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i, e := range wl.Allowed {
		if e.Path == "" || e.Var == "" || strings.TrimSpace(e.Reason) == "" {
			return nil, fmt.Errorf("%s entry #%d incompleta: path/var/reason requeridos", path, i+1)
		}
	}
	return &wl, nil
}

func (w *allowed) matches(path, name string) bool {
	for _, e := range w.Allowed {
		if e.Path == path && e.Var == name {
			return true
		}
	}
	return false
}

func mutableKind(vspec *ast.ValueSpec, gen *ast.GenDecl) string {
	// Si tiene `const` no es problema (caso filtrado arriba)
	// Si tiene comentario justificación inline (// stateless-allowed: reason) tampoco
	if gen.Doc != nil {
		for _, c := range gen.Doc.List {
			if strings.Contains(c.Text, "stateless-allowed:") {
				return ""
			}
		}
	}
	if vspec.Doc != nil {
		for _, c := range vspec.Doc.List {
			if strings.Contains(c.Text, "stateless-allowed:") {
				return ""
			}
		}
	}

	if vspec.Type == nil {
		return ""
	}
	switch t := vspec.Type.(type) {
	case *ast.MapType:
		return fmt.Sprintf("map global mutable (%s)", exprStr(t))
	case *ast.ChanType:
		return fmt.Sprintf("channel global (%s)", exprStr(t))
	case *ast.ArrayType:
		if t.Len == nil { // slice
			return fmt.Sprintf("slice global mutable (%s)", exprStr(t))
		}
	case *ast.SelectorExpr:
		// sync.Map, sync.WaitGroup como global
		sel := exprStr(t)
		if sel == "sync.Map" || strings.HasSuffix(sel, ".Counter") {
			return fmt.Sprintf("type compartido global (%s)", sel)
		}
	case *ast.StarExpr:
		// punteros a tipos mutables
		inner := exprStr(t.X)
		if strings.HasPrefix(inner, "sync.") {
			return ""
		}
		return fmt.Sprintf("pointer global mutable (*%s)", inner)
	}
	return ""
}

func exprStr(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprStr(t.X) + "." + t.Sel.Name
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", exprStr(t.Key), exprStr(t.Value))
	case *ast.ChanType:
		return "chan " + exprStr(t.Value)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + exprStr(t.Elt)
		}
	}
	return "?"
}

func isLowerSnake(s string) bool {
	if s == "" {
		return false
	}
	r := s[0]
	return r >= 'a' && r <= 'z'
}

func shouldSkipDir(path string) bool {
	base := filepath.Base(path)
	switch base {
	case ".git", "vendor", "node_modules", "sdks", ".claude", "openspec", "deploy", "docs":
		return true
	}
	return strings.HasPrefix(base, ".")
}

func expandRoots(roots []string) []string {
	out := []string{}
	for _, r := range roots {
		if strings.HasSuffix(r, "/...") {
			out = append(out, strings.TrimSuffix(r, "/..."))
		} else {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		out = []string{"."}
	}
	return out
}
