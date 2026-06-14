package issuebuilder

import "fmt"

// rfcFlow define los steps del wizard para mode=rfc (ADR).
// RFCs capturan decisiones arquitectónicas: contexto, decisión, alternativas.
var rfcFlow = []step{
	{
		Key:    "adr_status",
		Prompt: "¿Estado del ADR?",
		options: []Option{
			{Value: "proposed", Label: "Propuesto", Description: "En discusión, busca feedback", Recommended: true},
			{Value: "decided", Label: "Decidido", Description: "Aceptado, se va a implementar"},
			{Value: "deprecated", Label: "Deprecado", Description: "Reemplazado por otro ADR"},
			{Value: "superseded", Label: "Superseded", Description: "Una decisión posterior lo invalida"},
		},
		Validate: requireOneOf("proposed", "decided", "deprecated", "superseded"),
	},
	{
		Key:    "context",
		Prompt: "Contexto del problema — ¿qué forces están en juego y por qué se necesita esta decisión?",
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "decision",
		Prompt: "Decisión tomada — ¿qué se elige y qué NO se elige explícitamente?",
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "consequences",
		Prompt: "Consecuencias positivas, negativas y neutrales de esta decisión",
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "alternatives",
		Prompt: "Alternativas consideradas (2-3) y por qué se descartaron",
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "related",
		Prompt: "RFCs/ADRs relacionados (IDs o slugs separados por coma, o vacío si ninguno)",
		Validate: func(a any) error {
			_, ok := a.(string)
			if !ok {
				return fmt.Errorf("string required")
			}
			return nil
		},
	},
	{
		Key:    "slug",
		Prompt: "Slug corto kebab-case para esta HU (sin prefijo HU-XX.Y)",
		Validate: slugValidator,
	},
	{
		Key:    "summary",
		Prompt: "Resumen ejecutivo del RFC (2-3 líneas) — decisión clave y rationale",
		Validate: requireNonEmptyString(),
	},
}
