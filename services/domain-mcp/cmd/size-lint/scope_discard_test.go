package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// DOMAINSERV-187: hay firmas que PIDEN un parámetro de scope y no lo usan. El efecto es
// que todo consumidor de esa función PARECE scopear sin scopear.
//
// Esa es exactamente la clase de bug que dejó pasar el leak cross-tenant de
// DOMAINSERV-182: el handler de knowledge parseaba el orgID del Principal y lo pasaba al
// service, el service lo ignoraba porque la migración 000142 le había borrado la columna
// a la tabla, y la firma se veía correcta. Nadie lo notó en review.
//
// El modelo de la plataforma es SINGLE-TENANT por decisión previa del proyecto: la
// migración 000142 eliminó organization_id de todas las tablas, la 000143 dropeó la
// tabla organizations, y los headers de la 000180 y la 000277 lo declaran por escrito.
// O sea que el problema NO es que falte aislamiento por org: es que las firmas todavía
// prometen uno que no existe.
//
// POR QUÉ AST Y NO REGEX (ampliado el 2026-07-29): la primera versión de este guard
// buscaba `_ = orgID` con un regex por línea, y eso dejaba fuera la mayor parte del
// problema. Medido: 21 descartes explícitos contra 73 firmas que aceptan el parámetro y
// NUNCA lo mencionan. Go no marca parámetros sin usar, así que esas 73 no dejan rastro
// grep-able. El regex además contaba comentarios: una línea que MENCIONA `_ = orgID`
// dentro de un comentario inflaba el conteo del archivo.
//
// Con el AST las dos formas son un solo concepto —una firma que promete scope y no lo
// usa— y los comentarios no participan.
//
// Este guard NO exige limpiarlas. Congela las conocidas en un baseline y falla ante una
// NUEVA, igual que cmd/size-lint con .size-lint-baseline. Eliminar los parámetros toca
// 23+ archivos y cientos de call sites para un cambio de comportamiento nulo, así que es
// deuda con fundamento, no una tarea pendiente de este guard.
//
// ALCANCE: funciones y métodos con cuerpo. Los métodos declarados en una interfaz también
// prometen scope, pero no tienen cuerpo donde verificar el uso: la implementación que la
// satisface sí lo tiene y es la que este guard cuenta.

const scopeDiscardBaselinePath = "../../.scope-discard-baseline"

// parametrosDeScope son los nombres de parámetro que prometen aislamiento por
// organización o tenant. Un `err` o un `ctx` sin usar son otra cosa y no prometen nada.
var parametrosDeScope = map[string]bool{
	"orgID": true, "orgId": true,
	"organizationID": true, "organizationId": true,
	"tenantID": true, "tenantId": true,
}

// firmaSinScope describe un parámetro de scope que la función acepta y no usa.
type firmaSinScope struct {
	archivo  string
	linea    int
	funcion  string
	param    string
	explicit bool // true si hay un `_ = orgID`; false si el parámetro nunca se menciona
}

// posicionesDeDescarte devuelve las posiciones de los identificadores que solo aparecen
// para ser descartados —el `orgID` de un `_ = orgID`— y el conjunto de nombres así
// descartados. Contar esas menciones como uso haría que un descarte explícito pase por
// parámetro usado.
func posicionesDeDescarte(cuerpo *ast.BlockStmt) (map[token.Pos]bool, map[string]bool) {
	descartados := map[token.Pos]bool{}
	nombres := map[string]bool{}
	ast.Inspect(cuerpo, func(n ast.Node) bool {
		asig, ok := n.(*ast.AssignStmt)
		if !ok || len(asig.Lhs) != 1 || len(asig.Rhs) != 1 {
			return true
		}
		lhs, ok := asig.Lhs[0].(*ast.Ident)
		if !ok || lhs.Name != "_" {
			return true
		}
		if rhs, ok := asig.Rhs[0].(*ast.Ident); ok {
			descartados[rhs.Pos()] = true
			nombres[rhs.Name] = true
		}
		return true
	})
	return descartados, nombres
}

// usosReales cuenta las menciones del identificador en el cuerpo, sin contar las que
// existen solo para descartarlo.
func usosReales(cuerpo *ast.BlockStmt, nombre string, descartados map[token.Pos]bool) int {
	usos := 0
	ast.Inspect(cuerpo, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if ok && id.Name == nombre && !descartados[id.Pos()] {
			usos++
		}
		return true
	})
	return usos
}

func nombreDeFuncion(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	var receptor strings.Builder
	if err := printerTipo(&receptor, fd.Recv.List[0].Type); err != nil {
		return fd.Name.Name
	}
	return "(" + receptor.String() + ")." + fd.Name.Name
}

// printerTipo escribe la forma textual de un tipo de receptor (`*Service`, `Service`).
// No usa go/printer para no arrastrar el fset: solo necesita punteros y nombres.
func printerTipo(w *strings.Builder, t ast.Expr) error {
	switch v := t.(type) {
	case *ast.StarExpr:
		w.WriteString("*")
		return printerTipo(w, v.X)
	case *ast.Ident:
		w.WriteString(v.Name)
		return nil
	case *ast.IndexExpr:
		return printerTipo(w, v.X)
	default:
		return fmt.Errorf("tipo de receptor no soportado")
	}
}

// analizarArchivo devuelve las firmas del archivo que aceptan un parámetro de scope sin
// usarlo. Parsea sin comentarios: una mención dentro de un comentario no es un descarte.
func analizarArchivo(fset *token.FileSet, path, rel string) ([]firmaSinScope, error) {
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parseando %s: %w", path, err)
	}

	var hallazgos []firmaSinScope
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		descartados, nombresDescartados := posicionesDeDescarte(fd.Body)
		for _, campo := range fd.Type.Params.List {
			for _, nombre := range campo.Names {
				if !parametrosDeScope[nombre.Name] {
					continue
				}
				if usosReales(fd.Body, nombre.Name, descartados) > 0 {
					continue
				}
				hallazgos = append(hallazgos, firmaSinScope{
					archivo:  rel,
					linea:    fset.Position(nombre.Pos()).Line,
					funcion:  nombreDeFuncion(fd),
					param:    nombre.Name,
					explicit: nombresDescartados[nombre.Name],
				})
			}
		}
	}
	return hallazgos, nil
}

// firmasSinScopePorArchivo camina el árbol de producción y agrupa los hallazgos por
// archivo. El baseline es por archivo y no por línea a propósito: una edición que corre
// las líneas sin agregar firmas no debería tocar el baseline.
func firmasSinScopePorArchivo(t *testing.T, raiz string) (map[string]int, []firmaSinScope) {
	t.Helper()
	fset := token.NewFileSet()
	porArchivo := map[string]int{}
	var todos []firmaSinScope

	err := filepath.WalkDir(raiz, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(raiz, path)
		rel = filepath.ToSlash(rel)
		hallazgos, aerr := analizarArchivo(fset, path, rel)
		if aerr != nil {
			return aerr
		}
		porArchivo[rel] += len(hallazgos)
		todos = append(todos, hallazgos...)
		return nil
	})
	if err != nil {
		t.Fatalf("recorriendo %s: %v", raiz, err)
	}
	for archivo, n := range porArchivo {
		if n == 0 {
			delete(porArchivo, archivo)
		}
	}
	return porArchivo, todos
}

func cargarBaselineDescartes(t *testing.T) map[string]int {
	t.Helper()
	f, err := os.Open(scopeDiscardBaselinePath)
	if err != nil {
		t.Fatalf("sin baseline en %s no se puede distinguir deuda vieja de deuda nueva: %v",
			scopeDiscardBaselinePath, err)
	}
	defer f.Close()

	baseline := map[string]int{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		linea := strings.TrimSpace(sc.Text())
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		i := strings.LastIndex(linea, ":")
		if i < 0 {
			t.Fatalf("línea de baseline mal formada (se espera relpath:count): %q", linea)
		}
		n, cerr := strconv.Atoi(strings.TrimSpace(linea[i+1:]))
		if cerr != nil {
			t.Fatalf("count no numérico en el baseline: %q", linea)
		}
		baseline[strings.TrimSpace(linea[:i])] = n
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("leyendo baseline: %v", err)
	}
	return baseline
}

// TestScopeDiscard_RegenerarBaseline reescribe el baseline con la realidad actual. Corre
// solo con SCOPE_DISCARD_REGEN=1 para que nunca se dispare en CI: un guard que se
// auto-actualiza al fallar no vigila nada. Existe porque el baseline anterior decía
// "regenerar con go test" sin que hubiera con qué, así que se mantenía a mano.
func TestScopeDiscard_RegenerarBaseline(t *testing.T) {
	if os.Getenv("SCOPE_DISCARD_REGEN") != "1" {
		t.Skip("solo con SCOPE_DISCARD_REGEN=1")
	}
	actual, todos := firmasSinScopePorArchivo(t, "../..")

	archivos := make([]string, 0, len(actual))
	for a := range actual {
		archivos = append(archivos, a)
	}
	sort.Strings(archivos)

	var explicitos int
	for _, h := range todos {
		if h.explicit {
			explicitos++
		}
	}

	var b strings.Builder
	b.WriteString(encabezadoBaseline(len(todos), explicitos, len(todos)-explicitos, len(archivos)))
	for _, a := range archivos {
		fmt.Fprintf(&b, "%s:%d\n", a, actual[a])
	}
	if err := os.WriteFile(scopeDiscardBaselinePath, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("escribiendo baseline: %v", err)
	}
	t.Logf("baseline regenerado: %d firmas (%d explícitas, %d implícitas) en %d archivos",
		len(todos), explicitos, len(todos)-explicitos, len(archivos))
}

func encabezadoBaseline(total, explicitos, implicitos, archivos int) string {
	return fmt.Sprintf(`# DOMAINSERV-187 — firmas que piden un parámetro de scope y no lo usan, congeladas.
#
# Formato: relpath:count — cuántas firmas de ese archivo aceptan orgID (o una variante:
# orgId, organizationID, organizationId, tenantID, tenantId) sin usarlo en el cuerpo.
# Por archivo y no por línea a propósito: una edición que corre las líneas sin agregar
# firmas no debería tocar este archivo.
#
# Estado congelado: %d firmas en %d archivos — %d con descarte explícito (`+"`_ = orgID`"+`)
# y %d que nunca mencionan el parámetro. Las segundas son invisibles a un grep, y son la
# mayoría: por eso el guard analiza el AST y no busca un patrón de texto.
#
# La plataforma es SINGLE-TENANT: la migración 000142 eliminó organization_id de todas las
# tablas, la 000143 dropeó la tabla organizations, y los headers de la 000180 y la 000277
# lo declaran por escrito. Estas firmas prometen un aislamiento por org que ya no existe.
#
# NO se limpian acá: eliminar los parámetros toca 23+ archivos y cientos de call sites,
# con cadenas de hasta 3 capas (ListFiltered → ClientSvc.Get → repo.GetBySlug), para un
# cambio de comportamiento nulo. Es deuda con fundamento, no una tarea pendiente de este
# guard, cuyo único trabajo es que la deuda no CREZCA.
#
# Regenerar: SCOPE_DISCARD_REGEN=1 go test -run TestScopeDiscard_RegenerarBaseline ./cmd/size-lint/...
`, total, archivos, explicitos, implicitos)
}

// Una firma NUEVA que pide scope sin usarlo promete un aislamiento que no existe. El
// guard falla y el mensaje explica la alternativa, porque un guard que solo dice "no
// permitido" invita a agregar la línea al baseline sin pensarlo.
func TestScopeDiscard_NingunArchivoSupereSuBaseline(t *testing.T) {
	actual, todos := firmasSinScopePorArchivo(t, "../..")
	baseline := cargarBaselineDescartes(t)

	detallePorArchivo := map[string][]string{}
	for _, h := range todos {
		forma := "nunca se menciona"
		if h.explicit {
			forma = "descartado con `_ = " + h.param + "`"
		}
		detallePorArchivo[h.archivo] = append(detallePorArchivo[h.archivo],
			fmt.Sprintf("      %s:%d %s pide %s y %s", h.archivo, h.linea, h.funcion, h.param, forma))
	}

	var nuevos []string
	for archivo, n := range actual {
		if n <= baseline[archivo] {
			continue
		}
		detalle := detallePorArchivo[archivo]
		sort.Strings(detalle)
		nuevos = append(nuevos, fmt.Sprintf("  %s: %d firma(s), baseline %d\n%s",
			archivo, n, baseline[archivo], strings.Join(detalle, "\n")))
	}
	if len(nuevos) == 0 {
		return
	}
	sort.Strings(nuevos)
	t.Errorf(`firma(s) NUEVA(s) que piden un parámetro de scope sin usarlo, en %d archivo(s):
%s

Una firma que acepta orgID y no lo usa hace que quien la lee —o quien le pasa el valor—
asuma que scopea. Es lo que dejó pasar el leak cross-tenant de DOMAINSERV-182, donde el
handler pasaba un orgID que el service ignoraba.

La plataforma es SINGLE-TENANT (migraciones 000142 y 000143). Si la función no va a
scopear por org, la alternativa correcta es NO ACEPTAR el parámetro. Si el scope que
corresponde es por proyecto, el parámetro es un project_id y se usa en el WHERE.

Descartarlo con `+"`_ = orgID`"+` no cambia nada: este guard cuenta las dos formas igual,
porque las dos dejan la misma firma mentirosa.

Sumar la línea al baseline es válido solo si la firma es deliberada y está explicada en
el código.`, len(nuevos), strings.Join(nuevos, "\n"))
}

// El baseline tiene que poder APRETARSE: si alguien limpia firmas y el baseline queda
// alto, el guard deja de vigilar esa diferencia en silencio. No es un error —bajar la
// deuda siempre está permitido— así que avisa sin fallar.
func TestScopeDiscard_BaselineNoQuedaMasFlojoQueLaRealidad(t *testing.T) {
	actual, _ := firmasSinScopePorArchivo(t, "../..")
	baseline := cargarBaselineDescartes(t)

	var flojos []string
	for archivo, n := range baseline {
		if actual[archivo] < n {
			flojos = append(flojos, fmt.Sprintf("  %s: baseline %d, real %d", archivo, n, actual[archivo]))
		}
	}
	if len(flojos) > 0 {
		sort.Strings(flojos)
		t.Logf("el baseline quedó más flojo que la realidad en %d archivo(s) — conviene apretarlo:\n%s",
			len(flojos), strings.Join(flojos, "\n"))
	}
}

// El guard tiene que estar mirando algo: si el análisis deja de encontrar firmas —por un
// rename de los parámetros, por ejemplo— los dos tests de arriba pasarían vacíos y nadie
// se enteraría de que la vigilancia se apagó.
func TestScopeDiscard_ElGuardNoEstaMirandoUnConjuntoVacio(t *testing.T) {
	_, todos := firmasSinScopePorArchivo(t, "../..")
	if len(todos) == 0 {
		t.Fatal("cero firmas con parámetro de scope sin usar en todo el repo: o se limpiaron " +
			"todas (y hay que borrar este guard con su baseline) o los nombres de parámetro " +
			"cambiaron y parametrosDeScope quedó desactualizado")
	}
}

// El AST tiene que ver las dos formas de la misma falla. Si una futura refactorización
// del guard vuelve a mirar solo los `_ = orgID`, este test lo atrapa: el repo tiene
// firmas de los dos tipos y ambas deben aparecer.
func TestScopeDiscard_DetectaDescarteExplicitoYParametroImplicito(t *testing.T) {
	_, todos := firmasSinScopePorArchivo(t, "../..")

	var explicitos, implicitos int
	for _, h := range todos {
		if h.explicit {
			explicitos++
			continue
		}
		implicitos++
	}
	if explicitos == 0 {
		t.Error("ningún descarte explícito (`_ = orgID`) detectado: el guard dejó de ver la " +
			"forma que motivó DOMAINSERV-187")
	}
	if implicitos == 0 {
		t.Error("ninguna firma con parámetro de scope nunca mencionado: esa es la forma que el " +
			"regex original no veía y la razón por la que este guard pasó a AST")
	}
}
