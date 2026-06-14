// Command response-shape-lint — issue-13.9 entrypoint CLI.
//
// Verifica que cada handler HTTP en internal/api/handler/*.go use los helpers
// canónicos de response shape (writeData / writeDataWithMeta / writeError)
// y no escriba al ResponseWriter de forma cruda (w.Write, json.NewEncoder(w),
// fmt.Fprintf(w, ...), etc.).
//
// Uso:
//
//	response-shape-lint                                  # default dir
//	response-shape-lint -dir internal/api/handler
//	response-shape-lint -dir internal/api/handler -verbose
//
// Exit code: 0 limpio, 1 violations, 2 errores I/O.
package main

import (
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
	dir := flag.String("dir", "internal/api/handler", "directorio de handlers HTTP a scanear")
	routes := flag.String("routes", "internal/api/handler/api.go", "archivo con registraciones mux.HandleFunc")
	snapshotDir := flag.String("snapshot-dir", "internal/api/handler/testdata/api", "directorio de snapshots endpoint_shapes.json + error_codes.json")
	update := flag.Bool("update", false, "regenera snapshots en lugar de comparar")
	verbose := flag.Bool("verbose", false, "imprime handlers scaneados aunque no haya violations")
	flag.Parse()

	violations, scanned, err := lintDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "response-shape-lint: %v\n", err)
		os.Exit(2)
	}

	shapeViolations, err := runShapeChecks(*dir, *routes, *snapshotDir, *update)
	if err != nil {
		fmt.Fprintf(os.Stderr, "response-shape-lint: %v\n", err)
		os.Exit(2)
	}
	violations = append(violations, shapeViolations...)

	if *verbose {
		fmt.Fprintf(os.Stderr, "scanned %d handler(s) in %s\n", scanned, *dir)
	}

	if len(violations) > 0 {
		sort.Slice(violations, func(i, j int) bool {
			if violations[i].File != violations[j].File {
				return violations[i].File < violations[j].File
			}
			return violations[i].Line < violations[j].Line
		})
		for _, v := range violations {
			fmt.Println(v.String())
		}
		fmt.Fprintf(os.Stderr, "\n%d violation(s) found\n", len(violations))
		os.Exit(1)
	}

	if *verbose {
		fmt.Println("response-shape-lint: OK")
	}
}

// Violation reporta un uso prohibido detectado en un handler.
type Violation struct {
	File    string
	Line    int
	Handler string
	Reason  string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s:%d: handler %s %s", v.File, v.Line, v.Handler, v.Reason)
}

// lintDir scanea recursivamente .go files (no test, no api.go) y devuelve violations.
func lintDir(dir string) (violations []Violation, scanned int, err error) {
	files, err := collectGoFiles(dir)
	if err != nil {
		return nil, 0, err
	}

	fset := token.NewFileSet()
	for _, path := range files {
		base := filepath.Base(path)
		if base == "api.go" {
			continue
		}
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			return nil, 0, fmt.Errorf("parse %s: %w", path, err)
		}
		fileViolations, fileScanned := lintFile(fset, path, f)
		violations = append(violations, fileViolations...)
		scanned += fileScanned
	}
	return violations, scanned, nil
}

func collectGoFiles(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s no es directorio", dir)
	}
	var files []string
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// lintFile inspecciona funciones cuya firma sea
// `func (X *API) name(w http.ResponseWriter, r *http.Request)` y reporta
// usos prohibidos del ResponseWriter.
func lintFile(fset *token.FileSet, path string, f *ast.File) (violations []Violation, scanned int) {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		writerName, ok := apiHandlerWriterName(fn)
		if !ok {
			continue
		}
		scanned++
		if hasAllowDirective(fn) {
			continue
		}
		violations = append(violations, scanHandler(fset, path, fn, writerName)...)
	}
	return violations, scanned
}

// hasAllowDirective detecta `// response-shape-lint:allow <reason>` en el doc
// comment del handler. La reason es obligatoria (directiva sin reason no aplica).
// Uso legítimo: streams SSE / binarios que no responden JSON envelope.
func hasAllowDirective(fn *ast.FuncDecl) bool {
	if fn.Doc == nil {
		return false
	}
	for _, c := range fn.Doc.List {
		text := strings.TrimPrefix(c.Text, "//")
		text = strings.TrimSpace(text)
		if reason, ok := strings.CutPrefix(text, "response-shape-lint:allow"); ok {
			if strings.TrimSpace(reason) != "" {
				return true
			}
		}
	}
	return false
}

// apiHandlerWriterName devuelve el nombre del parámetro http.ResponseWriter
// si la función es un handler `func (X *API) name(w http.ResponseWriter, r *http.Request)`.
func apiHandlerWriterName(fn *ast.FuncDecl) (string, bool) {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return "", false
	}
	star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return "", false
	}
	ident, ok := star.X.(*ast.Ident)
	if !ok || ident.Name != "API" {
		return "", false
	}
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 2 {
		return "", false
	}
	// Param 0: w http.ResponseWriter
	p0 := fn.Type.Params.List[0]
	if len(p0.Names) != 1 {
		return "", false
	}
	if !isSelector(p0.Type, "http", "ResponseWriter") {
		return "", false
	}
	// Param 1: r *http.Request
	p1 := fn.Type.Params.List[1]
	if len(p1.Names) != 1 {
		return "", false
	}
	star2, ok := p1.Type.(*ast.StarExpr)
	if !ok {
		return "", false
	}
	if !isSelector(star2.X, "http", "Request") {
		return "", false
	}
	return p0.Names[0].Name, true
}

func isSelector(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == pkg && sel.Sel.Name == name
}

// scanHandler busca dentro del body llamadas crudas al writer.
func scanHandler(fset *token.FileSet, path string, fn *ast.FuncDecl, writer string) []Violation {
	var out []Violation
	handlerName := fn.Name.Name
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		pos := fset.Position(call.Pos())
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			// w.Write([]byte...) / w.WriteHeader(status) / w.Header().Set(...)
			if isIdent(fun.X, writer) {
				switch fun.Sel.Name {
				case "Write":
					out = append(out, Violation{
						File: path, Line: pos.Line, Handler: handlerName,
						Reason: "uses raw " + writer + ".Write — use writeData/writeError instead",
					})
				case "WriteHeader":
					if !allowedWriteHeader(call) {
						out = append(out, Violation{
							File: path, Line: pos.Line, Handler: handlerName,
							Reason: "uses raw " + writer + ".WriteHeader(<non-204/304 status>) — use writeData/writeError instead",
						})
					}
				}
				return true
			}
			// json.NewEncoder(w).Encode(...) — detect Encode whose receiver is NewEncoder(w)
			if fun.Sel.Name == "Encode" && isJSONNewEncoderOnWriter(fun.X, writer) {
				out = append(out, Violation{
					File: path, Line: pos.Line, Handler: handlerName,
					Reason: "uses raw json.NewEncoder(" + writer + ").Encode — use writeData/writeError instead",
				})
				return true
			}
			// fmt.Fprintf(w, ...) / fmt.Fprintln(w, ...) / fmt.Fprint(w, ...)
			if isSelector(fun, "fmt", "Fprintf") || isSelector(fun, "fmt", "Fprintln") || isSelector(fun, "fmt", "Fprint") {
				if len(call.Args) >= 1 && isIdent(call.Args[0], writer) {
					out = append(out, Violation{
						File: path, Line: pos.Line, Handler: handlerName,
						Reason: "uses raw fmt.F" + fun.Sel.Name[1:] + "(" + writer + ", ...) — use writeData/writeError instead",
					})
				}
			}
			// io.WriteString(w, ...) / io.Copy(w, ...)
			if isSelector(fun, "io", "WriteString") || isSelector(fun, "io", "Copy") {
				if len(call.Args) >= 1 && isIdent(call.Args[0], writer) {
					out = append(out, Violation{
						File: path, Line: pos.Line, Handler: handlerName,
						Reason: "uses raw io." + fun.Sel.Name + "(" + writer + ", ...) — use writeData/writeError instead",
					})
				}
			}
		}
		return true
	})
	return out
}

func isIdent(e ast.Expr, name string) bool {
	ident, ok := e.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == name
}

// isJSONNewEncoderOnWriter detecta `json.NewEncoder(<writer>)` como receptor.
func isJSONNewEncoderOnWriter(e ast.Expr, writer string) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	if !isSelector(call.Fun, "json", "NewEncoder") {
		return false
	}
	if len(call.Args) != 1 {
		return false
	}
	return isIdent(call.Args[0], writer)
}

// allowedWriteHeader devuelve true si w.WriteHeader(X) usa un status
// considerado "standalone-OK": StatusNoContent (DELETE) o StatusNotModified (ETag 304).
func allowedWriteHeader(call *ast.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	sel, ok := call.Args[0].(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if !isIdent(sel.X, "http") {
		return false
	}
	switch sel.Sel.Name {
	case "StatusNoContent", "StatusNotModified":
		return true
	}
	return false
}
