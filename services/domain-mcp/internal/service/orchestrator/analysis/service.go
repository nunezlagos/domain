// Package analysis — issue-08.10 ana-002 mini-pipeline de análisis read-only.
//
// El análisis es un intent especial del PromptRouter (IntentAnalysis) que NO
// pasa por el pipeline SDD completo. En lugar de crear issues/proposals/designs,
// ejecuta un mini-pipeline de 2 fases server-side:
//
//  1. explore: usa el LLM para producir un markdown estructurado con la
//     información solicitada por el usuario
//  2. write_doc: persiste el resultado como knowledge_doc + observation
//
// El resultado es un documento searchable que el usuario puede consultar
// después viaja domain_knowledge_search.
package analysis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	knowsvc "nunezlagos/domain/internal/service/knowledge"
	obssvc "nunezlagos/domain/internal/service/observation"
)

// ErrNoProjectForOrg indica que la organización no tiene projects seedeados.
var ErrNoProjectForOrg = errors.New("analysis: no projects found for organization — seed one first")

// DefaultAnalysisSystemPrompt es el system prompt del mini-pipeline de
// análisis read-only por defecto. Se seedea en la tabla prompts con
// slug='analysis' para que sea editable desde el dashboard. El servicio lo
// usa como fallback si la DB no tiene el prompt o el loader no está cableado.
const DefaultAnalysisSystemPrompt = `Eres un analista técnico de proyectos de software.
Dado un prompt de un usuario, produces un análisis markdown estructurado
con la información solicitada.

Reglas:
- Responde ÚNICAMENTE con markdown, sin JSON, sin código extra.
- Si el prompt pide listar algo, produce una lista con formato markdown.
- Si el prompt pide investigar algo, produce un informe estructurado.
- Si no puedes responder porque necesitas acceso al codebase, dilo
  claramente en el análisis.
- El análisis debe ser auto-contenido: cualquiera que lo lea debe
  entender el contexto sin referencias externas.
- Incluye un resumen ejecutivo al inicio y conclusiones al final.`

// Service ejecuta el mini-pipeline de análisis read-only.
type Service struct {
	Pool        *pgxpool.Pool
	Audit       audit.Recorder
	LLM         *llm.Factory
	Knowledge   *knowsvc.Service
	Observation *obssvc.Service

	PromptLoader func(ctx context.Context) (string, error)
}

// Input es lo que recibe RunAnalysis.
type Input struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	RawText        string
}

// Result es lo que devuelve RunAnalysis.
type Result struct {
	KnowledgeDocID uuid.UUID
	Title          string
	Body           string
}

// RunAnalysis ejecuta el mini-pipeline: explora el prompt con el LLM y
// persiste el resultado como knowledge_doc + observation indexable.
func (s *Service) RunAnalysis(ctx context.Context, in Input) (*Result, error) {
	if s.LLM == nil {
		return nil, fmt.Errorf("analysis: LLM factory required")
	}

	projectID, err := s.resolveProjectID(ctx, in.OrganizationID)
	if err != nil {
		return nil, err
	}

	content, err := s.explore(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("analysis explore: %w", err)
	}

	title := inferTitle(content, in.RawText)

	doc, _, err := s.Knowledge.Save(ctx, knowsvc.SaveInput{
		OrganizationID: in.OrganizationID,
		ProjectID:      projectID,
		CreatedBy:      &in.UserID,
		Title:          title,
		Body:           content,
		Source:         "analysis",
		Tags:           []string{"analysis", "auto-generated"},
		Metadata: map[string]any{
			"source_prompt": in.RawText,
			"generated_at":  time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("analysis save knowledge doc: %w", err)
	}

	obsContent := fmt.Sprintf("Analysis: %s\n\nSource prompt: %s\n\nKnowledge doc: %s",
		title, in.RawText, doc.ID.String())
	_, err = s.Observation.Save(ctx, obssvc.SaveInput{
		OrganizationID:  in.OrganizationID,
		ProjectID:       projectID,
		CreatedBy:       &in.UserID,
		Content:         obsContent,
		ObservationType: "analysis",
		Tags:            []string{"analysis", "knowledge_doc"},
		Metadata: map[string]any{
			"knowledge_doc_id": doc.ID.String(),
			"title":            title,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("analysis save observation: %w", err)
	}

	return &Result{
		KnowledgeDocID: doc.ID,
		Title:          title,
		Body:           content,
	}, nil
}

// resolveProjectID busca un project activo. Usamos el primero encontrado
// (single-org, no hay scope por org). Si no hay projects, devuelve
// ErrNoProjectForOrg.
//
// ISSUE-21.6 Fase D clean: el param orgID se ignora (_ = orgID).
func (s *Service) resolveProjectID(ctx context.Context, orgID uuid.UUID) (uuid.UUID, error) {
	_ = orgID
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT id FROM projects
		WHERE deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT 1`,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNoProjectForOrg
		}
		return uuid.Nil, fmt.Errorf("resolve project: %w", err)
	}
	return id, nil
}

// explore usa el LLM para generar contenido de análisis a partir del
// prompt del usuario. Usa un provider default del factory con system
// prompt que instruye al modelo a producir markdown estructurado.
func (s *Service) explore(ctx context.Context, in Input) (string, error) {
	provider, err := s.LLM.Get("")
	if err != nil {
		return "", fmt.Errorf("get provider: %w", err)
	}

	systemPrompt := DefaultAnalysisSystemPrompt
	if s.PromptLoader != nil {
		if loaded, lerr := s.PromptLoader(ctx); lerr == nil && strings.TrimSpace(loaded) != "" {
			systemPrompt = loaded
		}
	}

	resp, err := provider.Complete(ctx, llm.CompletionOptions{
		MaxTokens:    2048,
		Temperature:  0.3,
		SystemPrompt: systemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: in.RawText}},
	})
	if err != nil {
		return "", fmt.Errorf("llm complete: %w", err)
	}

	out := strings.TrimSpace(resp.Content)
	if out == "" {
		return "", fmt.Errorf("llm returned empty content")
	}
	return out, nil
}

// inferTitle extrae un título desde el contenido markdown o fallback al
// prompt original truncado.
func inferTitle(content, rawText string) string {

	lines := strings.SplitN(content, "\n", 5)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimPrefix(trimmed, "# ")
		}
		if strings.HasPrefix(trimmed, "## ") {
			return strings.TrimPrefix(trimmed, "## ")
		}
	}

	words := strings.Fields(rawText)
	if len(words) > 8 {
		words = words[:8]
	}
	title := strings.Join(words, " ")
	if len(title) > 120 {
		title = title[:120]
	}
	return title
}
