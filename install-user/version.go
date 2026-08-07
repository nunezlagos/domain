package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Version, Commit y BuildTime los inyecta el linker con -X (ver Makefile y
// release-installer.yml). Hasta issue-57.1 el binario no sabía qué versión era, así que no
// había con qué comparar y el usuario no podía enterarse de que su cliente quedó atrás.
//
// El default NO es "0.0.0" a propósito: cualquier default con forma de semver haría que el
// hook lo compare como si fuera real y le diga a quien compila local que está desactualizado.
var (
	Version   = versionDeDesarrollo
	Commit    = ""
	BuildTime = ""
)

const (
	versionDeDesarrollo = "dev"

	// versionesIncomparables no es -1: -1 significa "a es menor que b", y confundirlos haría
	// que un formato irreconocible se lea como "estás viejo".
	versionesIncomparables = -2

	// Hoy actualizar el cliente es re-correr el instalador, que clona y compila
	// (INSTALL.md:72). La fase 3 de REQ-57 lo reemplaza por un comando de un paso.
	comandoDeActualizacion = "./install-user/bootstrap.sh"
)

func VersionInfo() string {
	info := "domain-install " + Version
	if Commit != "" {
		info += " (" + Commit + ")"
	}
	if BuildTime != "" {
		info += " built " + BuildTime
	}
	return info
}

// EsVersionDeRelease distingue un número publicado de cualquier otra cosa: el default de
// desarrollo, un string vacío o un tag con formato que no sabemos ordenar.
func EsVersionDeRelease(v string) bool {
	_, ok := partesDeVersion(v)
	return ok
}

// CompararVersiones devuelve -1, 0 o 1, y versionesIncomparables cuando alguna de las dos no
// se puede ordenar. Comparar por segmento numérico y no como strings es lo que hace que
// 1.10.0 sea mayor que 1.9.0.
func CompararVersiones(a, b string) int {
	pa, okA := partesDeVersion(a)
	pb, okB := partesDeVersion(b)
	if !okA || !okB {
		return versionesIncomparables
	}
	for i := 0; i < len(pa) || i < len(pb); i++ {
		va, vb := segmento(pa, i), segmento(pb, i)
		if va != vb {
			if va < vb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// AvisoDeActualizacion devuelve la línea para el bloque de inicio, o "" si no hay nada que
// decir. Devolver "" ante cualquier duda es deliberado: el aviso vive en el arranque de cada
// sesión, así que un falso positivo se paga en todas y entrena a ignorarlo.
func AvisoDeActualizacion(cliente, server, minimoSoportado string) string {
	if !EsVersionDeRelease(cliente) || !EsVersionDeRelease(server) {
		return ""
	}
	if CompararVersiones(cliente, server) >= 0 {
		return ""
	}
	if EsVersionDeRelease(minimoSoportado) && CompararVersiones(cliente, minimoSoportado) < 0 {
		return fmt.Sprintf(
			"⚠ domain: tu cliente %s YA NO ESTA SOPORTADO (el server exige %s o mayor, y corre %s). "+
				"Actualizá con: %s",
			cliente, minimoSoportado, server, comandoDeActualizacion)
	}
	return fmt.Sprintf(
		"· domain: hay una version nueva del cliente (%s → %s). Actualizá cuando te quede comodo con: %s",
		cliente, server, comandoDeActualizacion)
}

// imprimirAvisoDeActualizacion es lo que el hook SessionStart invoca. No devuelve error ni
// sale con código distinto de cero en NINGÚN caso: corre en el arranque de cada sesión, y un
// aviso que puede romperla vale menos que no avisar (REQ-3).
func imprimirAvisoDeActualizacion(versionDelServer, minimoSoportado string) {
	if aviso := AvisoDeActualizacion(Version, versionDelServer, minimoSoportado); aviso != "" {
		fmt.Println(aviso)
	}
}

// partesDeVersion acepta "1.2.3" y "v1.2.3" y rechaza todo lo demás, incluidos los sufijos
// de pre-release y build metadata: ordenarlos bien exige las reglas completas de semver, y
// una versión que no sabemos ordenar tiene que decir "no sé" en vez de arriesgar un orden.
func partesDeVersion(v string) ([]int, bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return nil, false
	}
	campos := strings.Split(v, ".")
	partes := make([]int, 0, len(campos))
	for _, c := range campos {
		n, err := strconv.Atoi(c)
		if err != nil || n < 0 {
			return nil, false
		}
		partes = append(partes, n)
	}
	return partes, true
}

// segmento trata los segmentos faltantes como cero, para que "1.2" y "1.2.0" sean la misma
// versión y no dos que se avisan mutuamente para siempre.
func segmento(partes []int, i int) int {
	if i < len(partes) {
		return partes[i]
	}
	return 0
}
