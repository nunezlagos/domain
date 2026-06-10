// Package promptrouter — single-shot entry point del flow Domain.
//
// `domain_prompt(raw_text)` es la única tool que el agente IA (Claude
// Code / OpenCode) necesita conocer. El router:
//
//  1. Clasifica intent (chat | idea | feature | fix | hotfix | refactor | doc | rfc)
//  2. Si chat/idea: responde directamente, NO entra al SDD.
//  3. Si fix/feature/etc.: crea intake_payload + arranca el wizard
//     interactivo issue-04.7 con el mode correspondiente, devuelve la
//     primera pregunta al cliente.
//
// El cliente sigue con domain_hu_create_answer / domain_intake_* etc.
// Este router es el "decisor" inicial — concentra la lógica de routing
// en un lugar para que el agente IA no tenga que conocer 20 MCP tools.
package promptrouter

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/issuebuilder"
	"nunezlagos/domain/internal/service/intake"
)

// Intent es el resultado de la clasificación inicial.
type Intent string

const (
	IntentChat     Intent = "chat"
	IntentIdea     Intent = "idea"
	IntentFeature  Intent = "feature"
	IntentFix      Intent = "fix"
	IntentHotfix   Intent = "hotfix"
	IntentRefactor Intent = "refactor"
	IntentDoc      Intent = "doc"
	IntentRFC      Intent = "rfc"
)

// Outcome describe qué hizo el router con el prompt.
type Outcome string

const (
	// OutcomeChat: el router NO disparó SDD; la respuesta es Reply directo.
	OutcomeChat Outcome = "chat"
	// OutcomeWizardStarted: el wizard arrancó, hay DraftID + NextQuestion.
	OutcomeWizardStarted Outcome = "wizard_started"
	// OutcomeIntakeOnly: el intake_payload se persistió pero el wizard no
	// arrancó (clasificador con confidence baja → review manual).
	OutcomeIntakeOnly Outcome = "intake_only"
)

// Response devuelta por Route.
type Response struct {
	Outcome      Outcome             `json:"outcome"`
	Intent       Intent              `json:"intent"`
	Confidence   float64             `json:"confidence"`
	IntakeID     *uuid.UUID          `json:"intake_id,omitempty"`
	DraftID      *uuid.UUID          `json:"draft_id,omitempty"`
	NextQuestion *issuebuilder.Question `json:"next_question,omitempty"`
	Reply        string              `json:"reply,omitempty"`
	Reasoning    string              `json:"reasoning,omitempty"`
}

// Classifier es la interfaz para clasificación. Inyectable para tests +
// para swap entre LLM real (Anthropic/OpenAI) y stub heurístico.
type Classifier interface {
	Classify(ctx context.Context, rawText string) (Intent, float64, string, error)
}

// Router orquesta el flow.
type Router struct {
	IntakeService    *intake.Service
	IssueBuilderService *issuebuilder.Service
	Classifier       Classifier

	// MinConfidenceForWizard: si la confianza del classifier es menor,
	// el router devuelve OutcomeIntakeOnly (sin arrancar wizard) para
	// que un humano revise. Default 0.5.
	MinConfidenceForWizard float64

	// ChatReplyTemplate: si Intent=chat, el router responde con este
	// template. Si vacío, devuelve un default.
	ChatReplyTemplate string
}

var (
	ErrEmptyPrompt = errors.New("raw_text required")
)

// Route es el entry point: prompt → outcome.
func (r *Router) Route(ctx context.Context, rawText string, createdBy *uuid.UUID) (*Response, error) {
	if strings.TrimSpace(rawText) == "" {
		return nil, ErrEmptyPrompt
	}

	intent := IntentChat
	conf := 1.0
	reasoning := "default chat"
	if r.Classifier != nil {
		var err error
		intent, conf, reasoning, err = r.Classifier.Classify(ctx, rawText)
		if err != nil {
			return nil, fmt.Errorf("classify: %w", err)
		}
	} else {
		intent, conf, reasoning = heuristicClassify(rawText)
	}

	// Chat/idea: NO arranca SDD. Responde directo.
	if intent == IntentChat || intent == IntentIdea {
		reply := r.ChatReplyTemplate
		if reply == "" {
			reply = defaultChatReply(intent, rawText)
		}
		return &Response{
			Outcome:    OutcomeChat,
			Intent:     intent,
			Confidence: conf,
			Reply:      reply,
			Reasoning:  reasoning,
		}, nil
	}

	// Persistir intake_payload para audit incluso si arrancamos wizard.
	intakeP, err := r.IntakeService.Submit(ctx, intake.SubmitInput{
		Source:      intake.SourceAgent,
		RawText:     rawText,
		SubmittedBy: actorRef(createdBy),
	})
	if err != nil {
		return nil, fmt.Errorf("intake submit: %w", err)
	}
	// Update classification en el intake.
	_, _ = r.IntakeService.UpdateClassification(ctx, intakeP.ID,
		string(intent), severityFromIntent(intent), conf, reasoning)

	minConf := r.MinConfidenceForWizard
	if minConf <= 0 {
		minConf = 0.5
	}

	// Confianza baja: persistimos intake pero NO arrancamos wizard.
	if conf < minConf {
		return &Response{
			Outcome:    OutcomeIntakeOnly,
			Intent:     intent,
			Confidence: conf,
			IntakeID:   &intakeP.ID,
			Reasoning:  reasoning + " (confidence < " + fmtFloat(minConf) + ")",
		}, nil
	}

	// Mapeo Intent → wizard mode.
	mode := wizardModeForIntent(intent)
	draft, q, err := r.IssueBuilderService.Start(ctx, mode, rawText, createdBy)
	if err != nil {
		return nil, fmt.Errorf("wizard start: %w", err)
	}

	return &Response{
		Outcome:      OutcomeWizardStarted,
		Intent:       intent,
		Confidence:   conf,
		IntakeID:     &intakeP.ID,
		DraftID:      &draft.ID,
		NextQuestion: q,
		Reasoning:    reasoning,
	}, nil
}

func actorRef(u *uuid.UUID) string {
	if u == nil {
		return ""
	}
	return u.String()
}

func severityFromIntent(in Intent) string {
	switch in {
	case IntentHotfix:
		return "critical"
	case IntentFix:
		return "high"
	case IntentFeature, IntentRefactor:
		return "medium"
	case IntentDoc, IntentRFC:
		return "low"
	}
	return "medium"
}

func wizardModeForIntent(in Intent) string {
	switch in {
	case IntentFix, IntentHotfix:
		return issuebuilder.ModeBugFix
	case IntentRefactor:
		return issuebuilder.ModeRefactor
	case IntentDoc:
		return issuebuilder.ModeDoc
	case IntentRFC:
		return issuebuilder.ModeRFC
	}
	return issuebuilder.ModeFeature
}

func defaultChatReply(intent Intent, rawText string) string {
	if intent == IntentIdea {
		return "Anoté la idea. Si querés convertirla en feature concreto, pasame el alcance y arrancamos el wizard SDD."
	}
	return "Recibido. Si necesitás que esto se materialice en una HU, decime y arranco el wizard."
}

func fmtFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

// HeuristicClassifier — classifier sin LLM, basado en patterns de keywords.
// Útil para boot sin Anthropic key + para tests determinísticos.
type HeuristicClassifier struct{}

// Classify implements Classifier.
func (HeuristicClassifier) Classify(_ context.Context, rawText string) (Intent, float64, string, error) {
	intent, conf, reason := heuristicClassify(rawText)
	return intent, conf, reason, nil
}

func heuristicClassify(rawText string) (Intent, float64, string) {
	t := strings.ToLower(rawText)
	// Patterns ordenados por especificidad.
	switch {
	case containsAny(t, "urgente", "production caída", "prod down", "p0", "p1", "critical bug"):
		return IntentHotfix, 0.85, "keywords de urgencia detectadas"
	case containsAny(t, "bug", "no funciona", "no anda", "rota", "roto", "falla", "error",
		"unexpected", "broken", "doesn't work"):
		return IntentFix, 0.75, "keywords de bug detectadas"
	case containsAny(t, "refactor", "reorganizar", "limpiar código", "extract", "rename"):
		return IntentRefactor, 0.7, "keywords de refactor detectadas"
	case containsAny(t, "doc", "readme", "documentación", "documentation", "explicar en"):
		return IntentDoc, 0.7, "keywords de documentación detectadas"
	case containsAny(t, "rfc", "diseño arquitectura", "architecture decision", "tradeoffs"):
		return IntentRFC, 0.7, "keywords de RFC detectadas"
	case containsAny(t, "quiero", "necesito", "feature", "implementar", "agregar capacidad",
		"add the ability", "support", "i need", "i want", "puedo tener", "se podrá"):
		return IntentFeature, 0.7, "keywords de feature request detectadas"
	case containsAny(t, "?", "cómo", "como hago", "how do", "what is", "qué es", "puedes",
		"can you tell"):
		return IntentChat, 0.65, "pregunta directa / chat"
	case containsAny(t, "y si", "se me ocurre", "what if", "wondering", "idea:", "qué tal si"):
		return IntentIdea, 0.65, "exploración de idea sin compromiso"
	}
	return IntentChat, 0.4, "no se detectaron patterns claros — default chat"
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}
