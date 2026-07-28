package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// DOMAINSERV-187: hay firmas que PIDEN un parámetro de scope y lo descartan con un
// `_ = orgID` explícito. El descarte no está escondido —la línea se lee— pero el efecto
// sí: todo consumidor de esa función PARECE scopear sin scopear.
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
// Este guard NO exige limpiarlas. Congela las conocidas en un baseline y falla ante una
// NUEVA, igual que cmd/size-lint con .size-lint-baseline: "CI falla solo ante funciones
// NUEVAS que superen el umbral, no ante las ya existentes". Eliminar los parámetros
// tocaría 90 call sites para un cambio de comportamiento nulo, así que es deuda con
// fundamento, no una tarea pendiente de este guard.

const scopeDiscardBaselinePath = "../../.scope-discard-baseline"

// descarteDeScope matchea el descarte de un parámetro de scope: `_ = orgID` y sus
// variantes de nombre. Solo cuenta los de scope: un `_ = err` o un `_ = ctx` son otra
// cosa y no prometen aislamiento.
var descarteDeScope = regexp.MustCompile(`_\s*=\s*(orgID|orgId|organizationID|organizationId|tenantID|tenantId)\b`)

// contarDescartesPorArchivo camina el árbol y devuelve cuántos descartes de scope tiene
// cada archivo .go de producción. El baseline es por archivo y no por línea a propósito:
// una edición que corre las líneas sin agregar descartes no debería tocar el baseline.
func contarDescartesPorArchivo(t *testing.T, raiz string) map[string]int {
	t.Helper()
	porArchivo := map[string]int{}

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
		f, oerr := os.Open(path)
		if oerr != nil {
			return oerr
		}
		defer f.Close()

		rel, _ := filepath.Rel(raiz, path)
		rel = filepath.ToSlash(rel)
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		for sc.Scan() {
			if descarteDeScope.MatchString(sc.Text()) {
				porArchivo[rel]++
			}
		}
		return sc.Err()
	})
	if err != nil {
		t.Fatalf("recorriendo %s: %v", raiz, err)
	}
	return porArchivo
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

// Un descarte NUEVO agrega una firma que promete aislamiento sin darlo. El guard falla y
// el mensaje explica la alternativa, porque un guard que solo dice "no permitido" invita
// a agregar la línea al baseline sin pensarlo.
func TestScopeDiscard_NingunArchivoSupereSuBaseline(t *testing.T) {
	actual := contarDescartesPorArchivo(t, "../..")
	baseline := cargarBaselineDescartes(t)

	var nuevos []string
	for archivo, n := range actual {
		if n > baseline[archivo] {
			nuevos = append(nuevos, fmt.Sprintf("  %s: %d descarte(s), baseline %d", archivo, n, baseline[archivo]))
		}
	}
	if len(nuevos) == 0 {
		return
	}
	sort.Strings(nuevos)
	t.Errorf(`descarte de parámetro de scope NUEVO en %d archivo(s):
%s

Un `+"`_ = orgID`"+` hace que la firma prometa un aislamiento que no existe: quien la lee
—o quien le pasa el valor— asume que scopea. Es lo que dejó pasar el leak cross-tenant
de DOMAINSERV-182, donde el handler pasaba un orgID que el service ignoraba.

La plataforma es SINGLE-TENANT (migraciones 000142 y 000143). Si la función no va a
scopear por org, la alternativa correcta es NO ACEPTAR el parámetro, no aceptarlo y
descartarlo. Si el scope que corresponde es por proyecto, el parámetro es un project_id
y se usa en el WHERE.

Sumar la línea al baseline es válido solo si el descarte es deliberado y está explicado
en el código.`, len(nuevos), strings.Join(nuevos, "\n"))
}

// El baseline tiene que poder APRETARSE: si alguien limpia descartes y el baseline queda
// alto, el guard deja de vigilar esa diferencia en silencio. No es un error —bajar la
// deuda siempre está permitido— así que avisa sin fallar.
func TestScopeDiscard_BaselineNoQuedaMasFlojoQueLaRealidad(t *testing.T) {
	actual := contarDescartesPorArchivo(t, "../..")
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

// El guard tiene que estar mirando algo: si el regex deja de matchear —por un rename de
// los parámetros, por ejemplo— los dos tests de arriba pasarían vacíos y nadie se
// enteraría de que la vigilancia se apagó.
func TestScopeDiscard_ElGuardNoEstaMirandoUnConjuntoVacio(t *testing.T) {
	actual := contarDescartesPorArchivo(t, "../..")
	total := 0
	for _, n := range actual {
		total += n
	}
	if total == 0 {
		t.Fatal("cero descartes de scope detectados en todo el repo: o se limpiaron todos " +
			"(y hay que borrar este guard con su baseline) o el regex dejó de matchear")
	}
}
