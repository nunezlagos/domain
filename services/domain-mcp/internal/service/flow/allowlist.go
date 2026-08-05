package flow

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

var (
	// ErrAllowlistSinPrefijo: un glob que arranca con wildcard no tiene scope
	// decidible. Ver ValidarAllowlist.
	ErrAllowlistSinPrefijo = errors.New("flow allowlist: un glob que arranca con wildcard no acota nada")
	// ErrAllowlistSolapada: dos sub-tareas reclaman el mismo territorio.
	ErrAllowlistSolapada = errors.New("flow allowlist: dos allowlists se solapan")
)

// ValidarAllowlist rechaza los globs sobre los que el gate NO puede decidir
// solapamiento, en vez de aceptarlos y descubrir el conflicto al editar.
//
// El criterio es fail-closed a propósito: se acepta solo la gramática que hace
// COMPLETA la comparación de scopes —un prefijo literal seguido de wildcards—, y
// se rechaza todo lo demás. La alternativa (aceptar cualquier glob y comparar lo
// que se pueda) da falsos negativos silenciosos: dos allowlists que sí se
// solapan pasarían el chequeo y dos agentes editarían el mismo archivo creyendo
// tener scopes disjuntos. Un guard que a veces no detecta lo que existe para
// detectar es el defecto que DOMAINSERV-218 vino a cerrar, no uno nuevo que
// introducir.
//
// Casos rechazados y por qué:
//   - "*", "**", "**/*.go": el prefijo literal es vacío, así que el scope es el
//     repo entero. Como allowlist de batch-mode no acota nada y hace que dos
//     sub-tareas cualesquiera se solapen — pedirlo es casi siempre un error de
//     orquestación, no una intención.
//   - "": no dice nada.
func ValidarAllowlist(globs []string) error {
	for _, g := range globs {
		if strings.TrimSpace(g) == "" {
			return fmt.Errorf("%w: glob vacío", ErrAllowlistSinPrefijo)
		}
		if scopeDe(g) == "" {
			return fmt.Errorf("%w: %q — usá un prefijo de directorio, ej. \"services/domain-mcp/**\"", ErrAllowlistSinPrefijo, g)
		}
	}
	return nil
}

// ValidarParticionDisjunta verifica que N allowlists no reclamen territorio
// común, para que el gate nunca tenga que elegir entre dos tokens que autorizan
// el mismo path. El ticket lo pide explícitamente: el solapamiento es un error de
// orquestación y se rechaza al EMITIR, no al editar.
//
// Devuelve el error apenas encuentra el primer par en conflicto, con los dos
// índices y el par de globs, porque quien orquesta necesita saber cuáles corregir.
func ValidarParticionDisjunta(allowlists [][]string) error {
	for i, a := range allowlists {
		if err := ValidarAllowlist(a); err != nil {
			return fmt.Errorf("allowlist #%d: %w", i, err)
		}
	}
	for i := range allowlists {
		for j := i + 1; j < len(allowlists); j++ {
			gi, gj, hay := primerSolape(allowlists[i], allowlists[j])
			if hay {
				return fmt.Errorf("%w: #%d %q y #%d %q reclaman el mismo territorio", ErrAllowlistSolapada, i, gi, j, gj)
			}
		}
	}
	return nil
}

// primerSolape devuelve el primer par de globs cuyos scopes se contienen.
func primerSolape(a, b []string) (string, string, bool) {
	for _, ga := range a {
		for _, gb := range b {
			if scopesSeContienen(scopeDe(ga), scopeDe(gb)) {
				return ga, gb, true
			}
		}
	}
	return "", "", false
}

// scopeDe devuelve el directorio literal que un glob acota: el tramo previo al
// primer wildcard, truncado al último '/'. Es lo que hace decidible la
// comparación, y por eso un glob sin prefijo literal no se acepta.
//
//	"services/domain-mcp/**"        → "services/domain-mcp"
//	"services/domain-mcp/foo.go"    → "services/domain-mcp"
//	"install-user/hooks/*.sh"       → "install-user/hooks"
//	"**/*.go"                       → "" (rechazado por ValidarAllowlist)
func scopeDe(glob string) string {
	corte := strings.IndexAny(glob, "*?[")
	literal := glob
	if corte >= 0 {
		literal = glob[:corte]
	}
	// un glob sin wildcard es un archivo puntual: su scope es su directorio
	dir := path.Dir(literal)
	if corte >= 0 {
		if i := strings.LastIndex(literal, "/"); i >= 0 {
			dir = literal[:i]
		} else {
			dir = ""
		}
	}
	dir = strings.Trim(path.Clean(dir), "/")
	if dir == "." {
		return ""
	}
	return dir
}

// scopesSeContienen: dos scopes chocan si son iguales o si uno es directorio
// ancestro del otro. Comparar por segmentos y no por prefijo de string evita el
// falso positivo clásico: "services/domain" NO es ancestro de
// "services/domain-mcp" aunque su string sí sea prefijo.
func scopesSeContienen(a, b string) bool {
	if a == b {
		return true
	}
	return esAncestro(a, b) || esAncestro(b, a)
}

func esAncestro(ancestro, desc string) bool {
	if ancestro == "" || desc == "" {
		return false
	}
	return strings.HasPrefix(desc, ancestro+"/")
}
