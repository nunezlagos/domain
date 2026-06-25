package wizardplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"nunezlagos/domain/internal/llm"
)

// Question es lo que el wizard devuelve al cliente.
type Question struct {
	SlotKey       string   `json:"slot_key"`
	Prompt        string   `json:"prompt"`
	ContextNote   string   `json:"context_note,omitempty"` // explica QUÉ encontramos antes de preguntar
	Options       []Option `json:"options,omitempty"`      // sugerencias derivadas del envelope
	AllowsFreeText bool    `json:"allows_free_text"`
}

// Option es una opción sugerida (no obligatoria de elegir).
type Option struct {
	Value       string `json:"value"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`     // de qué finding salió
	Confidence  float64 `json:"confidence,omitempty"`
}

// Planner decide qué slot pendiente preguntar próximo y formula la pregunta.
type Planner struct {

	QuestionFormulator QuestionFormulator

	SlotThreshold float64
}

// NoMoreQuestionsErr se devuelve por NextQuestion cuando todos los slots
// requeridos están conocidos (envelope listo para BuildPreview).
var NoMoreQuestionsErr = errors.New("no more questions; envelope complete")

// NextQuestion elige el slot pendiente de mayor prioridad y formula la
// pregunta. Si todos están conocidos, devuelve NoMoreQuestionsErr.
func (p *Planner) NextQuestion(ctx context.Context, env *ContextEnvelope) (*Question, error) {
	threshold := p.SlotThreshold
	if threshold <= 0 {
		threshold = 0.75
	}

	pending := env.PendingSlots(threshold)
	if len(pending) == 0 {
		return nil, NoMoreQuestionsErr
	}

	slot := prioritizeSlot(pending, env)
	contextNote := buildContextNote(env)

	prompt := ""
	options := suggestionsForSlot(slot, env)

	if p.QuestionFormulator != nil {
		q, err := p.QuestionFormulator.FormulateQuestion(ctx, FormulateInput{
			SlotKey:      slot,
			Envelope:     env,
			Suggestions:  options,
			ContextNote:  contextNote,
		})
		if err == nil && strings.TrimSpace(q) != "" {
			prompt = q
		}
	}
	if prompt == "" {
		prompt = templatePromptForSlot(slot, env)
	}

	return &Question{
		SlotKey:        slot,
		Prompt:         prompt,
		ContextNote:    contextNote,
		Options:        options,
		AllowsFreeText: allowsFreeText(slot),
	}, nil
}

// RecordAnswer marca el slot como provided por el usuario y lo persiste.
func (p *Planner) RecordAnswer(env *ContextEnvelope, slot string, value any) {
	env.Touch(slot, value, "user", 1.0, "respuesta directa del usuario")
}

// prioritizeSlot ordena los slots pendientes por importancia para el flow.
// Intent siempre primero; después depende del intent.
func prioritizeSlot(pending []string, env *ContextEnvelope) string {

	prio := map[string]int{
		SlotIntent:    0,
		SlotSeverity:  1, // crítico para bug-fix
		SlotComponent: 2,
		SlotActual:    3,
		SlotExpected:  4,
		SlotGoal:      5,
		SlotSummary:   6,
		SlotREQParent: 7,
		SlotAudience:  8,
		SlotSlug:      9, // último: derivable post-summary
	}
	best := pending[0]
	bestP := 1000
	for _, s := range pending {
		p := prio[s]
		if p < bestP {
			best = s
			bestP = p
		}
	}
	return best
}

// suggestionsForSlot construye opciones sugeridas según el envelope.
func suggestionsForSlot(slot string, env *ContextEnvelope) []Option {
	out := []Option{}
	switch slot {
	case SlotREQParent:
		if env.HUMatches != nil {
			seen := map[string]bool{}
			for _, c := range env.HUMatches.Candidates {

				if seen[c.Slug] {
					continue
				}
				seen[c.Slug] = true
				out = append(out, Option{
					Value: c.Slug, Label: c.Title,
					Description: fmt.Sprintf("similarity %.2f", c.Similarity),
					Source: "hu_dedup", Confidence: c.Similarity,
				})
				if len(out) >= 5 {
					break
				}
			}
		}
	case SlotSeverity:
		out = []Option{
			{Value: "critical", Label: "Critical", Description: "Producción caída, datos perdidos"},
			{Value: "high", Label: "High", Description: "Funcionalidad mayor rota sin workaround"},
			{Value: "medium", Label: "Medium", Description: "Funcionalidad menor rota con workaround"},
			{Value: "low", Label: "Low", Description: "Cosmético, edge case raro"},
		}
		if env.Intent != nil && env.Intent.Intent == "hotfix" {
			out[0].Confidence = 0.9 // critical pre-seleccionado
		}
	case SlotIntent:
		out = []Option{
			{Value: "feature", Label: "Feature nuevo"},
			{Value: "fix", Label: "Bug fix"},
			{Value: "hotfix", Label: "Hotfix urgente"},
			{Value: "refactor", Label: "Refactor"},
			{Value: "doc", Label: "Documentation"},
			{Value: "rfc", Label: "RFC arquitectura"},
		}
	case SlotAudience:
		out = []Option{
			{Value: "dx-engineer", Label: "DX engineer"},
			{Value: "platform-engineer", Label: "Platform engineer"},
			{Value: "org-owner", Label: "Org owner"},
			{Value: "org-member", Label: "Org member"},
			{Value: "auditor", Label: "Auditor"},
		}
	}
	return out
}

func allowsFreeText(slot string) bool {
	switch slot {
	case SlotGoal, SlotSummary, SlotExpected, SlotActual, SlotSlug, SlotComponent, SlotREQParent:
		return true
	}
	return false
}

// buildContextNote describe en una frase qué encontramos en el análisis.
func buildContextNote(env *ContextEnvelope) string {
	parts := []string{}
	if env.Intent != nil {
		parts = append(parts, fmt.Sprintf("intent=%s (conf %.2f)",
			env.Intent.Intent, env.Intent.Confidence))
	}
	if env.HUMatches != nil && len(env.HUMatches.Candidates) > 0 {
		top := env.HUMatches.Candidates[0]
		parts = append(parts, fmt.Sprintf("HU similar: %s (sim %.2f)",
			top.Slug, top.Similarity))
	}
	if env.Code != nil && len(env.Code.Hits) > 0 {
		parts = append(parts, fmt.Sprintf("%d hits en código (%s)",
			len(env.Code.Hits), env.Code.Hits[0].Path))
	}
	if env.Memory != nil && len(env.Memory.Matches) > 0 {
		parts = append(parts, fmt.Sprintf("%d memorias relacionadas",
			len(env.Memory.Matches)))
	}
	if env.History != nil && len(env.History.RelatedRuns) > 0 {
		parts = append(parts, fmt.Sprintf("%d agent runs recientes",
			len(env.History.RelatedRuns)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "[Análisis: " + strings.Join(parts, "; ") + "]"
}

// templatePromptForSlot fallback determinístico.
func templatePromptForSlot(slot string, env *ContextEnvelope) string {
	switch slot {
	case SlotIntent:
		return "¿Qué tipo de cambio describe esto?"
	case SlotSeverity:
		return "¿Cuán crítico es este bug?"
	case SlotComponent:
		return "¿Qué componente o sistema está afectado?"
	case SlotActual:
		return "¿Qué pasa actualmente? (describir el comportamiento defectuoso)"
	case SlotExpected:
		return "¿Qué debería pasar? (comportamiento esperado)"
	case SlotGoal:
		return "¿Qué se gana con este cambio? (1 línea)"
	case SlotSummary:
		return "Resumí lo que hace este cambio (2-3 líneas)"
	case SlotREQParent:
		return "¿Bajo qué REQ vive? (slug, ej REQ-04-opsx-sdd)"
	case SlotAudience:
		return "¿Quién es la audiencia principal?"
	case SlotSlug:
		return "Slug corto kebab-case (ej. csv-export-runs)"
	}
	return "Necesito más información para " + slot
}

// QuestionFormulator es la interfaz para generar preguntas con LLM.
type QuestionFormulator interface {
	FormulateQuestion(ctx context.Context, in FormulateInput) (string, error)
}

// FormulateInput input para el LLM formulator.
type FormulateInput struct {
	SlotKey     string
	Envelope    *ContextEnvelope
	Suggestions []Option
	ContextNote string
}

// LLMQuestionFormulator usa un Provider LLM para formular preguntas
// naturales incorporando el envelope.
type LLMQuestionFormulator struct {
	Provider llm.Provider
	Model    string






	PromptLoader func(ctx context.Context) (string, error)
}

// DefaultFormulatorSystemPrompt es el system prompt del wizard formulator por
// defecto. Se seedea en la tabla prompts con slug='wizard-formulator' para
// que sea editable desde el dashboard. El formulator lo usa como fallback si
// la DB no tiene el prompt o el loader no está cableado. Es solo el skeleton:
// el envelope runtime que se concatena/interpola se arma aparte y sigue
// siendo dinámico.
const DefaultFormulatorSystemPrompt = `Sos un wizard interactivo que ayuda a un usuario a especificar una HU técnica.

Recibís: (a) el slot que necesitás clarificar, (b) un envelope con análisis automático del prompt original (intent, HUs similares, hits en código, memorias, agent runs previos), (c) opciones sugeridas.

Tu trabajo: formular UNA pregunta natural, breve (1-2 frases), en español rioplatense, que:
1. Use el contexto del envelope ("encontré X..., ¿es eso?")
2. Sea específica al slot ("severidad", "componente afectado", "comportamiento esperado", etc.)
3. NO repita la pregunta tipo formulario ("¿Severity?") — formulá una pregunta que un humano haría en chat
4. Si hay options, mencionalas al final como "(opciones: ...)" si son ≤4

Output: SOLO la pregunta en texto plano, sin markdown, sin prefijos, sin comillas.`

func (f *LLMQuestionFormulator) FormulateQuestion(ctx context.Context, in FormulateInput) (string, error) {
	if f.Provider == nil {
		return "", errors.New("no provider")
	}
	model := f.Model
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}

	envSummary, _ := json.Marshal(map[string]any{
		"slot":         in.SlotKey,
		"intent":       in.Envelope.Intent,
		"hu_matches":   in.Envelope.HUMatches,
		"code_hits":    in.Envelope.Code,
		"memory":       in.Envelope.Memory,
		"history":      in.Envelope.History,
		"raw_prompt":   in.Envelope.RawPrompt,
		"suggestions":  in.Suggestions,
		"context_note": in.ContextNote,
	})

	systemPrompt := DefaultFormulatorSystemPrompt
	if f.PromptLoader != nil {
		if loaded, lerr := f.PromptLoader(ctx); lerr == nil && strings.TrimSpace(loaded) != "" {
			systemPrompt = loaded
		}
	}

	resp, err := f.Provider.Complete(ctx, llm.CompletionOptions{
		Model:        model,
		Temperature:  0.4,
		MaxTokens:    256,
		SystemPrompt: systemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: "Slot a clarificar + envelope:\n" + string(envSummary)},
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}
