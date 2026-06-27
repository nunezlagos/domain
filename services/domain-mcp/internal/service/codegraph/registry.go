// Package codegraph — capa de extensibilidad multi-lenguaje del grafo de código.
//
// Este archivo (registry.go) introduce el punto de extensión: una interface
// LanguageParser y un registry por extensión de archivo. El parser Go histórico
// (go/ast, parser.go) se expone como goParser registrado para ".go". La capa de
// servicio (service.go) despacha por extensión vía este registry en lugar de
// hardcodear ".go" + ParseFile.
//
// Los parsers no-Go (Python/PHP/JS/TS/TSX) viven detrás del build-tag
// 'treesitter' (register_treesitter.go) porque su único backend con cobertura de
// gramáticas es tree-sitter vía CGO, lo que rompería el cross-compile
// CGO_ENABLED=0 del release. Por DEFECTO (sin el tag) solo se registra Go y los
// demás lenguajes devuelven un error claro y accionable
// (register_default.go). Así el build por defecto NUNCA se rompe.
package codegraph

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// LanguageParser es la abstracción de un parser PURO de un lenguaje: entra
// (path, bytes) y sale un *ParsedFile determinista, sin tocar DB ni red. Cada
// impl declara las extensiones que maneja (sin punto inicial duplicado: ".py").
type LanguageParser interface {
	// Language devuelve el identificador del lenguaje del parser (ej. "go",
	// "python"). Se propaga al campo Language de los nodos.
	Language() string
	// Extensions devuelve las extensiones de archivo (con punto, en minúsculas)
	// que este parser maneja. Ej: []string{".ts", ".tsx"}.
	Extensions() []string
	// Parse produce el grafo intra-archivo de src. Debe ser determinista.
	Parse(filePath string, src []byte) (*ParsedFile, error)
}

// registry es el mapa extensión -> parser. Protegido por mutex porque los
// archivos init() de los distintos build-tags lo pueblan en el arranque.
var (
	registryMu sync.RWMutex
	registry   = map[string]LanguageParser{}
)

// RegisterParser registra un parser para todas sus extensiones. Si una extensión
// ya está registrada, la última gana (permite override por build-tag). Pensado
// para llamarse desde init(); es idempotente y thread-safe.
func RegisterParser(p LanguageParser) {
	registryMu.Lock()
	defer registryMu.Unlock()
	for _, ext := range p.Extensions() {
		registry[strings.ToLower(ext)] = p
	}
}

// parserForExt devuelve el parser registrado para una extensión (con punto), o
// (nil, false) si no hay ninguno.
func parserForExt(ext string) (LanguageParser, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := registry[strings.ToLower(ext)]
	return p, ok
}

// parserForPath resuelve el parser por la extensión del path.
func parserForPath(filePath string) (LanguageParser, bool) {
	return parserForExt(extOf(filePath))
}

// RegisteredExtensions devuelve las extensiones con parser registrado, ordenadas
// (determinismo). Útil para que el servicio sepa qué archivos son elegibles.
func RegisteredExtensions() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for ext := range registry {
		out = append(out, ext)
	}
	sort.Strings(out)
	return out
}

// extOf extrae la extensión (con punto, minúsculas) del último segmento del
// path. Ej: "a/b/foo.TS" => ".ts"; "Makefile" => "".
func extOf(filePath string) string {
	base := baseName(filePath)
	i := strings.LastIndex(base, ".")
	if i < 0 {
		return ""
	}
	return strings.ToLower(base[i:])
}

// goParser es el LanguageParser para Go. Delega en el parser histórico go/ast
// (ParseFile) para no duplicar lógica y mantener el comportamiento idéntico.
type goParser struct{}

func (goParser) Language() string     { return "go" }
func (goParser) Extensions() []string { return []string{".go"} }
func (goParser) Parse(filePath string, src []byte) (*ParsedFile, error) {
	return ParseFile(filePath, src)
}

// init registra SIEMPRE el parser Go, en cualquier build. Los parsers no-Go se
// registran en register_default.go (stubs) o register_treesitter.go (reales)
// según el build-tag.
func init() {
	RegisterParser(goParser{})
}

// ErrLanguageNotSupported lo devuelve un parser stub cuando el lenguaje requiere
// el build con -tags treesitter. Mensaje accionable para el operador.
func errLanguageRequiresTag(language string) error {
	return fmt.Errorf("lenguaje %s requiere compilar con -tags treesitter (el build por defecto solo soporta Go para preservar CGO_ENABLED=0)", language)
}
