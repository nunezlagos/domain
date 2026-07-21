// inference.go — inferencia de aristas tipadas entre memorias usando un LLM
// (MiniMax-M3 vía endpoint anthropic-compatible), SIN embeddings.
//
// Dos piezas:
//   - SuggestLinks: arma PARES CANDIDATOS por señales baratas (co-sesión,
//     solapamiento de tags, solapamiento léxico tsvector). NO usa LLM ni
//     embeddings: siempre funciona. Sirve para el subagente del cliente
//     (que razona con su propio modelo) y como insumo del razonador server-side.
//   - InferEdgesLLM: toma los candidatos, le pide a MiniMax-M3 que clasifique
//     cada par (supersedes|contradicts|derived_from|depends_on|relates_to|none)
//     con una razón breve, parsea el JSON, valida tipos y crea las aristas vía
//     EdgeService.Link con origin='inferred' (idempotente: maneja ErrEdgeExists).
//
// DEGRADACIÓN ELEGANTE (igual que rerank.go):
//   - SuggestLinks nunca necesita LLM: funciona siempre.
//   - InferEdgesLLM SÍ requiere MiniMax. Si no está disponible (sin
//     MINIMAX_API_KEY → provider "minimax" no registrado), devuelve
//     ErrInferenceUnavailable con mensaje claro, SIN crashear. El suggest_links
//     sigue funcionando.
package observation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/service/observation/observationdb"
)

// ErrInferenceUnavailable se devuelve cuando se pide inferencia LLM pero MiniMax
// no está configurado. El handler MCP la mapea a un mensaje claro sin tumbar el
// flujo: las aristas manuales y el suggest_links (sin LLM) siguen funcionando.
var ErrInferenceUnavailable = errors.New("inferencia LLM requiere MINIMAX_API_KEY")

const (
	// defaultSuggestScanLimit: cuántas observations recientes del project se
	// escanean para armar pares. Acota el costo O(n^2) del cruce de pares.
	defaultSuggestScanLimit = 100

	// maxSuggestScanLimit: tope duro del barrido.
	maxSuggestScanLimit = 300

	// defaultSuggestMaxPairs: cuántos pares candidatos se devuelven por defecto.
	defaultSuggestMaxPairs = 30

	// maxSuggestMaxPairs: tope duro de pares (acota el prompt del LLM y el output).
	maxSuggestMaxPairs = 100

	// inferSnippetMaxLen: recorte del content de cada lado del par en el prompt.
	inferSnippetMaxLen = 500

	// inferMaxTokens: techo de la respuesta del LLM (lista de clasificaciones).
	inferMaxTokens = 4096
)

// CandidatePair es un par de memorias a evaluar, con sus señales baratas y el
// contenido acotado para que un LLM razone sobre la relación.
type CandidatePair struct {
	SourceID       uuid.UUID `json:"source_id"`
	TargetID       uuid.UUID `json:"target_id"`
	SourceContent  string    `json:"source_content"`
	TargetContent  string    `json:"target_content"`
	SourceType     string    `json:"source_type"`
	TargetType     string    `json:"target_type"`
	SourceTags     []string  `json:"source_tags"`
	TargetTags     []string  `json:"target_tags"`
	SameSession    bool      `json:"same_session"`
	SharedTags     int       `json:"shared_tags"`
	LexicalOverlap bool      `json:"lexical_overlap"`
	SignalScore    float64   `json:"signal_score"`
}

// SuggestLinksInput parametriza la generación de pares candidatos.
type SuggestLinksInput struct {
	ProjectID uuid.UUID
	// AnchorID opcional: si está, solo se devuelven pares que incluyan esta
	// observation (un lado del par). Si es nil, se cruzan todas las del barrido.
	AnchorID *uuid.UUID
	// MaxPairs límite de pares devueltos. <=0 usa el default.
	MaxPairs int
	// ScanLimit cuántas observations recientes escanear. <=0 usa el default.
	ScanLimit int
}

// SuggestLinks arma los pares candidatos por señales baratas (co-sesión,
// solapamiento de tags, solapamiento léxico). NO usa LLM ni embeddings: es
// determinista y siempre disponible. El contenido viene recortado a
// inferSnippetMaxLen para acotar el contexto del consumidor (subagente o LLM).
func (s *Service) SuggestLinks(ctx context.Context, in SuggestLinksInput) ([]CandidatePair, error) {
	maxPairs := in.MaxPairs
	if maxPairs <= 0 {
		maxPairs = defaultSuggestMaxPairs
	}
	if maxPairs > maxSuggestMaxPairs {
		maxPairs = maxSuggestMaxPairs
	}
	scanLimit := in.ScanLimit
	if scanLimit <= 0 {
		scanLimit = defaultSuggestScanLimit
	}
	if scanLimit > maxSuggestScanLimit {
		scanLimit = maxSuggestScanLimit
	}

	rows, err := s.queries().FindCandidatePairsBySignals(ctx, observationdb.FindCandidatePairsBySignalsParams{
		ProjectID:   in.ProjectID,
		AnchorID:    in.AnchorID,
		ScanLimit:   int32(scanLimit),
		ResultLimit: int32(maxPairs),
	})
	if err != nil {
		return nil, fmt.Errorf("suggest links: %w", err)
	}

	out := make([]CandidatePair, 0, len(rows))
	for _, r := range rows {
		out = append(out, CandidatePair{
			SourceID:       r.SourceID,
			TargetID:       r.TargetID,
			SourceContent:  clip(r.SourceContent, inferSnippetMaxLen),
			TargetContent:  clip(r.TargetContent, inferSnippetMaxLen),
			SourceType:     r.SourceType,
			TargetType:     r.TargetType,
			SourceTags:     r.SourceTags,
			TargetTags:     r.TargetTags,
			SameSession:    boolVal(r.SameSession),
			SharedTags:     int(r.SharedTags),
			LexicalOverlap: boolVal(r.LexicalOverlap),
			SignalScore:    r.SignalScore,
		})
	}
	return out, nil
}

// InferEdgesLLMInput parametriza una corrida de inferencia LLM.
type InferEdgesLLMInput struct {
	ProjectID uuid.UUID
	AnchorID  *uuid.UUID
	MaxPairs  int
	CreatedBy *uuid.UUID
}

// InferEdgesLLMResult resume el resultado de una corrida de inferencia.
type InferEdgesLLMResult struct {
	Candidates int                  `json:"candidates"` // pares evaluados
	Created    int                  `json:"created"`    // aristas nuevas creadas
	Skipped    int                  `json:"skipped"`    // pares clasificados como 'none'
	Existing   int                  `json:"existing"`   // pares con arista ya existente
	Edges      []InferredEdgeResult `json:"edges"`      // detalle de lo decidido
}

// InferredEdgeResult es una decisión del LLM por par (creada o no).
type InferredEdgeResult struct {
	SourceID uuid.UUID `json:"source_id"`
	TargetID uuid.UUID `json:"target_id"`
	EdgeType string    `json:"edge_type"`
	Reason   string    `json:"reason"`
	Created  bool      `json:"created"`
	Existing bool      `json:"existing,omitempty"`
}

// InferEdgesLLM es el razonador server-side. Arma los candidatos con
// SuggestLinks, los manda a MiniMax-M3 para clasificar la relación de cada par,
// y crea las aristas resultantes vía EdgeService.Link con origin='inferred'.
//
// DEGRADACIÓN: si MiniMax no está disponible devuelve ErrInferenceUnavailable
// (sin crashear). El edges debe estar inyectado para poder crear aristas.
//
// Idempotencia: Link maneja ErrEdgeExists (arista activa ya presente) → se
// cuenta como Existing, no aborta el resto.
func (s *Service) InferEdgesLLM(ctx context.Context, edges EdgeLinker, in InferEdgesLLMInput) (*InferEdgesLLMResult, error) {
	provider, model, err := s.inferProvider()
	if err != nil {
		return nil, err
	}
	inferredBy := model
	if inferredBy == "" {
		inferredBy = provider.Name()
	}

	pairs, err := s.SuggestLinks(ctx, SuggestLinksInput{
		ProjectID: in.ProjectID,
		AnchorID:  in.AnchorID,
		MaxPairs:  in.MaxPairs,
	})
	if err != nil {
		return nil, err
	}
	if len(pairs) == 0 {
		return &InferEdgesLLMResult{}, nil
	}

	system, user := buildInferencePrompt(pairs)
	resp, err := provider.Complete(ctx, llm.CompletionOptions{
		Model:        model,
		Temperature:  0,
		MaxTokens:    inferMaxTokens,
		SystemPrompt: system,
		Messages:     []llm.Message{{Role: "user", Content: user}},
	})
	if err != nil || resp == nil {
		return nil, fmt.Errorf("inferencia LLM falló: %w", err)
	}

	decisions, perr := parseInferenceDecisions(resp.Content)
	if perr != nil {
		return nil, fmt.Errorf("parseo de respuesta LLM falló: %w", perr)
	}

	return s.applyInferenceDecisions(ctx, edges, pairs, decisions, in.CreatedBy, inferredBy)
}

// EdgeLinker es el subconjunto de EdgeService que InferEdgesLLM necesita para
// crear aristas. Permite testear el razonador sin tocar la DB.
type EdgeLinker interface {
	Link(ctx context.Context, in LinkInput) (*Edge, error)
}

// inferProvider resuelve el provider/modelo del rol "infer" (config-driven,
// DOMAINSERV-57) o devuelve ErrInferenceUnavailable. A diferencia del rerank,
// acá la ausencia ES un error (la inferencia LLM no tiene fallback heurístico
// equivalente; el fallback es SuggestLinks crudo).
func (s *Service) inferProvider() (llm.Provider, string, error) {
	if s.LLM == nil {
		return nil, "", ErrInferenceUnavailable
	}
	provider, model, err := s.LLM.ProviderForRole(llm.RoleInfer)
	if err != nil {
		return nil, "", ErrInferenceUnavailable
	}
	return provider, model, nil
}

// applyInferenceDecisions crea las aristas decididas por el LLM. Reconcilia las
// decisiones contra los pares originales (solo pares realmente enviados; ignora
// IDs alucinados) y valida edge_type contra el set permitido.
func (s *Service) applyInferenceDecisions(
	ctx context.Context, edges EdgeLinker, pairs []CandidatePair,
	decisions []inferenceDecision, createdBy *uuid.UUID, inferredBy string,
) (*InferEdgesLLMResult, error) {
	// Index de pares válidos por (source,target) para no confiar en el LLM.
	valid := make(map[string]CandidatePair, len(pairs))
	for _, p := range pairs {
		valid[pairKey(p.SourceID, p.TargetID)] = p
	}

	res := &InferEdgesLLMResult{}
	for _, d := range decisions {
		sid, err1 := uuid.Parse(d.SourceID)
		tid, err2 := uuid.Parse(d.TargetID)
		if err1 != nil || err2 != nil {
			continue
		}
		// El LLM puede invertir el orden source/target respecto al candidato
		// (la dirección importa para la semántica). Aceptamos el par si existe
		// en cualquiera de los dos órdenes como candidato enviado.
		if _, ok := valid[pairKey(sid, tid)]; !ok {
			if _, ok2 := valid[pairKey(tid, sid)]; !ok2 {
				continue // par no enviado: ignorar (anti-alucinación)
			}
		}

		et := strings.ToLower(strings.TrimSpace(d.EdgeType))
		if et == "none" || et == "" {
			res.Skipped++
			continue
		}
		if !validEdgeType(et) {
			res.Skipped++
			continue
		}

		res.Candidates++
		out := InferredEdgeResult{SourceID: sid, TargetID: tid, EdgeType: et, Reason: clip(d.Reason, 280)}

		note := d.Reason
		var notePtr *string
		if strings.TrimSpace(note) != "" {
			notePtr = &note
		}
		_, err := edges.Link(ctx, LinkInput{
			SourceID:   sid,
			TargetID:   tid,
			EdgeType:   et,
			Confidence: inferenceConfidence(d.Confidence),
			Note:       notePtr,
			Metadata:   map[string]any{"origin": "inferred", "inferred_by": inferredBy},
			CreatedBy:  createdBy,
		})
		switch {
		case errors.Is(err, ErrEdgeExists):
			out.Existing = true
			res.Existing++
		case err != nil:
			// Un fallo puntual (ej. cross-project espurio) no aborta el lote.
			out.Created = false
		default:
			out.Created = true
			res.Created++
		}
		res.Edges = append(res.Edges, out)
	}
	// Candidates totales = pares evaluados por el LLM (no solo los creados).
	res.Candidates = len(decisions)
	return res, nil
}

// inferenceConfidence acota la confianza reportada por el LLM a (0,1]. Si el LLM
// no la da o es inválida, usa 0.6 (inferida, menor que manual=1.0).
func inferenceConfidence(c float64) float32 {
	if c <= 0 || c > 1 {
		return 0.6
	}
	return float32(c)
}

// buildInferencePrompt arma un prompt determinista que pide clasificación por
// par en JSON. La dirección source->target es semántica (ver mig 000175).
func buildInferencePrompt(pairs []CandidatePair) (system, user string) {
	system = "Eres un analista de un grafo de conocimiento. Recibes PARES de memorias (notas/decisiones) de un mismo proyecto. " +
		"Para CADA par decide si existe una relación dirigida source -> target y de qué TIPO, eligiendo UNO de:\n" +
		"  supersedes   : source reemplaza/revierte a target (target queda obsoleto)\n" +
		"  contradicts  : source contradice a target\n" +
		"  derived_from : source se deriva de target\n" +
		"  depends_on   : source depende de target\n" +
		"  relates_to   : relación genérica relevante\n" +
		"  none         : no hay relación que valga la pena registrar\n" +
		"La DIRECCIÓN importa: si la relación va en el sentido inverso, intercambia source_id y target_id en tu respuesta. " +
		"Sé conservador: ante la duda usa 'none'. NO inventes IDs; usa solo los provistos.\n" +
		"Responde EXCLUSIVAMENTE un objeto JSON con la forma:\n" +
		"{\"results\":[{\"source_id\":\"<uuid>\",\"target_id\":\"<uuid>\",\"edge_type\":\"<tipo>\",\"confidence\":0.0,\"reason\":\"<breve>\"}]}\n" +
		"Sin texto adicional, sin markdown, sin fences."

	var sb strings.Builder
	sb.WriteString("Pares a analizar:\n")
	for i, p := range pairs {
		fmt.Fprintf(&sb, "\n[%d]\n", i+1)
		fmt.Fprintf(&sb, "source_id=%s (tipo=%s, tags=%s)\n  %s\n", p.SourceID, p.SourceType, strings.Join(p.SourceTags, ","), oneLine(p.SourceContent))
		fmt.Fprintf(&sb, "target_id=%s (tipo=%s, tags=%s)\n  %s\n", p.TargetID, p.TargetType, strings.Join(p.TargetTags, ","), oneLine(p.TargetContent))
		fmt.Fprintf(&sb, "señales: misma_sesion=%t tags_compartidos=%d solape_lexico=%t\n", p.SameSession, p.SharedTags, p.LexicalOverlap)
	}
	sb.WriteString("\nDevuelve solo el JSON {\"results\":[...]}.")
	return system, sb.String()
}

// inferenceDecision es una decisión del LLM por par.
type inferenceDecision struct {
	SourceID   string  `json:"source_id"`
	TargetID   string  `json:"target_id"`
	EdgeType   string  `json:"edge_type"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

type inferenceResponse struct {
	Results []inferenceDecision `json:"results"`
}

// parseInferenceDecisions extrae las decisiones del JSON de la respuesta,
// tolerando prosa/fences alrededor (mismo enfoque defensivo que parseRerankIDs).
func parseInferenceDecisions(raw string) ([]inferenceDecision, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("respuesta vacía")
	}
	start := strings.Index(s, "{")
	if start < 0 {
		return nil, fmt.Errorf("sin objeto JSON en la respuesta")
	}
	depth := 0
	end := -1
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
		if end > 0 {
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("JSON sin terminar en la respuesta")
	}
	var ir inferenceResponse
	if err := json.Unmarshal([]byte(s[start:end]), &ir); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return ir.Results, nil
}

// validEdgeType refleja el CHECK de la mig 000175 (set permitido).
func validEdgeType(t string) bool {
	switch t {
	case "supersedes", "contradicts", "derived_from", "depends_on", "relates_to":
		return true
	}
	return false
}

// queries devuelve un *observationdb.Queries atado al pool (las inferencias son
// de solo lectura para SuggestLinks; la escritura va por EdgeService.Link).
func (s *Service) queries() *observationdb.Queries {
	return observationdb.New(s.Pool)
}

// helpers

func pairKey(a, b uuid.UUID) string { return a.String() + "|" + b.String() }

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func oneLine(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", " ")
}

func boolVal(b *bool) bool { return b != nil && *b }
