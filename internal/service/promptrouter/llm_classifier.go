package promptrouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"nunezlagos/domain/internal/llm"
)

// LLMClassifier usa un Provider LLM real para clasificar intent. Más preciso
// que el HeuristicClassifier pero requiere API key + tiene costo.
//
// Si Provider es nil o devuelve error, Fallback se usa (default
// HeuristicClassifier para que el router siga funcionando offline).
type LLMClassifier struct {
	Provider llm.Provider
	Model    string  // "claude-haiku-4-5-20251001" recomendado (rápido + cheap)
	Fallback Classifier
	// MaxTokens default 256 — suficiente para JSON corto.
	MaxTokens int
	// Temperature default 0.0 — clasificación, no creatividad.
	Temperature float64
}

const llmClassifierSystemPrompt = `Sos un clasificador de prompts. Tu trabajo:
dado un texto que un usuario escribió en su agente IA, identificás cuál de
estas categorías describe MEJOR su intención:

  - chat:     pregunta directa o conversación informal ("cómo se configura X?")
  - idea:     exploración sin compromiso ("y si agregamos Y?", "se me ocurre Z")
  - feature:  pide implementar una nueva capacidad ("quiero exportar a CSV")
  - fix:      reporta un bug funcional ("no funciona el botón X", "falla al hacer Y")
  - hotfix:   bug urgente / producción ("URGENTE", "prod down", "critical")
  - refactor: mejora interna sin cambio funcional ("limpiar código", "extract Z")
  - doc:      cambio en documentación ("actualizar README", "agregar ejemplo")
  - rfc:      decisión arquitectónica ("diseño de", "tradeoffs entre", "RFC")

IMPORTANTE: respondés ÚNICAMENTE con JSON estricto sin texto adicional, sin
fences markdown, sin comentarios. Schema:

  {"intent":"<category>","confidence":<0.0-1.0>,"reasoning":"<breve, 1-2 frases>"}

Si tenés dudas entre dos categorías, elegí la que mejor matchee la INTENCIÓN
del usuario (no las palabras literales). Si el prompt es ambiguo, devolvé
confidence < 0.6 y reasoning explicando el conflicto.`

// Classify implements Classifier.
func (c *LLMClassifier) Classify(ctx context.Context, rawText string) (Intent, float64, string, error) {
	if c.Provider == nil {
		if c.Fallback != nil {
			return c.Fallback.Classify(ctx, rawText)
		}
		return IntentChat, 0.0, "no provider + no fallback", errors.New("llm classifier not configured")
	}

	model := c.Model
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	maxTok := c.MaxTokens
	if maxTok == 0 {
		maxTok = 256
	}

	resp, err := c.Provider.Complete(ctx, llm.CompletionOptions{
		Model:        model,
		Temperature:  c.Temperature,
		MaxTokens:    maxTok,
		SystemPrompt: llmClassifierSystemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: rawText},
		},
	})
	if err != nil {
		return c.fallbackOrFail(ctx, rawText, err)
	}

	parsed, parseErr := parseClassifierResponse(resp.Content)
	if parseErr != nil {
		return c.fallbackOrFail(ctx, rawText, parseErr)
	}
	return parsed.intent, parsed.confidence, parsed.reasoning, nil
}

func (c *LLMClassifier) fallbackOrFail(ctx context.Context, rawText string, originalErr error) (Intent, float64, string, error) {
	if c.Fallback != nil {
		i, conf, reason, err := c.Fallback.Classify(ctx, rawText)
		if err == nil {
			return i, conf, reason + " (fallback after LLM error: " + originalErr.Error() + ")", nil
		}
	}
	return IntentChat, 0.0, "llm error: " + originalErr.Error(), originalErr
}

type classifierParsed struct {
	intent     Intent
	confidence float64
	reasoning  string
}

func parseClassifierResponse(raw string) (*classifierParsed, error) {
	// Algunos providers envuelven JSON en fences ```json...```.
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var shape struct {
		Intent     string  `json:"intent"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(cleaned), &shape); err != nil {
		return nil, fmt.Errorf("parse llm json: %w (raw=%s)", err, raw)
	}

	if !validIntent(shape.Intent) {
		return nil, fmt.Errorf("invalid intent from LLM: %q", shape.Intent)
	}
	conf := shape.Confidence
	if conf < 0 {
		conf = 0
	}
	if conf > 1 {
		conf = 1
	}
	return &classifierParsed{
		intent:     Intent(shape.Intent),
		confidence: conf,
		reasoning:  shape.Reasoning,
	}, nil
}

func validIntent(s string) bool {
	switch Intent(s) {
	case IntentChat, IntentIdea, IntentFeature, IntentFix, IntentHotfix,
		IntentRefactor, IntentDoc, IntentRFC:
		return true
	}
	return false
}
