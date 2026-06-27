// fase 1 — memory graph: tools MCP de aristas tipadas y bi-temporales entre
// observations (knowledge_observation_edges, mig 000175). Sigue el patrón de
// memory_tools.go: toolXxx() + handleXxx + registerMemoryGraphTools, wrapper
// rls (withOrgTxHandler) y resolución de Principal/project_slug.
package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	obssvc "nunezlagos/domain/internal/service/observation"
)

// memoryEdgeService abstrae el EdgeService para testabilidad del handler.
//
// Single-tenant: los métodos id-based (Link/Unlink/Neighbors/Path) resuelven la
// observation/edge por id dentro del service y derivan el project. Los
// slug-based (graph/infer) reciben el project ya resuelto vía GetBySlug.
type memoryEdgeService interface {
	Link(ctx context.Context, in obssvc.LinkInput) (*obssvc.Edge, error)
	Unlink(ctx context.Context, edgeID uuid.UUID) error
	Neighbors(ctx context.Context, observationID uuid.UUID, direction string, edgeType *string) ([]obssvc.Edge, error)
	Subgraph(ctx context.Context, projectID uuid.UUID, edgeType *string) (nodes []uuid.UUID, edges []obssvc.Edge, err error)
	Path(ctx context.Context, fromID, toID uuid.UUID, maxDepth int) ([]obssvc.Edge, error)
	InferEdges(ctx context.Context, in obssvc.InferInput) (created int, candidates int, err error)
}

// memoryInferenceService abstrae el InferenceService (métodos sobre
// observation.Service) para los tools suggest_links / infer_edges_llm.
//
// SuggestLinks no usa LLM (señales baratas, siempre disponible). InferEdgesLLM
// requiere MiniMax y recibe el edgeLinker para crear las aristas inferidas;
// degrada con ErrInferenceUnavailable si no hay MINIMAX_API_KEY.
type memoryInferenceService interface {
	SuggestLinks(ctx context.Context, in obssvc.SuggestLinksInput) ([]obssvc.CandidatePair, error)
	InferEdgesLLM(ctx context.Context, edges obssvc.EdgeLinker, in obssvc.InferEdgesLLMInput) (*obssvc.InferEdgesLLMResult, error)
}

// validEdgeTypes refleja el CHECK de la mig 000175.
var validEdgeTypes = map[string]bool{
	"supersedes":   true,
	"contradicts":  true,
	"derived_from": true,
	"depends_on":   true,
	"relates_to":   true,
}

type memoryGraphHandlers struct {
	edges     memoryEdgeService
	inference memoryInferenceService
	projects  memoryProjectGetter
	principal *apikey.Principal
}

func registerMemoryGraphTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &memoryGraphHandlers{
		edges:     deps.ObservationEdges,
		inference: deps.Observations,
		projects:  deps.Projects,
		principal: deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolMemLink(), Handler: wrap.Wrap("domain_mem_link", rls(h.handleMemLink))},
		{Tool: toolMemUnlink(), Handler: wrap.Wrap("domain_mem_unlink", rls(h.handleMemUnlink))},
		{Tool: toolMemRelated(), Handler: wrap.Wrap("domain_mem_related", rls(h.handleMemRelated))},
		{Tool: toolMemGraph(), Handler: wrap.Wrap("domain_mem_graph", rls(h.handleMemGraph))},
		{Tool: toolMemPath(), Handler: wrap.Wrap("domain_mem_path", rls(h.handleMemPath))},
		{Tool: toolMemInferEdges(), Handler: wrap.Wrap("domain_mem_infer_edges", rls(h.handleMemInferEdges))},
		{Tool: toolMemSuggestLinks(), Handler: wrap.Wrap("domain_mem_suggest_links", rls(h.handleMemSuggestLinks))},
		{Tool: toolMemInferEdgesLLM(), Handler: wrap.Wrap("domain_mem_infer_edges_llm", rls(h.handleMemInferEdgesLLM))},
	}
}

// edgeJSON serializa una arista para la respuesta del tool.
func edgeJSON(e obssvc.Edge) map[string]any {
	m := map[string]any{
		"id":         e.ID,
		"project_id": e.ProjectID,
		"source_id":  e.SourceID,
		"target_id":  e.TargetID,
		"edge_type":  e.EdgeType,
		"origin":     e.Origin,
		"confidence": e.Confidence,
		"valid_from": e.ValidFrom,
		"valid_to":   e.ValidTo, // nil = vigente
		"created_at": e.CreatedAt,
		"updated_at": e.UpdatedAt,
	}
	if e.Note != nil {
		m["note"] = *e.Note
	}
	if e.Metadata != nil {
		m["metadata"] = e.Metadata
	}
	if e.CreatedBy != nil {
		m["created_by"] = *e.CreatedBy
	}
	return m
}

func toolMemLink() mcp.Tool {
	return mcp.NewTool("domain_mem_link",
		mcp.WithDescription("Crea una arista manual dirigida source -> target entre dos observations del mismo project (origin='manual', confidence=1.0). Tipos: supersedes, contradicts, derived_from, depends_on, relates_to."),
		mcp.WithString("source_id",
			mcp.Description("UUID de la observation origen"),
			mcp.Required(),
		),
		mcp.WithString("target_id",
			mcp.Description("UUID de la observation destino"),
			mcp.Required(),
		),
		mcp.WithString("edge_type",
			mcp.Description("Tipo de relacion: supersedes | contradicts | derived_from | depends_on | relates_to"),
			mcp.Required(),
		),
		mcp.WithString("note",
			mcp.Description("Nota opcional describiendo la relacion"),
		),
	)
}

func (h *memoryGraphHandlers) handleMemLink(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}

	args := req.GetArguments()
	sourceID, err := uuid.Parse(strOf(args["source_id"]))
	if err != nil {
		return mcp.NewToolResultError("source_id invalido"), nil
	}
	targetID, err := uuid.Parse(strOf(args["target_id"]))
	if err != nil {
		return mcp.NewToolResultError("target_id invalido"), nil
	}
	edgeType := strOf(args["edge_type"])
	if !validEdgeTypes[edgeType] {
		return mcp.NewToolResultError("edge_type invalido: usar supersedes | contradicts | derived_from | depends_on | relates_to"), nil
	}
	if sourceID == targetID {
		return mcp.NewToolResultError("source_id y target_id deben ser distintos"), nil
	}

	var note *string
	if n := strOf(args["note"]); n != "" {
		note = &n
	}
	var createdBy *uuid.UUID
	if uid, err := uuid.Parse(h.principal.UserID); err == nil {
		createdBy = &uid
	}

	// El service resuelve source/target por id (GetObservation) y deriva el
	// project del source validado; el handler no provee project.
	edge, err := h.edges.Link(ctx, obssvc.LinkInput{
		SourceID:   sourceID,
		TargetID:   targetID,
		EdgeType:   edgeType,
		Confidence: 1.0,
		Note:       note,
		CreatedBy:  createdBy,
	})
	if err != nil {
		switch {
		case errors.Is(err, obssvc.ErrEdgeExists):
			return mcp.NewToolResultError("ya existe una arista activa con ese source, target y tipo"), nil
		case errors.Is(err, obssvc.ErrEdgeSelf):
			return mcp.NewToolResultError("source_id y target_id deben ser distintos"), nil
		case errors.Is(err, obssvc.ErrEdgeCrossProject):
			return mcp.NewToolResultError("source y target deben pertenecer al mismo project"), nil
		case errors.Is(err, obssvc.ErrNotFound):
			return mcp.NewToolResultError(fmt.Sprintf("observation not found: %v", err)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("link failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"edge": edgeJSON(*edge)})
}

func toolMemUnlink() mcp.Tool {
	return mcp.NewTool("domain_mem_unlink",
		mcp.WithDescription("Elimina (soft-delete) una arista del grafo de memoria por id."),
		mcp.WithString("edge_id",
			mcp.Description("UUID de la arista a eliminar"),
			mcp.Required(),
		),
	)
}

func (h *memoryGraphHandlers) handleMemUnlink(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	args := req.GetArguments()
	edgeID, err := uuid.Parse(strOf(args["edge_id"]))
	if err != nil {
		return mcp.NewToolResultError("edge_id invalido"), nil
	}
	if err := h.edges.Unlink(ctx, edgeID); err != nil {
		if errors.Is(err, obssvc.ErrEdgeNotFound) {
			return mcp.NewToolResultError("edge not found"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("unlink failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"deleted": true, "edge_id": edgeID})
}

func toolMemRelated() mcp.Tool {
	return mcp.NewTool("domain_mem_related",
		mcp.WithDescription("Devuelve las observations vecinas (aristas vigentes) de una observation. direction: forward (salientes) | backward (entrantes) | both (default). edge_type opcional filtra por tipo."),
		mcp.WithString("observation_id",
			mcp.Description("UUID de la observation ancla"),
			mcp.Required(),
		),
		mcp.WithString("direction",
			mcp.Description("forward | backward | both (default both)"),
		),
		mcp.WithString("edge_type",
			mcp.Description("Filtrar por tipo de arista (opcional)"),
		),
	)
}

// parseMemRelatedArgs parsea y valida los args de domain_mem_related.
func parseMemRelatedArgs(req mcp.CallToolRequest) (obsID uuid.UUID, direction string, edgeType *string, errRes *mcp.CallToolResult) {
	args := req.GetArguments()
	obsID, err := uuid.Parse(strOf(args["observation_id"]))
	if err != nil {
		return uuid.Nil, "", nil, mcp.NewToolResultError("observation_id invalido")
	}
	direction = strOf(args["direction"])
	switch direction {
	case "", "both":
		direction = "both"
	case "forward", "backward":
	default:
		return uuid.Nil, "", nil, mcp.NewToolResultError("direction invalida: usar forward | backward | both")
	}
	if et := strOf(args["edge_type"]); et != "" {
		if !validEdgeTypes[et] {
			return uuid.Nil, "", nil, mcp.NewToolResultError("edge_type invalido")
		}
		edgeType = &et
	}
	return obsID, direction, edgeType, nil
}

func (h *memoryGraphHandlers) handleMemRelated(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	obsID, direction, edgeType, errRes := parseMemRelatedArgs(req)
	if errRes != nil {
		return errRes, nil
	}

	// El service resuelve la observation por id y deriva el project.
	edges, err := h.edges.Neighbors(ctx, obsID, direction, edgeType)
	if err != nil {
		if errors.Is(err, obssvc.ErrNotFound) {
			return mcp.NewToolResultError("observation not found"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("related failed: %v", err)), nil
	}

	edgesOut := make([]map[string]any, 0, len(edges))
	seen := make(map[uuid.UUID]struct{})
	neighbors := make([]uuid.UUID, 0, len(edges))
	for _, e := range edges {
		edgesOut = append(edgesOut, edgeJSON(e))
		// el vecino es el extremo distinto de obsID.
		other := e.TargetID
		if e.TargetID == obsID {
			other = e.SourceID
		}
		if _, ok := seen[other]; !ok {
			seen[other] = struct{}{}
			neighbors = append(neighbors, other)
		}
	}
	return toolResultJSON(map[string]any{
		"observation_id": obsID,
		"direction":      direction,
		"edges":          edgesOut,
		"neighbors":      neighbors,
		"count":          len(edgesOut),
	})
}

// graphEdgeLimit acota el output del subgrafo para no explotar el contexto.
const graphEdgeLimit = 500

func toolMemGraph() mcp.Tool {
	return mcp.NewTool("domain_mem_graph",
		mcp.WithDescription("Subgrafo vigente de un project: nodos (observation ids) y aristas, con resumen de conteo por edge_type. Limita la salida a 500 aristas."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project"),
			mcp.Required(),
		),
		mcp.WithString("edge_type",
			mcp.Description("Filtrar por tipo de arista (opcional)"),
		),
	)
}

func (h *memoryGraphHandlers) handleMemGraph(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	orgID, err := uuid.Parse(h.principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	args := req.GetArguments()
	slug := strOf(args["project_slug"])
	if slug == "" {
		return mcp.NewToolResultError("project_slug es requerido"), nil
	}
	proj, err := h.projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
	}
	var edgeType *string
	if et := strOf(args["edge_type"]); et != "" {
		if !validEdgeTypes[et] {
			return mcp.NewToolResultError("edge_type invalido"), nil
		}
		edgeType = &et
	}

	nodes, edges, err := h.edges.Subgraph(ctx, proj.ID, edgeType)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("graph failed: %v", err)), nil
	}

	byType := map[string]int{}
	for _, e := range edges {
		byType[e.EdgeType]++
	}

	truncated := false
	if len(edges) > graphEdgeLimit {
		edges = edges[:graphEdgeLimit]
		truncated = true
	}
	edgesOut := make([]map[string]any, 0, len(edges))
	for _, e := range edges {
		edgesOut = append(edgesOut, edgeJSON(e))
	}

	return toolResultJSON(map[string]any{
		"project_slug": slug,
		"nodes":        nodes,
		"edges":        edgesOut,
		"summary": map[string]any{
			"node_count":    len(nodes),
			"edges_total":   sumInts(byType),
			"by_edge_type":  byType,
			"truncated":     truncated,
			"edges_emitted": len(edgesOut),
		},
	})
}

func sumInts(m map[string]int) int {
	t := 0
	for _, v := range m {
		t += v
	}
	return t
}

func toolMemPath() mcp.Tool {
	return mcp.NewTool("domain_mem_path",
		mcp.WithDescription("Camino mas corto (en numero de aristas) entre dos observations recorriendo aristas vigentes en direccion source -> target. Devuelve la cadena de aristas o found=false si no hay camino."),
		mcp.WithString("from_id",
			mcp.Description("UUID de la observation origen"),
			mcp.Required(),
		),
		mcp.WithString("to_id",
			mcp.Description("UUID de la observation destino"),
			mcp.Required(),
		),
		mcp.WithNumber("max_depth",
			mcp.Description("Profundidad maxima del BFS (default 6)"),
		),
	)
}

func (h *memoryGraphHandlers) handleMemPath(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	args := req.GetArguments()
	fromID, err := uuid.Parse(strOf(args["from_id"]))
	if err != nil {
		return mcp.NewToolResultError("from_id invalido"), nil
	}
	toID, err := uuid.Parse(strOf(args["to_id"]))
	if err != nil {
		return mcp.NewToolResultError("to_id invalido"), nil
	}
	maxDepth := 0
	if v, ok := args["max_depth"].(float64); ok {
		maxDepth = int(v)
	}

	// El service resuelve la observation origen por id y deriva el project.
	edges, err := h.edges.Path(ctx, fromID, toID, maxDepth)
	if err != nil {
		if errors.Is(err, obssvc.ErrNotFound) {
			return mcp.NewToolResultError("from observation not found"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("path failed: %v", err)), nil
	}
	if edges == nil {
		return toolResultJSON(map[string]any{
			"from_id": fromID,
			"to_id":   toID,
			"found":   false,
		})
	}
	edgesOut := make([]map[string]any, 0, len(edges))
	for _, e := range edges {
		edgesOut = append(edgesOut, edgeJSON(e))
	}
	return toolResultJSON(map[string]any{
		"from_id": fromID,
		"to_id":   toID,
		"found":   true,
		"length":  len(edgesOut),
		"path":    edgesOut,
	})
}

func toolMemInferEdges() mcp.Tool {
	return mcp.NewTool("domain_mem_infer_edges",
		mcp.WithDescription("Infiere aristas relates_to (origin='inferred') por similitud de embedding entre observations del project. observation_id opcional acota a una sola fuente. Devuelve {created, candidates}."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project"),
			mcp.Required(),
		),
		mcp.WithString("observation_id",
			mcp.Description("UUID de una observation fuente (opcional; si se omite infiere desde todas las del project)"),
		),
		mcp.WithNumber("threshold",
			mcp.Description("Similitud coseno minima 0..1 (default 0.85)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Candidatos top-K por observation (default 10)"),
		),
	)
}

// buildInferInput resuelve el project del slug y arma el InferInput desde los
// args de domain_mem_infer_edges (observation_id/threshold/limit/created_by).
func (h *memoryGraphHandlers) buildInferInput(ctx context.Context, req mcp.CallToolRequest) (obssvc.InferInput, *mcp.CallToolResult) {
	orgID, err := uuid.Parse(h.principal.OrganizationID)
	if err != nil {
		return obssvc.InferInput{}, mcp.NewToolResultError("invalid principal org_id")
	}
	args := req.GetArguments()
	slug := strOf(args["project_slug"])
	if slug == "" {
		return obssvc.InferInput{}, mcp.NewToolResultError("project_slug es requerido")
	}
	proj, err := h.projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return obssvc.InferInput{}, mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug))
	}

	in := obssvc.InferInput{ProjectID: proj.ID}
	if oid := strOf(args["observation_id"]); oid != "" {
		id, err := uuid.Parse(oid)
		if err != nil {
			return obssvc.InferInput{}, mcp.NewToolResultError("observation_id invalido")
		}
		in.ObservationID = &id
	}
	if v, ok := args["threshold"].(float64); ok {
		in.Threshold = v
	}
	if v, ok := args["limit"].(float64); ok {
		in.TopK = int(v)
	}
	if uid, err := uuid.Parse(h.principal.UserID); err == nil {
		in.CreatedBy = &uid
	}
	return in, nil
}

func (h *memoryGraphHandlers) handleMemInferEdges(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	in, errRes := h.buildInferInput(ctx, req)
	if errRes != nil {
		return errRes, nil
	}

	created, candidates, err := h.edges.InferEdges(ctx, in)
	if err != nil {
		if errors.Is(err, obssvc.ErrNotFound) {
			return mcp.NewToolResultError("observation not found"), nil
		}
		if errors.Is(err, obssvc.ErrEdgeCrossProject) {
			return mcp.NewToolResultError("la observation no pertenece al project indicado"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("infer failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"created":    created,
		"candidates": candidates,
	})
}

// resolveProjectArg resuelve el project del slug en los args (single-tenant:
// orgID del principal es vestigial, GetBySlug lo ignora) y opcionalmente parsea
// observation_id como ancla. Devuelve un errRes listo si algo falla.
func (h *memoryGraphHandlers) resolveProjectArg(ctx context.Context, req mcp.CallToolRequest) (projID uuid.UUID, slug string, anchor *uuid.UUID, errRes *mcp.CallToolResult) {
	orgID, err := uuid.Parse(h.principal.OrganizationID)
	if err != nil {
		return uuid.Nil, "", nil, mcp.NewToolResultError("invalid principal org_id")
	}
	args := req.GetArguments()
	slug = strOf(args["project_slug"])
	if slug == "" {
		return uuid.Nil, "", nil, mcp.NewToolResultError("project_slug es requerido")
	}
	proj, err := h.projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return uuid.Nil, "", nil, mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug))
	}
	if oid := strOf(args["observation_id"]); oid != "" {
		id, perr := uuid.Parse(oid)
		if perr != nil {
			return uuid.Nil, "", nil, mcp.NewToolResultError("observation_id invalido")
		}
		anchor = &id
	}
	return proj.ID, slug, anchor, nil
}

func toolMemSuggestLinks() mcp.Tool {
	return mcp.NewTool("domain_mem_suggest_links",
		mcp.WithDescription("Arma PARES CANDIDATOS de memorias para analizar relaciones, SIN embeddings ni LLM, por señales baratas: co-sesion (mismo session_id), solapamiento de tags y solapamiento lexico (tsvector en espanol). Devuelve los pares con su contenido acotado y las señales, ordenados por un score heuristico. Util para que un subagente (IDE) razone las relaciones y luego las cree con domain_mem_link, o como insumo de domain_mem_infer_edges_llm."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project"),
			mcp.Required(),
		),
		mcp.WithString("observation_id",
			mcp.Description("UUID de una observation ancla (opcional; si se indica solo se devuelven pares que la incluyan)"),
		),
		mcp.WithNumber("max_pairs",
			mcp.Description("Maximo de pares a devolver (default 30, tope 100)"),
		),
	)
}

func (h *memoryGraphHandlers) handleMemSuggestLinks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if h.inference == nil {
		return mcp.NewToolResultError("inference service no disponible"), nil
	}
	projID, slug, anchor, errRes := h.resolveProjectArg(ctx, req)
	if errRes != nil {
		return errRes, nil
	}
	args := req.GetArguments()
	maxPairs := 0
	if v, ok := args["max_pairs"].(float64); ok {
		maxPairs = int(v)
	}

	pairs, err := h.inference.SuggestLinks(ctx, obssvc.SuggestLinksInput{
		ProjectID: projID,
		AnchorID:  anchor,
		MaxPairs:  maxPairs,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("suggest_links failed: %v", err)), nil
	}

	out := make([]map[string]any, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, map[string]any{
			"source_id":       p.SourceID,
			"target_id":       p.TargetID,
			"source_content":  p.SourceContent,
			"target_content":  p.TargetContent,
			"source_type":     p.SourceType,
			"target_type":     p.TargetType,
			"source_tags":     p.SourceTags,
			"target_tags":     p.TargetTags,
			"same_session":    p.SameSession,
			"shared_tags":     p.SharedTags,
			"lexical_overlap": p.LexicalOverlap,
			"signal_score":    p.SignalScore,
		})
	}
	return toolResultJSON(map[string]any{
		"project_slug": slug,
		"pairs":        out,
		"count":        len(out),
		"edge_types":   []string{"supersedes", "contradicts", "derived_from", "depends_on", "relates_to"},
		"hint":         "Razona cada par y crea las relaciones que correspondan con domain_mem_link (source_id, target_id, edge_type).",
	})
}

func toolMemInferEdgesLLM() mcp.Tool {
	return mcp.NewTool("domain_mem_infer_edges_llm",
		mcp.WithDescription("Razonador server-side: arma pares candidatos (como suggest_links), le pide a MiniMax-M3 que clasifique la relacion de cada par (supersedes|contradicts|derived_from|depends_on|relates_to|none) y crea las aristas resultantes con origin='inferred' (idempotente). REQUIERE MINIMAX_API_KEY; si no esta seteada devuelve un error claro sin crashear (suggest_links sigue funcionando). Devuelve {candidates, created, skipped, existing, edges}."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project"),
			mcp.Required(),
		),
		mcp.WithString("observation_id",
			mcp.Description("UUID de una observation ancla (opcional; acota los pares a los que la incluyan)"),
		),
		mcp.WithNumber("max_pairs",
			mcp.Description("Maximo de pares a evaluar por el LLM (default 30, tope 100)"),
		),
	)
}

func (h *memoryGraphHandlers) handleMemInferEdgesLLM(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if h.inference == nil {
		return mcp.NewToolResultError("inference service no disponible"), nil
	}
	if h.edges == nil {
		return mcp.NewToolResultError("edge service no disponible"), nil
	}
	projID, slug, anchor, errRes := h.resolveProjectArg(ctx, req)
	if errRes != nil {
		return errRes, nil
	}
	args := req.GetArguments()
	maxPairs := 0
	if v, ok := args["max_pairs"].(float64); ok {
		maxPairs = int(v)
	}
	var createdBy *uuid.UUID
	if uid, err := uuid.Parse(h.principal.UserID); err == nil {
		createdBy = &uid
	}

	res, err := h.inference.InferEdgesLLM(ctx, h.edges, obssvc.InferEdgesLLMInput{
		ProjectID: projID,
		AnchorID:  anchor,
		MaxPairs:  maxPairs,
		CreatedBy: createdBy,
	})
	if err != nil {
		if errors.Is(err, obssvc.ErrInferenceUnavailable) {
			return mcp.NewToolResultError("inferencia LLM requiere MINIMAX_API_KEY (usa domain_mem_suggest_links para obtener los pares y crearlos manualmente con domain_mem_link)"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("infer_edges_llm failed: %v", err)), nil
	}

	edgesOut := make([]map[string]any, 0, len(res.Edges))
	for _, e := range res.Edges {
		edgesOut = append(edgesOut, map[string]any{
			"source_id": e.SourceID,
			"target_id": e.TargetID,
			"edge_type": e.EdgeType,
			"reason":    e.Reason,
			"created":   e.Created,
			"existing":  e.Existing,
		})
	}
	return toolResultJSON(map[string]any{
		"project_slug": slug,
		"candidates":   res.Candidates,
		"created":      res.Created,
		"skipped":      res.Skipped,
		"existing":     res.Existing,
		"edges":        edgesOut,
	})
}

// strOf hace un type-assert seguro a string.
func strOf(v any) string {
	s, _ := v.(string)
	return s
}
