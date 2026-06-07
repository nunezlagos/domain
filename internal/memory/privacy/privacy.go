// Package privacy — HU-03.6 stripping de contenido marcado como privado.
//
// Convención: cualquier bloque <private>...</private> en content NO se persiste.
// Útil para que clientes/agentes IA marquen secretos antes de guardar.
//
// El regex captura bloques cross-line con flag (?s).
package privacy

import (
	"regexp"
	"strings"
)

var privateBlockRe = regexp.MustCompile(`(?s)<private>.*?</private>`)

// Strip remove todos los bloques <private>...</private>.
// Retorna (clean, redactedCount): cuántos bloques se removieron.
func Strip(content string) (string, int) {
	matches := privateBlockRe.FindAllString(content, -1)
	if len(matches) == 0 {
		return content, 0
	}
	cleaned := privateBlockRe.ReplaceAllString(content, "")
	return cleaned, len(matches)
}

// HasPrivateBlocks indica si el content contiene al menos un bloque <private>.
func HasPrivateBlocks(content string) bool {
	return privateBlockRe.MatchString(content)
}

// NormalizeWhitespace colapsa runs de whitespace en un solo espacio y trim.
// Útil junto con Strip para que tras eliminar bloques no queden espacios dobles.
func NormalizeWhitespace(content string) string {
	out := strings.Join(strings.Fields(content), " ")
	return out
}
