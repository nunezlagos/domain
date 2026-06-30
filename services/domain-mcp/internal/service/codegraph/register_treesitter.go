//go:build treesitter

// register_treesitter.go — registra los parsers tree-sitter reales (Python, PHP,
// JavaScript, TypeScript/TSX) cuando se compila con -tags treesitter. Override de
// los stubs de register_default.go (mismas extensiones).
//
// Las gramáticas vienen de github.com/alexaandru/go-sitter-forest/<lang>
// (cada una embebe su .scm y el parser C compilado vía CGO sobre
// go-tree-sitter-bare). Por eso este archivo y treesitter.go solo se compilan con
// el tag: el binario por defecto NO arrastra CGO.
package codegraph

import (
	sitter "github.com/alexaandru/go-tree-sitter-bare"

	gjs "github.com/alexaandru/go-sitter-forest/javascript"
	gphp "github.com/alexaandru/go-sitter-forest/php"
	gpy "github.com/alexaandru/go-sitter-forest/python"
	gtsx "github.com/alexaandru/go-sitter-forest/tsx"
	gts "github.com/alexaandru/go-sitter-forest/typescript"
)

func init() {
	for _, spec := range treesitterSpecs() {
		RegisterParser(tsParser{spec: spec})
	}
}

// treesitterSpecs declara la config tree-sitter de cada lenguaje soportado. Los
// node-types provienen de inspeccionar el árbol de cada gramática.
func treesitterSpecs() []langSpec {
	jsLikeIdents := set("identifier", "property_identifier", "type_identifier", "shorthand_property_identifier")
	jsCalls := set("call_expression", "new_expression")

	return []langSpec{
		{
			language:     "python",
			extensions:   []string{".py"},
			langFn:       func() *sitter.Language { return sitter.NewLanguage(gpy.GetLanguage()) },
			funcDecl:     set("function_definition"),
			methodDecl:   set("function_definition"),
			classDecl:    set("class_definition"),
			classBody:    set("block"),
			importDecl:   set("import_statement", "import_from_statement"),
			callExpr:     set("call"),
			nameFields:   []string{"name"},
			identTypes:   set("identifier", "dotted_name"),
			calleeFields: []string{"function"},
		},
		{
			language:     "php",
			extensions:   []string{".php"},
			langFn:       func() *sitter.Language { return sitter.NewLanguage(gphp.GetLanguage()) },
			funcDecl:     set("function_definition"),
			methodDecl:   set("method_declaration"),
			classDecl:    set("class_declaration"),
			ifaceDecl:    set("interface_declaration"),
			classBody:    set("declaration_list"),
			importDecl:   set("namespace_use_declaration"),
			callExpr:     set("function_call_expression", "member_call_expression", "scoped_call_expression", "object_creation_expression"),
			nameFields:   []string{"name"},
			identTypes:   set("name", "qualified_name"),
			calleeFields: []string{"function", "name"},
		},
		{
			language:     "javascript",
			extensions:   []string{".js", ".jsx", ".mjs", ".cjs"},
			langFn:       func() *sitter.Language { return sitter.NewLanguage(gjs.GetLanguage()) },
			funcDecl:     set("function_declaration", "generator_function_declaration"),
			methodDecl:   set("method_definition"),
			classDecl:    set("class_declaration"),
			classBody:    set("class_body"),
			importDecl:   set("import_statement"),
			callExpr:     jsCalls,
			nameFields:   []string{"name"},
			identTypes:   jsLikeIdents,
			calleeFields: []string{"function", "constructor"},
		},
		{
			language:     "typescript",
			extensions:   []string{".ts", ".mts", ".cts"},
			langFn:       func() *sitter.Language { return sitter.NewLanguage(gts.GetLanguage()) },
			funcDecl:     set("function_declaration", "generator_function_declaration"),
			methodDecl:   set("method_definition", "method_signature"),
			classDecl:    set("class_declaration"),
			ifaceDecl:    set("interface_declaration"),
			typeDecl:     set("type_alias_declaration"),
			classBody:    set("class_body"),
			importDecl:   set("import_statement"),
			callExpr:     jsCalls,
			nameFields:   []string{"name"},
			identTypes:   jsLikeIdents,
			calleeFields: []string{"function", "constructor"},
		},
		{
			language:     "tsx",
			extensions:   []string{".tsx"},
			langFn:       func() *sitter.Language { return sitter.NewLanguage(gtsx.GetLanguage()) },
			funcDecl:     set("function_declaration", "generator_function_declaration"),
			methodDecl:   set("method_definition", "method_signature"),
			classDecl:    set("class_declaration"),
			ifaceDecl:    set("interface_declaration"),
			typeDecl:     set("type_alias_declaration"),
			classBody:    set("class_body"),
			importDecl:   set("import_statement"),
			callExpr:     jsCalls,
			nameFields:   []string{"name"},
			identTypes:   jsLikeIdents,
			calleeFields: []string{"function", "constructor"},
		},
	}
}
