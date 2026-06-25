// Package tokens — issue-07.4 estimación local de tokens sin red.
//
// Implementación: aproximación basada en BPE-rule-of-thumb (4 chars/token
// para inglés/español, 3.5 para CJK). Suficiente para budget enforcement
// pre-flight (no requiere precisión exacta del tokenizer del provider).
//
// Para precisión exacta usar tiktoken (cl100k_base) — futura issue-06.6.
package tokens

import (
	"unicode"

	"nunezlagos/domain/internal/llm"
)

// Estimate aproxima el token count de un texto.
// Heurística: chars(latin)/4 + chars(cjk)/2 + chars(otros)/3.
// Sobreestima ligero (defensa para enforcement).
func Estimate(text string) int {
	if text == "" {
		return 0
	}
	var latin, cjk, other int
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r), unicode.Is(unicode.Hiragana, r),
			unicode.Is(unicode.Katakana, r), unicode.Is(unicode.Hangul, r):
			cjk++
		case r > 127:
			other++
		default:
			latin++
		}
	}

	t := (latin+3)/4 + (cjk+1)/2 + (other+2)/3
	if t < 1 {
		t = 1
	}
	return t
}

// EstimateMessages suma el estimate de todos los messages + system_prompt.
// Agrega +10 tokens por message (overhead de role tag aproximado).
func EstimateMessages(systemPrompt string, messages []llm.Message) int {
	total := Estimate(systemPrompt)
	for _, m := range messages {
		total += Estimate(m.Content) + 10
		for _, tc := range m.ToolCalls {
			total += Estimate(tc.Name) + 20 // tool call serialization overhead
		}
	}
	return total
}
