// rerank.go — paso opcional de RERANK sobre los resultados de SearchHybrid
// usando un LLM (MiniMax-M3 vía endpoint anthropic-compatible).
//
// El rerank es un MEJORADOR best-effort, NUNCA un bloqueante. Reglas de
// degradación elegante (idénticas al resto del wiring MiniMax):
//   - Si no hay Factory LLM inyectado            → orden BM25/RRF original.
//   - Si el provider "minimax" no está registrado → orden BM25/RRF original.
//     (MiniMax solo se registra si hay MINIMAX_API_KEY; sin key, no existe.)
//   - Si la llamada al LLM falla                  → orden BM25/RRF original.
//   - Si el parseo de la respuesta falla          → orden BM25/RRF original.
//   - Si el LLM omite o inventa IDs               → se reconcilian (ver reorder).
//
// En ningún caso devuelve error por culpa del rerank: el error se traga y se
// retorna el orden previo. El caller no necesita saber si hubo rerank o no.
package observation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/llm"
)

const (
	// defaultRerankTopN candidatos que se mandan al LLM para reordenar.
	// Acota el prompt (y el costo) sin perder los resultados más prometedores
	// del BM25/RRF, que ya vienen ordenados.
	defaultRerankTopN = 30

	// maxRerankTopN tope duro para no inflar el prompt sin control.
	maxRerankTopN = 50

	// rerankSnippetMaxLen recorta el content de cada candidato en el prompt.
	// El LLM solo necesita lo suficiente para juzgar relevancia, no el doc entero.
	rerankSnippetMaxLen = 400

	// rerankMaxTokens techo de la respuesta: es una lista de IDs, no necesita más.
	rerankMaxTokens = 1024
)

// RerankOptions configura el paso opcional de rerank en SearchHybrid.
type RerankOptions struct {
	// Enabled activa el rerank. Default false (no gastar tokens salvo que se pida).
	Enabled bool
	// TopN cuántos candidatos del BM25/RRF se mandan al LLM. <=0 usa el default.
	TopN int
}

// SearchHybridReranked ejecuta SearchHybrid y, si opts.Enabled, intenta un paso
// de rerank LLM sobre los top-N candidatos. SIEMPRE devuelve hasta `limit`
// resultados; ante cualquier fallo del rerank degrada silenciosamente al orden
// BM25/RRF original (mismo contrato que SearchHybrid).
//
// Single-tenant: se aísla por orgID/project igual que SearchHybrid (el rerank
// solo reordena los resultados ya filtrados, nunca amplía el universo).
func (s *Service) SearchHybridReranked(
	ctx context.Context, orgID uuid.UUID, query string, limit int, opts RerankOptions,
) ([]SearchResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Cuando hay rerank, pedimos a SearchHybrid suficientes candidatos para que
	// el LLM tenga material que reordenar (al menos topN), no solo `limit`.
	topN := opts.TopN
	if topN <= 0 {
		topN = defaultRerankTopN
	}
	if topN > maxRerankTopN {
		topN = maxRerankTopN
	}

	searchLimit := limit
	if opts.Enabled && topN > searchLimit {
		searchLimit = topN
	}

	results, err := s.SearchHybrid(ctx, orgID, query, searchLimit)
	if err != nil {
		return nil, err
	}
	if !opts.Enabled || len(results) <= 1 {
		return truncate(results, limit), nil
	}

	// Tomar los top-N candidatos (results ya viene ordenado por RRF desc).
	candidates := results
	if len(candidates) > topN {
		candidates = candidates[:topN]
	}

	reordered, ok := s.rerankWithLLM(ctx, query, candidates)
	if !ok {
		// Degradación: orden BM25/RRF original.
		return truncate(results, limit), nil
	}
	return truncate(reordered, limit), nil
}

// rerankWithLLM manda query + snippets de los candidatos a MiniMax-M3 y devuelve
// los candidatos reordenados según el ranking del modelo. El segundo retorno es
// false si NO se pudo rerankear (degradar al orden original). Nunca paniquea ni
// propaga error: cualquier problema => (nil, false).
func (s *Service) rerankWithLLM(
	ctx context.Context, query string, candidates []SearchResult,
) (out []SearchResult, ok bool) {
	// Guard total: si algo explota dentro (provider raro, panic en parse),
	// el rerank no debe tumbar la búsqueda.
	defer func() {
		if r := recover(); r != nil {
			out, ok = nil, false
		}
	}()

	if s.LLM == nil {
		return nil, false
	}

	// Resuelve el provider/modelo del rol "rerank" (config-driven, DOMAINSERV-57).
	// Sin provider disponible (ni default), degradar al orden BM25/RRF.
	provider, model, err := s.LLM.ProviderForRole(llm.RoleRerank)
	if err != nil {
		return nil, false
	}

	systemPrompt, userPrompt := buildRerankPrompt(query, candidates)

	resp, err := provider.Complete(ctx, llm.CompletionOptions{
		Model:        model,
		Temperature:  0,
		MaxTokens:    rerankMaxTokens,
		SystemPrompt: systemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: userPrompt}},
	})
	if err != nil || resp == nil {
		return nil, false
	}

	order, perr := parseRerankIDs(resp.Content)
	if perr != nil || len(order) == 0 {
		return nil, false
	}

	return reorderByIDs(candidates, order), true
}

// buildRerankPrompt arma un prompt acotado y determinista. Pide salida JSON
// parseable: {"order": ["<id>", ...]} con los IDs reordenados por relevancia.
func buildRerankPrompt(query string, candidates []SearchResult) (system, user string) {
	system = "Eres un reordenador de resultados de búsqueda. Recibes una consulta y una lista de documentos candidatos, cada uno con un ID. " +
		"Devuelve EXCLUSIVAMENTE un objeto JSON con la forma {\"order\": [\"id1\", \"id2\", ...]} " +
		"listando TODOS los IDs recibidos, ordenados del más relevante al menos relevante para la consulta. " +
		"No agregues texto, explicaciones ni markdown. No inventes IDs. No omitas IDs."

	var sb strings.Builder
	sb.WriteString("Consulta:\n")
	sb.WriteString(query)
	sb.WriteString("\n\nCandidatos:\n")
	for _, c := range candidates {
		snippet := c.Content
		if len(snippet) > rerankSnippetMaxLen {
			snippet = snippet[:rerankSnippetMaxLen]
		}
		// Una línea por candidato: ID + snippet en una sola línea.
		snippet = strings.ReplaceAll(snippet, "\n", " ")
		fmt.Fprintf(&sb, "- id=%s :: %s\n", c.ID.String(), snippet)
	}
	sb.WriteString("\nDevuelve solo el JSON {\"order\": [...]}.")
	return system, sb.String()
}

// rerankResponse es la forma esperada de la salida del LLM.
type rerankResponse struct {
	Order []string `json:"order"`
}

// parseRerankIDs extrae la lista de IDs ordenados de la respuesta del LLM.
// Tolera prosa/fences alrededor del JSON (algunos modelos los agregan).
func parseRerankIDs(raw string) ([]string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("empty rerank response")
	}
	// Recortar al primer objeto JSON balanceado.
	start := strings.Index(s, "{")
	if start < 0 {
		return nil, fmt.Errorf("no JSON object in rerank response")
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
		return nil, fmt.Errorf("unterminated JSON in rerank response")
	}
	var rr rerankResponse
	if err := json.Unmarshal([]byte(s[start:end]), &rr); err != nil {
		return nil, fmt.Errorf("unmarshal rerank: %w", err)
	}
	return rr.Order, nil
}

// reorderByIDs reordena candidates según la secuencia order. Reconciliación
// defensiva contra alucinaciones del modelo:
//   - IDs en `order` que no existen en candidates se ignoran.
//   - IDs en candidates que el modelo omitió se anexan al final preservando
//     su orden BM25/RRF original (no se pierde ningún resultado).
//   - IDs duplicados en `order` se cuentan una sola vez.
func reorderByIDs(candidates []SearchResult, order []string) []SearchResult {
	byID := make(map[string]SearchResult, len(candidates))
	for _, c := range candidates {
		byID[c.ID.String()] = c
	}

	out := make([]SearchResult, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, id := range order {
		if seen[id] {
			continue
		}
		if c, ok := byID[id]; ok {
			out = append(out, c)
			seen[id] = true
		}
	}
	// Anexar los que el modelo omitió, en su orden original.
	for _, c := range candidates {
		if !seen[c.ID.String()] {
			out = append(out, c)
			seen[c.ID.String()] = true
		}
	}
	return out
}

// truncate devuelve los primeros n elementos (o todos si hay menos).
func truncate(rs []SearchResult, n int) []SearchResult {
	if n > 0 && len(rs) > n {
		return rs[:n]
	}
	return rs
}
