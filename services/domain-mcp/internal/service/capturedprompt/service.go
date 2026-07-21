package capturedprompt

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

var ErrEmptyContent = errors.New("captured_prompt: content required")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

type CaptureInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID

	ProjectID  *uuid.UUID
	Content    string
	ClientKind string
	Model      string
}

// Capture persiste un prompt del usuario. char_count se computa server-side
// como proxy de tokens hasta tener integración con el cliente IDE real.
func (s *Service) Capture(ctx context.Context, in CaptureInput) (*Prompt, error) {
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return nil, ErrEmptyContent
	}
	return s.repo.Insert(ctx, InsertParams{
		OrganizationID: in.OrganizationID,
		UserID:         in.UserID,
		ProjectID:      in.ProjectID,
		Content:        content,
		ClientKind:     strings.TrimSpace(in.ClientKind),
		Model:          strings.TrimSpace(in.Model),
		CharCount:      utf8.RuneCountInString(content),
	})
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]*Prompt, int64, error) {
	return s.repo.List(ctx, orgID, filter)
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Prompt, error) {
	return s.repo.Get(ctx, orgID, id)
}

// CompleteTurn (REQ-47): cierra el turn con el output del LLM, estima
// tokens out y completa turn_completed_at. response_chars=0 es válido
// (turn sin respuesta — útil para trackear timeouts/cancels).
func (s *Service) CompleteTurn(ctx context.Context, in CompleteTurnInput) (*Prompt, error) {
	if in.ResponseChars < 0 {
		in.ResponseChars = 0
	}
	in.Model = strings.TrimSpace(in.Model)
	return s.repo.CompleteTurn(ctx, in)
}

// SummarizeByProject agrega tokens estimados de todos los turns de un project.
func (s *Service) SummarizeByProject(ctx context.Context, orgID, projectID uuid.UUID) (*SessionUsage, error) {
	return s.repo.SummarizeByProject(ctx, orgID, projectID)
}

// Defaults del heatmap (DOMAINSERV-61).
const (
	heatmapDefaultMinTurns    = 2
	heatmapDefaultMaxClusters = 50
	// heatmapSuggestThreshold: repeticiones de un cluster a partir de las cuales
	// vale la pena PROPONER estandarizarlo como skill/policy (human-in-the-loop).
	heatmapSuggestThreshold = 3
)

// HeatmapCluster es un cluster del mapa de calor con su propuesta (si aplica).
type HeatmapCluster struct {
	Key       string `json:"cluster_key"`
	Turns     int    `json:"turns"`
	Tokens    int64  `json:"tokens"`
	Sample    string `json:"sample"`
	SuggestAs string `json:"suggest_as,omitempty"` // skill | policy | ""
}

// Suggestion propone convertir un patrón repetido en skill/policy. NO se
// persiste: es una sugerencia para que el humano confirme (RFC lifecycle).
type Suggestion struct {
	ClusterKey string `json:"cluster_key"`
	Kind       string `json:"kind"` // skill | policy
	Reason     string `json:"reason"`
	Turns      int    `json:"turns"`
	Sample     string `json:"sample"`
}

// HeatmapResult es la respuesta read-only del heatmap: clusters + sugerencias.
type HeatmapResult struct {
	ProjectID   uuid.UUID        `json:"project_id"`
	Clusters    []HeatmapCluster `json:"clusters"`
	Suggestions []Suggestion     `json:"suggestions"`
}

// Heatmap agrupa los prompts del project por similitud (firma normalizada, en
// Postgres, sin LLM) con frecuencia + tokens, y propone estandarizar los
// clusters que superan el umbral como skill/policy. NUNCA persiste — la
// creación queda a confirmación humana.
func (s *Service) Heatmap(ctx context.Context, orgID, projectID uuid.UUID, minTurns, maxClusters int) (*HeatmapResult, error) {
	if minTurns <= 0 {
		minTurns = heatmapDefaultMinTurns
	}
	if maxClusters <= 0 {
		maxClusters = heatmapDefaultMaxClusters
	}
	clusters, err := s.repo.HeatmapByProject(ctx, orgID, projectID, minTurns, maxClusters)
	if err != nil {
		return nil, err
	}
	res := &HeatmapResult{ProjectID: projectID, Clusters: make([]HeatmapCluster, 0, len(clusters))}
	for _, c := range clusters {
		hc := HeatmapCluster{Key: c.Key, Turns: c.Turns, Tokens: c.Tokens, Sample: c.Sample}
		if c.Turns >= heatmapSuggestThreshold {
			kind := suggestionKind(c.Sample)
			hc.SuggestAs = kind
			res.Suggestions = append(res.Suggestions, Suggestion{
				ClusterKey: c.Key, Kind: kind, Turns: c.Turns, Sample: c.Sample,
				Reason: "patrón repetido; considerá estandarizarlo (sin auto-crear: confirmá vos)",
			})
		}
		res.Clusters = append(res.Clusters, hc)
	}
	return res, nil
}

// ruleWords: palabras de regla/restricción que sugieren policy sobre skill.
var ruleWords = map[string]bool{
	"siempre": true, "nunca": true, "prohibido": true,
	"debe": true, "regla": true, "obligatorio": true,
}

// suggestionKind heurística barata: prompts con lenguaje de regla/restricción →
// policy; el resto → skill. Determinística, sin LLM. Match por palabra completa
// para no colisionar (ej. "arreglar" no debe matchear "regla").
func suggestionKind(sample string) string {
	for _, w := range strings.Fields(strings.ToLower(sample)) {
		if ruleWords[strings.Trim(w, ".,;:!¿?\"'")] {
			return "policy"
		}
	}
	return "skill"
}
