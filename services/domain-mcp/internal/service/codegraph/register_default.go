//go:build !treesitter

// register_default.go — registro de lenguajes en el build POR DEFECTO (sin el
// tag 'treesitter'). Solo Go tiene parser real (registrado en registry.go). Los
// lenguajes no-Go se registran como stubs que devuelven un error claro y
// accionable: requieren compilar con -tags treesitter.
//
// Por qué stubs (y no simplemente "no registrar"): así el servicio distingue
// "extensión desconocida que se ignora" de "lenguaje soportado pero no compilado
// en este binario", dando un mensaje útil al operador en vez de un silencio.
//
// Este archivo NO importa tree-sitter ni nada CGO: el binario por defecto sigue
// siendo CGO_ENABLED=0 y cross-compilable.
package codegraph

// stubParser declara las extensiones de un lenguaje no-Go pero falla al parsear,
// indicando que hay que recompilar con -tags treesitter.
type stubParser struct {
	language string
	exts     []string
}

func (s stubParser) Language() string     { return s.language }
func (s stubParser) Extensions() []string { return s.exts }
func (s stubParser) Parse(string, []byte) (*ParsedFile, error) {
	return nil, errLanguageRequiresTag(s.language)
}

// init registra los stubs de los lenguajes soportados-bajo-tag. Las mismas
// extensiones las override register_treesitter.go cuando el tag está activo.
func init() {
	for _, s := range []stubParser{
		{language: "python", exts: []string{".py"}},
		{language: "php", exts: []string{".php"}},
		{language: "javascript", exts: []string{".js", ".jsx", ".mjs", ".cjs"}},
		{language: "typescript", exts: []string{".ts"}},
		{language: "tsx", exts: []string{".tsx"}},
	} {
		RegisterParser(s)
	}
}
