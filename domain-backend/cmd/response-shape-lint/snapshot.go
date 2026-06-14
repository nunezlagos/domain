// issue-13.9 — rsl-002..005: validación de rutas (kebab-case, POST create→201)
// y snapshots de endpoint shapes + error codes con modo -update.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Route es un endpoint registrado en api.go.
type Route struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
}

var reRoute = regexp.MustCompile(`mux\.HandleFunc\("(GET|POST|PUT|PATCH|DELETE) ([^"]+)",\s*a\.(\w+)\)`)

// extractRoutes parsea las registraciones mux.HandleFunc de routesFile.
func extractRoutes(routesFile string) ([]Route, error) {
	raw, err := os.ReadFile(routesFile)
	if err != nil {
		return nil, fmt.Errorf("read routes: %w", err)
	}
	var routes []Route
	for _, m := range reRoute.FindAllStringSubmatch(string(raw), -1) {
		routes = append(routes, Route{Method: m[1], Path: m[2], Handler: m[3]})
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})
	return routes, nil
}

var reKebabSegment = regexp.MustCompile(`^[a-z0-9-]+$`)

// lintRoutes valida kebab-case en URLs y que los handlers create* de rutas
// POST referencien http.StatusCreated (201 + Location según api.md).
func lintRoutes(routes []Route, handlersWithCreated map[string]bool) []Violation {
	var out []Violation
	for _, rt := range routes {
		for _, seg := range strings.Split(strings.TrimPrefix(rt.Path, "/"), "/") {
			if seg == "" || strings.HasPrefix(seg, "{") {
				continue
			}
			if !reKebabSegment.MatchString(seg) {
				out = append(out, Violation{
					File: "api.go", Handler: rt.Handler,
					Reason: fmt.Sprintf("route %s %s: segment %q must be kebab-case (no snake_case/uppercase)", rt.Method, rt.Path, seg),
				})
			}
		}
		if rt.Method == "POST" && strings.HasPrefix(rt.Handler, "create") {
			if !handlersWithCreated[rt.Handler] {
				out = append(out, Violation{
					File: "api.go", Handler: rt.Handler,
					Reason: fmt.Sprintf("route POST %s: create handler must respond http.StatusCreated (201)", rt.Path),
				})
			}
		}
	}
	return out
}

// collectHandlerFacts escanea el dir de handlers y devuelve:
//   - handlers que referencian http.StatusCreated
//   - error codes usados en writeError(w, status, "<code>", ...)
func collectHandlerFacts(dir string) (created map[string]bool, errorCodes []string, err error) {
	created = map[string]bool{}
	codeSet := map[string]bool{}
	files, err := collectGoFiles(dir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					if isIdent(sel.X, "http") && sel.Sel.Name == "StatusCreated" {
						created[name] = true
					}
				}
				if call, ok := n.(*ast.CallExpr); ok {
					if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "writeError" && len(call.Args) >= 3 {
						if lit, ok := call.Args[2].(*ast.BasicLit); ok {
							code := strings.Trim(lit.Value, `"`)
							if code != "" {
								codeSet[code] = true
							}
						}
					}
				}
				return true
			})
		}
	}
	for c := range codeSet {
		errorCodes = append(errorCodes, c)
	}
	sort.Strings(errorCodes)
	return created, errorCodes, nil
}

// checkSnapshot compara value contra el snapshot en file. En update=true
// regenera el archivo. Retorna violación descriptiva si hay drift.
func checkSnapshot(file string, value any, update bool) (*Violation, error) {
	current, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	current = append(current, '\n')

	if update {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(file, current, 0o644)
	}

	existing, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return &Violation{
			File:   file,
			Reason: "snapshot missing — run with -update to create it",
		}, nil
	}
	if err != nil {
		return nil, err
	}
	if string(existing) != string(current) {
		return &Violation{
			File:   file,
			Reason: "snapshot drift detected — review API change and run with -update",
		}, nil
	}
	return nil, nil
}

// runShapeChecks ejecuta rutas + snapshots. Retorna violations.
func runShapeChecks(handlerDir, routesFile, snapshotDir string, update bool) ([]Violation, error) {
	routes, err := extractRoutes(routesFile)
	if err != nil {
		return nil, err
	}
	created, errorCodes, err := collectHandlerFacts(handlerDir)
	if err != nil {
		return nil, err
	}

	violations := lintRoutes(routes, created)

	endpointSnap := filepath.Join(snapshotDir, "endpoint_shapes.json")
	if v, err := checkSnapshot(endpointSnap, routes, update); err != nil {
		return nil, err
	} else if v != nil {
		violations = append(violations, *v)
	}
	codesSnap := filepath.Join(snapshotDir, "error_codes.json")
	if v, err := checkSnapshot(codesSnap, errorCodes, update); err != nil {
		return nil, err
	} else if v != nil {
		violations = append(violations, *v)
	}
	return violations, nil
}
