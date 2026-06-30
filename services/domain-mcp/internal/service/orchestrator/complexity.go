package orchestrator

import (
	"strings"
	"unicode/utf8"
)

// Complexity categorizes the estimated effort of a raw_text request.
type Complexity string

const (
	ComplexityTrivial  Complexity = "trivial"  // single small edit → ModeSolo (if LLM available)
	ComplexitySimple   Complexity = "simple"   // focused fix → ModeExpress
	ComplexityModerate Complexity = "moderate" // contained change → ModeLite
	ComplexityComplex  Complexity = "complex"  // broad/multi-file change → ModeFull
)

// ComplexitySignal is the result of heuristic analysis over raw_text.
// No LLM call — zero latency, zero tokens, deterministic.
type ComplexitySignal struct {
	Level        Complexity
	MultiConcern bool   // request contains multiple independent intents
	Scope        string // "single_file" | "multi_file" | "unknown"
	Confidence   float64
}

// analyzeComplexity estimates the complexity of a request from raw_text.
// Uses lexical heuristics only — no network, no LLM, safe to call inline.
func analyzeComplexity(rawText string) ComplexitySignal {
	text := strings.TrimSpace(rawText)
	lower := strings.ToLower(text)
	chars := utf8.RuneCountInString(text)

	sig := ComplexitySignal{Confidence: 0.7}

	if hasSingleFileIndicators(lower) {
		sig.Scope = "single_file"
	} else if hasMultiFileIndicators(lower) {
		sig.Scope = "multi_file"
	} else {
		sig.Scope = "unknown"
	}

	sig.MultiConcern = detectsMultiConcern(lower)

	switch {
	case sig.MultiConcern || isComplexIntent(lower) || sig.Scope == "multi_file" || chars > 500:
		sig.Level = ComplexityComplex
		sig.Confidence = 0.8
	case isTrivialIntent(lower) && chars < 180 && sig.Scope != "multi_file":
		sig.Level = ComplexityTrivial
		sig.Confidence = 0.9
	case isSimpleIntent(lower) && chars < 350:
		sig.Level = ComplexitySimple
		sig.Confidence = 0.8
	default:
		sig.Level = ComplexityModerate
		sig.Confidence = 0.65
	}

	return sig
}

func isTrivialIntent(lower string) bool {
	for _, p := range []string{
		"fix typo", "corregir typo", "typo en",
		"rename ", "renombrar ",
		"add comment", "agregar comentario",
		"update constant", "actualizar constante",
		"change variable name", "cambiar nombre de variable",
		"remove unused import", "eliminar import no usado",
		"fix whitespace", "formatear espacios",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func isSimpleIntent(lower string) bool {
	for _, p := range []string{
		"fix bug in", "arreglar bug en", "fix the bug",
		"add parameter", "agregar parámetro", "agregar parametro",
		"update validation", "actualizar validación", "actualizar validacion",
		"remove field", "eliminar campo",
		"add field", "agregar campo",
		"change default", "cambiar default",
		"fix error handling", "arreglar manejo de error",
		"add missing", "agregar faltante",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func isComplexIntent(lower string) bool {
	for _, p := range []string{
		"implement ", "implementar ",
		"create service", "crear servicio",
		"add feature", "agregar funcionalidad", "nueva funcionalidad",
		"refactor ", "refactorizar ",
		"migrate ", "migrar ",
		"redesign", "rediseñar", "rediseniar",
		"new module", "nuevo módulo", "nuevo modulo",
		"integrate ", "integrar ",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func hasSingleFileIndicators(lower string) bool {
	for _, p := range []string{
		"in file ", "en el archivo ", "en archivo ",
		"in the file", "en el fichero",
		".go ", ".py ", ".ts ", ".js ", ".sql ",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func hasMultiFileIndicators(lower string) bool {
	for _, p := range []string{
		"all files", "todos los archivos",
		"across the ", "en todo el ",
		"entire codebase", "todo el proyecto",
		"multiple files", "varios archivos",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func detectsMultiConcern(lower string) bool {
	for _, p := range []string{
		" y también ", " y tambien ",
		" and also ", " además ", " ademas ",
		" furthermore ", " moreover ",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	// Multiple bullet items suggest independent intents
	return strings.Count(lower, "\n-") >= 2 || strings.Count(lower, "\n*") >= 2
}
