// Command mcp-result-lint — DOMAINSERV-161 (ADR-161.5).
//
// Verifica que ningún tool handler de internal/mcp/server construya su
// *mcp.CallToolResult a mano: el resultado se arma con los helpers canónicos
// (toolResultJSON / mcp.NewToolResultText / mcp.NewToolResultError).
//
// Existe porque toolResultJSON NO es el chokepoint: 17 retornos lo esquivaban
// armando el struct directamente, y son justo los que reventaron en
// DOMAINSERV-108 y 142. Complementa al guard de bytes de ResilientWrapper.Wrap:
// el lint es compile-time y evita que se escriba mal, el wrapper es runtime y
// evita que se escape lo ya escrito.
//
// Es un binario aparte de response-shape-lint, no un flag suyo: aquel detecta
// handlers por la firma HTTP (w http.ResponseWriter, r *http.Request) y compara
// contra snapshots de rutas que no existen en el mundo MCP.
//
// Uso:
//
//	mcp-result-lint
//	mcp-result-lint -dir internal/mcp/server -verbose
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

type Violation struct {
	File string
	Line int
	Msg  string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s:%d: %s", v.File, v.Line, v.Msg)
}

func main() {
	dir := flag.String("dir", "internal/mcp/server", "directorio de tool handlers MCP a scanear")
	verbose := flag.Bool("verbose", false, "imprime archivos scaneados aunque no haya violations")
	flag.Parse()

	violations, scanned, err := lintDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-result-lint: %v\n", err)
		os.Exit(2)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "scanned %d file(s) in %s\n", scanned, *dir)
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
		fmt.Println("mcp-result-lint: OK")
	}
}

// los archivos de test sí pueden construir el struct a mano: es la forma de fabricar
// un resultado arbitrario para ejercitar el guard
func lintDir(dir string) ([]Violation, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, fmt.Errorf("read dir %s: %w", dir, err)
	}

	var violations []Violation
	scanned := 0
	for _, e := range entries {
		nombre := e.Name()
		if e.IsDir() || !strings.HasSuffix(nombre, ".go") || strings.HasSuffix(nombre, "_test.go") {
			continue
		}
		ruta := filepath.Join(dir, nombre)
		vs, err := lintFile(ruta)
		if err != nil {
			return nil, scanned, err
		}
		scanned++
		violations = append(violations, vs...)
	}
	return violations, scanned, nil
}

func lintFile(ruta string) ([]Violation, error) {
	fset := token.NewFileSet()
	archivo, err := parser.ParseFile(fset, ruta, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", ruta, err)
	}

	permitidas := lineasConEscapeHatch(fset, archivo)

	var violations []Violation
	ast.Inspect(archivo, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !esCallToolResult(lit.Type) {
			return true
		}
		linea := fset.Position(lit.Pos()).Line
		if permitidas[linea] {
			return true
		}
		violations = append(violations, Violation{
			File: ruta,
			Line: linea,
			Msg:  "mcp.CallToolResult armado a mano: usar toolResultJSON, mcp.NewToolResultText o mcp.NewToolResultError",
		})
		return true
	})
	return violations, nil
}

// Escape-hatch con razón obligatoria, mismo patrón que size-lint:allow del repo. Marca la
// línea del comentario y la siguiente, así sirve puesto arriba o al lado del literal.
// El único caso legítimo previsto es la implementación del propio helper canónico: sin
// esta salida, el helper que centraliza la construcción se reportaría a sí mismo.
func lineasConEscapeHatch(fset *token.FileSet, archivo *ast.File) map[int]bool {
	const marca = "mcp-result-lint:allow"
	permitidas := map[int]bool{}
	for _, grupo := range archivo.Comments {
		for _, c := range grupo.List {
			idx := strings.Index(c.Text, marca)
			if idx < 0 {
				continue
			}
			// sin razón el hatch no vale: un allow mudo es deuda sin dueño
			if strings.TrimSpace(c.Text[idx+len(marca):]) == "" {
				continue
			}
			linea := fset.Position(c.Pos()).Line
			permitidas[linea] = true
			permitidas[linea+1] = true
		}
	}
	return permitidas
}

// matchea el selector mcp.CallToolResult, con o sin & adelante: el AST del composite
// literal expone el tipo igual en los dos casos
func esCallToolResult(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != "CallToolResult" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "mcp"
}
