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
	orchsvc "nunezlagos/domain/internal/service/orchestrator"
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
	IntentAnalysis Intent = "analysis"
)

// Outcome describe qué hizo el router con el prompt.
type Outcome string

const (
	// OutcomeChat: el router NO disparó SDD; la respuesta es Reply directo.
	OutcomeChat Outcome = "chat"
	// OutcomeWizardStarted: el wizard arrancó, hay DraftID + NextQuestion.
	// Path legacy — sólo si Router.Orchestrator es nil.
	OutcomeWizardStarted Outcome = "wizard_started"
	// OutcomeIntakeOnly: el intake_payload se persistió pero el wizard no
	// arrancó (clasificador con confidence baja → review manual).
	OutcomeIntakeOnly Outcome = "intake_only"
	// OutcomeOrchestratorStarted: el orquestador SDD (issue-08.10) inició
	// un flow_run. Hay FlowRunID + OrchestratorRunID + SnapshotPrompt para
	// que el cliente IDE arranque la primera fase.
	OutcomeOrchestratorStarted Outcome = "orchestrator_started"
	// OutcomeAnalysis: el prompt se clasificó como analysis y se ejecutó
	// el mini-pipeline de análisis read-only. Hay KnowledgeDocID con el
	// documento generado.
	OutcomeAnalysis Outcome = "analysis"
)

// Response devuelta por Route.
type Response struct {
	Outcome      Outcome                `json:"outcome"`
	Intent       Intent                 `json:"intent"`
	Confidence   float64                `json:"confidence"`
	IntakeID     *uuid.UUID             `json:"intake_id,omitempty"`
	DraftID      *uuid.UUID             `json:"draft_id,omitempty"`
	NextQuestion *issuebuilder.Question `json:"next_question,omitempty"`
	Reply        string                 `json:"reply,omitempty"`
	Reasoning    string                 `json:"reasoning,omitempty"`
	// Campos del orquestador (issue-08.10): poblados cuando
	// Outcome=OutcomeOrchestratorStarted.
	FlowRunID         *uuid.UUID `json:"flow_run_id,omitempty"`
	OrchestratorRunID *uuid.UUID `json:"orchestrator_run_id,omitempty"`
	SnapshotPrompt    string     `json:"snapshot_prompt,omitempty"`
	Mode              string     `json:"mode,omitempty"`
	// KnowledgeDocID se popula cuando Outcome=OutcomeAnalysis: el ID del
	// knowledge_document generado por el mini-pipeline de análisis.
	KnowledgeDocID *uuid.UUID `json:"knowledge_doc_id,omitempty"`
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

	// Orchestrator opcional (issue-08.10). Si está inyectado, los intents
	// feat/fix/refactor/hotfix/rfc/doc invocan orchestratorSvc.Run en
	// lugar del wizard legacy (issuebuilder.Start). Esto es lo que
	// activa el pipeline SDD plug-and-play del RFC 0006.
	//
	// Backward compat: si nil, el comportamiento previo se preserva
	// (wizard arranca, devuelve NextQuestion).
	Orchestrator *orchsvc.Service

	// MinConfidenceForWizard: si la confianza del classifier es menor,
	// el router devuelve OutcomeIntakeOnly (sin arrancar wizard) para
	// que un humano revise. Default 0.5.
	MinConfidenceForWizard float64

	// ChatReplyTemplate: si Intent=chat, el router responde con este
	// template. Si vacío, devuelve un default.
	ChatReplyTemplate string

	// AnalysisService opcional (issue-08.10 ana-002). Si está inyectado,
	// los prompts clasificados como IntentAnalysis invocan el mini-pipeline
	// de análisis read-only que produce un knowledge_doc + observation.
	AnalysisService AnalysisRunner
}

// AnalysisRunner es la interfaz que el router necesita del analysis
// service. Permite testear sin acoplar al service concreto.
type AnalysisRunner interface {
	RunAnalysis(ctx context.Context, in AnalysisInput) (*AnalysisResult, error)
}

// AnalysisInput es lo que el router pasa al analysis service.
type AnalysisInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	RawText        string
}

// AnalysisResult es lo que el analysis service devuelve al router.
type AnalysisResult struct {
	KnowledgeDocID uuid.UUID
	Title          string
	Body           string
}

var (
	ErrEmptyPrompt                    = errors.New("raw_text required")
	ErrOrgIDRequiredForOrchestrator   = errors.New("orgID required when Router.Orchestrator is configured")
)

// Route es el entry point: prompt → outcome.
//
// orgID es opcional para el path legacy (wizard). Si Router.Orchestrator
// está configurado, orgID es OBLIGATORIO — sin org_id el orquestador no
// puede crear el flow_run. En ese caso Route devuelve error tipado
// ErrOrgIDRequiredForOrchestrator.
func (r *Router) Route(ctx context.Context, rawText string, createdBy *uuid.UUID, orgID *uuid.UUID) (*Response, error) {
	return r.RouteWithIntent(ctx, rawText, createdBy, orgID, nil, nil)
}

// RouteWithIntent es Route con clasificación híbrida: si intentOverride
// es un Intent válido del enum, se usa DIRECTO y se SALTEA la clasificación
// (el cliente —Claude Code vía MCP— ya clasificó usando el prompt 'triage').
// Si intentOverride es nil o inválido, clasifica como Route normal (LLM si
// hay Provider, else keyword heurístico).
//
// Este es el modelo SIN API keys de LLM en la plataforma: el LLM es el
// agente cliente, que trae su propio intent. El keyword fallback se
// mantiene para clientes que no clasifican.
func (r *Router) RouteWithIntent(ctx context.Context, rawText string, createdBy *uuid.UUID, orgID *uuid.UUID, projectID *uuid.UUID, intentOverride *Intent) (*Response, error) {
	if strings.TrimSpace(rawText) == "" {
		return nil, ErrEmptyPrompt
	}

	intent := IntentChat
	conf := 1.0
	reasoning := "default chat"
	if intentOverride != nil && validIntent(string(*intentOverride)) {
		intent = *intentOverride
		conf = 1.0
		reasoning = "intent override del cliente (clasificación via prompt 'triage')"
	} else if r.Classifier != nil {
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
		ProjectID:   projectID,
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

	// Path analysis: mini-pipeline read-only que produce knowledge_doc + observation.
	// NO pasa por el orquestador ni el wizard — es una operación de análisis puro
	// que lee información y la persiste como documento.
	if intent == IntentAnalysis {
		return r.handleAnalysis(ctx, rawText, createdBy, orgID, intent, conf, reasoning, intakeP.ID)
	}

	// Path orquestador (issue-08.10): si está configurado, los intents
	// SDD-capables invocan el orquestador en lugar del wizard legacy.
	if r.Orchestrator != nil {
		if orgID == nil {
			return nil, ErrOrgIDRequiredForOrchestrator
		}
		userID := uuid.Nil
		if createdBy != nil {
			userID = *createdBy
		}
		mode := orchestratorModeForIntent(intent)
		// OrchestrateInput.ProjectID es uuid.UUID (valor); projectID es *uuid.UUID.
		var projID uuid.UUID
		if projectID != nil {
			projID = *projectID
		}
		orchRes, err := r.Orchestrator.Run(ctx, orchsvc.OrchestrateInput{
			OrganizationID: *orgID,
			UserID:         userID,
			RawText:        rawText,
			Mode:           mode,
			// project_id: scopea el flow_run al proyecto. Sin esto el flow_run
			// quedaba con project_id NULL aunque el intake sí lo recibía
			// (gap detectado en el test E2E del flujo via domain_prompt).
			ProjectID: projID,
			// Hardspec obligatorio por diseño: el camino domain_prompt es el que
			// usa el agente; si no lo seteamos quedaba en false (zero value) y la
			// reiteración humana en sdd-spec no se exigía. domain_orchestrate ya
			// lo default-ea a true; acá replicamos para el path principal.
			Hardspec: true,
		})
		if err != nil {
			return nil, fmt.Errorf("orchestrator run: %w", err)
		}
		return &Response{
			Outcome:           OutcomeOrchestratorStarted,
			Intent:            intent,
			Confidence:        conf,
			IntakeID:          &intakeP.ID,
			FlowRunID:         &orchRes.FlowRunID,
			OrchestratorRunID: &orchRes.OrchestratorRunID,
			SnapshotPrompt:    orchRes.SnapshotPrompt,
			Mode:              string(orchRes.Mode),
			Reasoning:         reasoning,
		}, nil
	}

	// Path legacy (wizard): preservado para deploys sin Orchestrator configurado.
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

// orchestratorModeForIntent decide el modo del orquestador según el
// intent clasificado. Reglas (RFC 0006):
//   - hotfix/fix → Express (cambios pequeños, fast path 2 fases)
//   - feature/refactor/doc/rfc → Full (pipeline 10 fases completo)
//
// El cliente puede override pasando Mode explícito si invoca el MCP
// tool domain_orchestrate directamente.
// handleAnalysis ejecuta el mini-pipeline de análisis read-only si el
// AnalysisService está configurado. Si no, responde con un mensaje default.
func (r *Router) handleAnalysis(ctx context.Context, rawText string, createdBy *uuid.UUID,
	orgID *uuid.UUID, intent Intent, conf float64, reasoning string, intakeID uuid.UUID,
) (*Response, error) {
	if r.AnalysisService == nil || orgID == nil {
		reply := "Clasifiqué el prompt como análisis, pero no tengo el motor de análisis configurado. Si necesitás convertir esto en una feature, decime y arranco el wizard SDD."
		return &Response{
			Outcome:    OutcomeChat,
			Intent:     intent,
			Confidence: conf,
			Reply:      reply,
			Reasoning:  reasoning + " (analysis service not available)",
		}, nil
	}
	userID := uuid.Nil
	if createdBy != nil {
		userID = *createdBy
	}
	result, err := r.AnalysisService.RunAnalysis(ctx, AnalysisInput{
		OrganizationID: *orgID,
		UserID:         userID,
		RawText:        rawText,
	})
	if err != nil {
		return nil, fmt.Errorf("analysis run: %w", err)
	}
	return &Response{
		Outcome:        OutcomeAnalysis,
		Intent:         intent,
		Confidence:     conf,
		IntakeID:       &intakeID,
		KnowledgeDocID: &result.KnowledgeDocID,
		SnapshotPrompt: result.Body,
		Reasoning:      reasoning,
	}, nil
}

func orchestratorModeForIntent(in Intent) orchsvc.Mode {
	switch in {
	case IntentHotfix, IntentFix:
		return orchsvc.ModeExpress
	}
	return orchsvc.ModeFull
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

// defaultChatReply devuelve un reply mínimo cuando el classifier resuelve
// a chat/idea. El protocolo cada-turno (mem_search → responder →
// mem_save) NO se replica acá — ya vive en la policy `agent-protocol`
// (BD, editable) y el MCP server lo inyecta en cada handshake initialize
// via el campo instructions. Duplicarlo acá lo desincroniza con la
// versión viva.
func defaultChatReply(intent Intent, rawText string) string {
	if intent == IntentIdea {
		return "Anoté la idea. Si querés convertirla en feature concreto, pasame el alcance y arrancamos el orquestador SDD."
	}
	return "Recibido."
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
	case containsAny(t, "analiza", "analizá", "investiga", "cuántos", "cuantas",
		"qué hu", "qué tables", "qué endpoints", "qué archivos", "qué módulos",
		"dónde está", "dónde se usa", "cómo está implementado",
		"trazabilidad", "impacto de", "qué pasa si", "explorar", "mapear",
		"listar", "listame", "decime qué", "mostrame",
		"cómo se llama", "qué hace", "qué contiene", "qué relación",
		"análisis", "analysis", "research", "investigate", "explore",
		"find all", "find where", "find out", "tell me about",
		"what modules", "what files", "what endpoints"):
		return IntentAnalysis, 0.7, "keywords de análisis/detección detectadas"
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
