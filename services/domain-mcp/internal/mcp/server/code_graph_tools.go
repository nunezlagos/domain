// fase 2d — code graph: tools MCP del grafo de CÓDIGO (tablas code_nodes /
// code_edges / code_index_files, mig 000178). Sigue el patrón EXACTO de
// memory_graph_tools.go: toolXxx() + handleXxx + registerCodeGraphTools, wrapper
// rls (withOrgTxHandler) y resolución de Principal/project_slug vía
// projects.GetBySlug.
//
// El binario domain-mcp corre stdio EN la máquina del cliente, así que tiene
// acceso al filesystem: domain_code_build recibe un root_path absoluto del repo,
// lo recorre, parsea los .go y persiste el grafo en Postgres. Las demás tools
// (explore/path/graph) sólo consultan el grafo ya persistido.
//
// Aislamiento single-tenant: TODO se acota por project_id (resuelto del slug).
// NADA de organization_id ni REFERENCES organizations.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	codegraphsvc "nunezlagos/domain/internal/service/codegraph"
)

// codeGraphService abstrae el CodegraphService para testabilidad del handler.
type codeGraphService interface {
	Build(ctx context.Context, in codegraphsvc.BuildInput) (*codegraphsvc.BuildStats, error)
	Upload(ctx context.Context, in codegraphsvc.UploadInput) (*codegraphsvc.UploadStats, error)
	Explore(ctx context.Context, projectID uuid.UUID, symbol string) (*codegraphsvc.ExploreResult, error)
	Path(ctx context.Context, projectID uuid.UUID, fromSymbol, toSymbol string, maxDepth int) ([]codegraphsvc.CodeEdge, error)
	Overview(ctx context.Context, projectID uuid.UUID) (*codegraphsvc.Overview, error)
	// fase 3 — cruce memoria/código.
	LinkObservationToCode(ctx context.Context, in codegraphsvc.LinkInput) (*codegraphsvc.ObsCodeLink, error)
	ObservationsForCode(ctx context.Context, projectID uuid.UUID, symbol string) ([]codegraphsvc.LinkedObservation, error)
	CodeForObservation(ctx context.Context, observationID uuid.UUID) ([]codegraphsvc.ObsCodeLink, []codegraphsvc.CodeNode, error)
}

// validCodeLinkTypes refleja el CHECK de la mig 000179.
var validCodeLinkTypes = map[string]bool{
	"affects":    true,
	"decided_in": true,
	"references": true,
	"implements": true,
}

// codeGraphEdgeLimit acota el output de god-nodes / paths para no explotar el
// contexto del agente.
const codeGraphEdgeLimit = 200

type codeGraphHandlers struct {
	graph     codeGraphService
	projects  memoryProjectGetter
	principal *apikey.Principal
}

func registerCodeGraphTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &codeGraphHandlers{
		graph:     deps.CodeGraph,
		projects:  deps.Projects,
		principal: deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolCodeBuild(), Handler: wrap.Wrap("domain_code_build", rls(h.handleCodeBuild))},
		{Tool: toolCodeUpload(), Handler: wrap.Wrap("domain_code_upload", rls(h.handleCodeUpload))},
		{Tool: toolCodeExplore(), Handler: wrap.Wrap("domain_code_explore", rls(h.handleCodeExplore))},
		{Tool: toolCodePath(), Handler: wrap.Wrap("domain_code_path", rls(h.handleCodePath))},
		{Tool: toolCodeGraph(), Handler: wrap.Wrap("domain_code_graph", rls(h.handleCodeGraph))},
		{Tool: toolMemLinkCode(), Handler: wrap.Wrap("domain_mem_link_code", rls(h.handleMemLinkCode))},
		{Tool: toolCodeObservations(), Handler: wrap.Wrap("domain_code_observations", rls(h.handleCodeObservations))},
		{Tool: toolMemCodeLinks(), Handler: wrap.Wrap("domain_mem_code_links", rls(h.handleMemCodeLinks))},
	}
}

// resolveProject resuelve el project del slug usando el orgID (vestigial) del
// Principal. GetBySlug ignora el orgID y filtra por slug (single-tenant); se
// pasa por consistencia con el resto de las tools.
func (h *codeGraphHandlers) resolveProject(ctx context.Context, slug string) (projectID uuid.UUID, errResult *mcp.CallToolResult) {
	if h.principal == nil {
		return uuid.Nil, mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)")
	}
	if h.projects == nil || h.graph == nil {
		return uuid.Nil, mcp.NewToolResultError("code graph service no configurado")
	}
	if slug == "" {
		return uuid.Nil, mcp.NewToolResultError("project_slug es requerido")
	}
	orgID, err := uuid.Parse(h.principal.OrganizationID)
	if err != nil {
		return uuid.Nil, mcp.NewToolResultError("invalid principal org_id")
	}
	proj, err := h.projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return uuid.Nil, mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug))
	}
	return proj.ID, nil
}

func toolCodeBuild() mcp.Tool {
	return mcp.NewTool("domain_code_build",
		mcp.WithDescription("Construye/refresca el grafo de CÓDIGO Go de un project recorriendo el repo en root_path (ruta ABSOLUTA en el filesystem local). Incremental: saltea archivos sin cambios (content_hash) y soft-deletea los que ya no existen. Excluye vendor/, testdata/, node_modules/, dirs ocultos y *_test.go (salvo include_tests). Devuelve BuildStats. Corre esto antes de domain_code_explore/path/graph."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project donde persistir el grafo"),
			mcp.Required(),
		),
		mcp.WithString("root_path",
			mcp.Description("Ruta ABSOLUTA del repo en el filesystem local (la raíz a recorrer)"),
			mcp.Required(),
		),
		mcp.WithBoolean("include_tests",
			mcp.Description("Si true, NO excluye archivos *_test.go. Default false."),
		),
	)
}

func (h *codeGraphHandlers) handleCodeBuild(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	slug := strOf(args["project_slug"])
	projectID, errRes := h.resolveProject(ctx, slug)
	if errRes != nil {
		return errRes, nil
	}

	rootPath := strOf(args["root_path"])
	if rootPath == "" {
		return mcp.NewToolResultError("root_path es requerido (ruta absoluta del repo)"), nil
	}
	includeTests, _ := args["include_tests"].(bool)

	// git_head best-effort: si root_path es un repo git, derivar el HEAD.
	gitHead := deriveGitHead(ctx, rootPath)

	stats, err := h.graph.Build(ctx, codegraphsvc.BuildInput{
		ProjectID:    projectID,
		RootPath:     rootPath,
		GitHead:      gitHead,
		IncludeTests: includeTests,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("build failed: %v", err)), nil
	}

	return toolResultJSON(map[string]any{
		"project_slug": slug,
		"root_path":    rootPath,
		"git_head":     gitHead,
		"stats": map[string]any{
			"files_scanned":  stats.FilesScanned,
			"files_parsed":   stats.FilesParsed,
			"files_skipped":  stats.FilesSkipped,
			"files_removed":  stats.FilesRemoved,
			"nodes_upserted": stats.NodesUpserted,
			"edges_created":  stats.EdgesCreated,
		},
	})
}

// deriveGitHead intenta `git rev-parse HEAD` en dir. Best-effort: "" si falla
// (no es un repo git, git no instalado, etc). No es un error fatal del build.
func deriveGitHead(ctx context.Context, dir string) string {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func toolCodeExplore() mcp.Tool {
	return mcp.NewTool("domain_code_explore",
		mcp.WithDescription("Blast-radius de un símbolo Go: nodo(s) que matchean (qualified_name exacto o nombre vía ILIKE) + callers (quién lo llama) + callees (a quién llama), con file:line y signature. Útil para saber el impacto de cambiar una función/método antes de tocarla."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project"),
			mcp.Required(),
		),
		mcp.WithString("symbol",
			mcp.Description("Símbolo a explorar: qualified_name (ej 'pkg.Tipo.Metodo') o nombre simple (ej 'Build')"),
			mcp.Required(),
		),
	)
}

func (h *codeGraphHandlers) handleCodeExplore(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	slug := strOf(args["project_slug"])
	projectID, errRes := h.resolveProject(ctx, slug)
	if errRes != nil {
		return errRes, nil
	}
	symbol := strOf(args["symbol"])
	if symbol == "" {
		return mcp.NewToolResultError("symbol es requerido"), nil
	}

	res, err := h.graph.Explore(ctx, projectID, symbol)
	if err != nil {
		if errors.Is(err, codegraphsvc.ErrNodeNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("symbol '%s' not found en el grafo (¿corriste domain_code_build?)", symbol)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("explore failed: %v", err)), nil
	}

	matches := make([]map[string]any, 0, len(res.Matches))
	for _, m := range res.Matches {
		matches = append(matches, codeNodeJSON(m))
	}
	callers := make([]map[string]any, 0, len(res.Callers))
	for _, e := range res.Callers {
		callers = append(callers, codeEdgeJSON(e))
	}
	callees := make([]map[string]any, 0, len(res.Callees))
	for _, e := range res.Callees {
		callees = append(callees, codeEdgeJSON(e))
	}

	return toolResultJSON(map[string]any{
		"project_slug": slug,
		"symbol":       symbol,
		"matches":      matches,
		"callers":      callers,
		"callees":      callees,
		"summary": map[string]any{
			"match_count":  len(matches),
			"caller_count": len(callers),
			"callee_count": len(callees),
		},
	})
}

func toolCodePath() mcp.Tool {
	return mcp.NewTool("domain_code_path",
		mcp.WithDescription("Camino más corto (en número de aristas) entre dos símbolos Go recorriendo el grafo en dirección source->target (calls/method_of/etc). Cada símbolo debe resolver a UN solo nodo (si es ambiguo, usa un qualified_name). Devuelve la cadena de aristas o found=false."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project"),
			mcp.Required(),
		),
		mcp.WithString("from_symbol",
			mcp.Description("Símbolo origen (qualified_name o nombre simple)"),
			mcp.Required(),
		),
		mcp.WithString("to_symbol",
			mcp.Description("Símbolo destino (qualified_name o nombre simple)"),
			mcp.Required(),
		),
		mcp.WithNumber("max_depth",
			mcp.Description("Profundidad máxima del BFS (default 8)"),
		),
	)
}

func (h *codeGraphHandlers) handleCodePath(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	slug := strOf(args["project_slug"])
	projectID, errRes := h.resolveProject(ctx, slug)
	if errRes != nil {
		return errRes, nil
	}
	fromSymbol := strOf(args["from_symbol"])
	toSymbol := strOf(args["to_symbol"])
	if fromSymbol == "" || toSymbol == "" {
		return mcp.NewToolResultError("from_symbol y to_symbol son requeridos"), nil
	}
	maxDepth := 0
	if v, ok := args["max_depth"].(float64); ok {
		maxDepth = int(v)
	}

	edges, err := h.graph.Path(ctx, projectID, fromSymbol, toSymbol, maxDepth)
	if err != nil {
		switch {
		case errors.Is(err, codegraphsvc.ErrNodeNotFound):
			return mcp.NewToolResultError("from_symbol o to_symbol no existe en el grafo (¿corriste domain_code_build?)"), nil
		case errors.Is(err, codegraphsvc.ErrSymbolAmbiguous):
			return mcp.NewToolResultError("símbolo ambiguo: resolvió a más de un nodo, usa un qualified_name (ej 'pkg.Tipo.Metodo')"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("path failed: %v", err)), nil
	}
	if edges == nil {
		return toolResultJSON(map[string]any{
			"project_slug": slug,
			"from_symbol":  fromSymbol,
			"to_symbol":    toSymbol,
			"found":        false,
		})
	}
	edgesOut := make([]map[string]any, 0, len(edges))
	for _, e := range edges {
		edgesOut = append(edgesOut, codeEdgeJSON(e))
	}
	return toolResultJSON(map[string]any{
		"project_slug": slug,
		"from_symbol":  fromSymbol,
		"to_symbol":    toSymbol,
		"found":        true,
		"length":       len(edgesOut),
		"path":         edgesOut,
	})
}

func toolCodeGraph() mcp.Tool {
	return mcp.NewTool("domain_code_graph",
		mcp.WithDescription("Overview del grafo de código del project: total de nodos/aristas, conteo por kind (file/func/method/type/...) y los god-nodes (mayor grado entrada+salida, candidatos a hotspot/acoplamiento alto). Útil para entender la forma del codebase de un vistazo."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project"),
			mcp.Required(),
		),
	)
}

func (h *codeGraphHandlers) handleCodeGraph(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	slug := strOf(args["project_slug"])
	projectID, errRes := h.resolveProject(ctx, slug)
	if errRes != nil {
		return errRes, nil
	}

	ov, err := h.graph.Overview(ctx, projectID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("graph overview failed: %v", err)), nil
	}

	byKind := make([]map[string]any, 0, len(ov.ByKind))
	for _, kc := range ov.ByKind {
		byKind = append(byKind, map[string]any{"kind": kc.Kind, "count": kc.Count})
	}

	gods := ov.GodNodes
	truncated := false
	if len(gods) > codeGraphEdgeLimit {
		gods = gods[:codeGraphEdgeLimit]
		truncated = true
	}
	godNodes := make([]map[string]any, 0, len(gods))
	for _, g := range gods {
		godNodes = append(godNodes, map[string]any{
			"node":       codeNodeJSON(g.Node),
			"in_degree":  g.InDegree,
			"out_degree": g.OutDegree,
			"degree":     g.Degree,
		})
	}

	return toolResultJSON(map[string]any{
		"project_slug": slug,
		"total_nodes":  ov.TotalNodes,
		"total_edges":  ov.TotalEdges,
		"by_kind":      byKind,
		"god_nodes":    godNodes,
		"truncated":    truncated,
	})
}

// codeNodeJSON serializa un nodo de código para la respuesta del tool.
func codeNodeJSON(n codegraphsvc.CodeNode) map[string]any {
	m := map[string]any{
		"id":             n.ID,
		"kind":           n.Kind,
		"name":           n.Name,
		"qualified_name": n.QualifiedName,
		"file_path":      n.FilePath,
		"line_start":     n.LineStart,
		"line_end":       n.LineEnd,
		"language":       n.Language,
	}
	if n.Signature != "" {
		m["signature"] = n.Signature
	}
	if n.Doc != "" {
		m["doc"] = n.Doc
	}
	return m
}

// codeEdgeJSON serializa una arista de código para la respuesta del tool.
func codeEdgeJSON(e codegraphsvc.CodeEdge) map[string]any {
	m := map[string]any{
		"id":             e.ID,
		"source_node_id": e.SourceNodeID,
		"target_node_id": e.TargetNodeID,
		"edge_type":      e.EdgeType,
	}
	if e.Metadata != nil {
		m["metadata"] = e.Metadata
	}
	return m
}

func toolMemLinkCode() mcp.Tool {
	return mcp.NewTool("domain_mem_link_code",
		mcp.WithDescription("Vincula una memoria/decisión (observation_id) con un nodo del grafo de CÓDIGO del mismo project. El nodo se identifica por code_node_id (UUID) o por symbol (qualified_name o nombre simple, debe resolver a UN solo nodo). link_type: affects | decided_in | references | implements. Idempotente: si el vínculo ya existe lo reporta. Ejecuta domain_code_build antes para tener nodos."),
		mcp.WithString("observation_id",
			mcp.Description("UUID de la observation/memoria a vincular"),
			mcp.Required(),
		),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project (debe ser el de la observation y el del nodo)"),
			mcp.Required(),
		),
		mcp.WithString("symbol",
			mcp.Description("Símbolo del nodo: qualified_name (ej 'pkg.Tipo.Metodo') o nombre simple. Alternativa a code_node_id."),
		),
		mcp.WithString("code_node_id",
			mcp.Description("UUID del nodo de código. Alternativa a symbol (tiene prioridad si se da)."),
		),
		mcp.WithString("link_type",
			mcp.Description("Tipo de vínculo: affects | decided_in | references | implements"),
			mcp.Required(),
		),
		mcp.WithString("note",
			mcp.Description("Nota opcional describiendo el vínculo"),
		),
	)
}

// parseMemLinkCodeArgs valida los args de domain_mem_link_code y arma el
// LinkInput (el nodo se identifica por code_node_id o por symbol).
func (h *codeGraphHandlers) parseMemLinkCodeArgs(req mcp.CallToolRequest, projectID uuid.UUID) (codegraphsvc.LinkInput, *mcp.CallToolResult) {
	args := req.GetArguments()
	obsID, err := uuid.Parse(strOf(args["observation_id"]))
	if err != nil {
		return codegraphsvc.LinkInput{}, mcp.NewToolResultError("observation_id invalido")
	}
	linkType := strOf(args["link_type"])
	if !validCodeLinkTypes[linkType] {
		return codegraphsvc.LinkInput{}, mcp.NewToolResultError("link_type invalido: usar affects | decided_in | references | implements")
	}

	in := codegraphsvc.LinkInput{
		ObservationID: obsID,
		ProjectID:     projectID,
		LinkType:      linkType,
		Note:          strOf(args["note"]),
	}
	if cnid := strOf(args["code_node_id"]); cnid != "" {
		id, err := uuid.Parse(cnid)
		if err != nil {
			return codegraphsvc.LinkInput{}, mcp.NewToolResultError("code_node_id invalido")
		}
		in.CodeNodeID = id
	} else {
		in.Symbol = strOf(args["symbol"])
		if in.Symbol == "" {
			return codegraphsvc.LinkInput{}, mcp.NewToolResultError("symbol o code_node_id es requerido")
		}
	}
	if h.principal != nil {
		if uid, err := uuid.Parse(h.principal.UserID); err == nil {
			in.CreatedBy = &uid
		}
	}
	return in, nil
}

func (h *codeGraphHandlers) handleMemLinkCode(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	slug := strOf(req.GetArguments()["project_slug"])
	projectID, errRes := h.resolveProject(ctx, slug)
	if errRes != nil {
		return errRes, nil
	}

	in, errRes := h.parseMemLinkCodeArgs(req, projectID)
	if errRes != nil {
		return errRes, nil
	}

	link, err := h.graph.LinkObservationToCode(ctx, in)
	if err != nil {
		switch {
		case errors.Is(err, codegraphsvc.ErrLinkExists):
			return mcp.NewToolResultError("ya existe un vínculo activo con esa observation, nodo y tipo"), nil
		case errors.Is(err, codegraphsvc.ErrObservationNotFound):
			return mcp.NewToolResultError("observation not found"), nil
		case errors.Is(err, codegraphsvc.ErrNodeNotFound):
			return mcp.NewToolResultError("nodo de código no encontrado (¿corriste domain_code_build?)"), nil
		case errors.Is(err, codegraphsvc.ErrSymbolAmbiguous):
			return mcp.NewToolResultError("símbolo ambiguo: resolvió a más de un nodo, usa un qualified_name o code_node_id"), nil
		case errors.Is(err, codegraphsvc.ErrLinkCrossProject):
			return mcp.NewToolResultError("la observation y el nodo deben pertenecer al mismo project"), nil
		case errors.Is(err, codegraphsvc.ErrLinkBadType):
			return mcp.NewToolResultError("link_type invalido"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("link failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"project_slug": slug,
		"link":         obsCodeLinkJSON(*link),
	})
}

func toolCodeObservations() mcp.Tool {
	return mcp.NewTool("domain_code_observations",
		mcp.WithDescription("Lista las memorias/decisiones vinculadas a un nodo de CÓDIGO (resuelto por symbol dentro del project), con su contenido, tipo y el tipo de vínculo. Responde '¿qué decisiones afectan a esta función/tipo?'."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project"),
			mcp.Required(),
		),
		mcp.WithString("symbol",
			mcp.Description("Símbolo del nodo: qualified_name (ej 'pkg.Tipo.Metodo') o nombre simple. Debe resolver a UN solo nodo."),
			mcp.Required(),
		),
	)
}

func (h *codeGraphHandlers) handleCodeObservations(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	slug := strOf(args["project_slug"])
	projectID, errRes := h.resolveProject(ctx, slug)
	if errRes != nil {
		return errRes, nil
	}
	symbol := strOf(args["symbol"])
	if symbol == "" {
		return mcp.NewToolResultError("symbol es requerido"), nil
	}

	obs, err := h.graph.ObservationsForCode(ctx, projectID, symbol)
	if err != nil {
		switch {
		case errors.Is(err, codegraphsvc.ErrNodeNotFound):
			return mcp.NewToolResultError(fmt.Sprintf("symbol '%s' not found en el grafo (¿corriste domain_code_build?)", symbol)), nil
		case errors.Is(err, codegraphsvc.ErrSymbolAmbiguous):
			return mcp.NewToolResultError("símbolo ambiguo: resolvió a más de un nodo, usa un qualified_name"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("observations failed: %v", err)), nil
	}

	out := make([]map[string]any, 0, len(obs))
	for _, o := range obs {
		out = append(out, linkedObservationJSON(o))
	}
	return toolResultJSON(map[string]any{
		"project_slug": slug,
		"symbol":       symbol,
		"observations": out,
		"count":        len(out),
	})
}

func toolMemCodeLinks() mcp.Tool {
	return mcp.NewTool("domain_mem_code_links",
		mcp.WithDescription("Lista los nodos de CÓDIGO vinculados a una memoria/decisión (observation_id), con el tipo de vínculo y el detalle de cada nodo (kind/qualified_name/file:line). Responde '¿qué código toca esta decisión?'."),
		mcp.WithString("observation_id",
			mcp.Description("UUID de la observation/memoria"),
			mcp.Required(),
		),
	)
}

func (h *codeGraphHandlers) handleMemCodeLinks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if h.graph == nil {
		return mcp.NewToolResultError("code graph service no configurado"), nil
	}
	args := req.GetArguments()
	obsID, err := uuid.Parse(strOf(args["observation_id"]))
	if err != nil {
		return mcp.NewToolResultError("observation_id invalido"), nil
	}

	links, nodes, err := h.graph.CodeForObservation(ctx, obsID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("code links failed: %v", err)), nil
	}

	// índice nodo por id para anexar el detalle a cada vínculo.
	nodeByID := make(map[uuid.UUID]map[string]any, len(nodes))
	for _, n := range nodes {
		nodeByID[n.ID] = codeNodeJSON(n)
	}
	out := make([]map[string]any, 0, len(links))
	for _, l := range links {
		m := obsCodeLinkJSON(l)
		if nj, ok := nodeByID[l.CodeNodeID]; ok {
			m["node"] = nj
		}
		out = append(out, m)
	}
	return toolResultJSON(map[string]any{
		"observation_id": obsID,
		"links":          out,
		"count":          len(out),
	})
}

// obsCodeLinkJSON serializa un vínculo memoria->código para la respuesta.
func obsCodeLinkJSON(l codegraphsvc.ObsCodeLink) map[string]any {
	m := map[string]any{
		"id":             l.ID,
		"project_id":     l.ProjectID,
		"observation_id": l.ObservationID,
		"code_node_id":   l.CodeNodeID,
		"link_type":      l.LinkType,
		"created_at":     l.CreatedAt,
	}
	if l.Note != "" {
		m["note"] = l.Note
	}
	if l.CreatedBy != nil {
		m["created_by"] = *l.CreatedBy
	}
	if l.Metadata != nil {
		m["metadata"] = l.Metadata
	}
	return m
}

// linkedObservationJSON serializa una memoria vinculada (con contenido) para la
// respuesta de domain_code_observations.
func linkedObservationJSON(o codegraphsvc.LinkedObservation) map[string]any {
	m := map[string]any{
		"link_id":                o.LinkID,
		"link_type":              o.LinkType,
		"linked_at":              o.LinkedAt,
		"observation_id":         o.ObservationID,
		"content":                o.Content,
		"observation_type":       o.ObservationType,
		"observation_created_at": o.ObservationCreatedAt,
	}
	if o.Note != "" {
		m["note"] = o.Note
	}
	return m
}

// toolCodeUpload — domain_code_upload (multi-lenguaje client-side)
//
// Reemplaza el flujo server-side de domain_code_build (que recibe root_path y
// recorre el FS). En setups donde domain-mcp corre en el VPS y no tiene acceso
// al filesystem del cliente, el cliente parsea con ast-grep y sube el grafo
// completo via esta tool.
//
// Input: graph_json con {nodes, edges, files_scanned, git_head}. El formato
// de cada ParsedNode y ParsedEdge es el MISMO que el parser server-side
// produce (ver service.go), asi que el cliente no necesita un formato custom.
func toolCodeUpload() mcp.Tool {
	return mcp.NewTool("domain_code_upload",
		mcp.WithDescription("Sube el code graph parseado por el cliente (multi-lenguaje via ast-grep). Reemplaza a domain_code_build cuando el server no tiene acceso al filesystem del cliente. Input: graph_json con {nodes, edges, files_scanned, git_head}."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project donde persistir el grafo"),
			mcp.Required(),
		),
		mcp.WithObject("graph_json",
			mcp.Description("JSON con {files_scanned:int, git_head:string, nodes:[{kind,name,qualified_name,file_path,line_start,line_end,signature,doc}], edges:[{source_qn,target_qn,edge_type}]}"),
			mcp.Required(),
		),
	)
}

func (h *codeGraphHandlers) handleCodeUpload(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	slug := strOf(args["project_slug"])
	projectID, errRes := h.resolveProject(ctx, slug)
	if errRes != nil {
		return errRes, nil
	}

	graphArg, ok := args["graph_json"].(map[string]any)
	if !ok {
		return mcp.NewToolResultError("graph_json es requerido (objeto con {files_scanned, git_head, nodes, edges})"), nil
	}

	// Unmarshal de nodes y edges (formato ParsedNode/ParsedEdge).
	nodesRaw, _ := graphArg["nodes"].([]any)
	edgesRaw, _ := graphArg["edges"].([]any)
	filesScanned := intOf(graphArg["files_scanned"])
	gitHead := strOf(graphArg["git_head"])

	nodes := make([]codegraphsvc.ParsedNode, 0, len(nodesRaw))
	for _, n := range nodesRaw {
		nm, ok := n.(map[string]any)
		if !ok {
			continue
		}
		nodes = append(nodes, codegraphsvc.ParsedNode{
			Kind:          strOf(nm["kind"]),
			Name:          strOf(nm["name"]),
			QualifiedName: strOf(nm["qualified_name"]),
			FilePath:      strOf(nm["file_path"]),
			LineStart:     intOf(nm["line_start"]),
			LineEnd:       intOf(nm["line_end"]),
			Signature:     strOf(nm["signature"]),
			Doc:           strOf(nm["doc"]),
		})
	}

	edges := make([]codegraphsvc.ParsedEdge, 0, len(edgesRaw))
	for _, e := range edgesRaw {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		edges = append(edges, codegraphsvc.ParsedEdge{
			SourceQN: strOf(em["source_qn"]),
			TargetQN: strOf(em["target_qn"]),
			EdgeType: strOf(em["edge_type"]),
		})
	}

	stats, err := h.graph.Upload(ctx, codegraphsvc.UploadInput{
		ProjectID:    projectID,
		GitHead:      gitHead,
		FilesScanned: filesScanned,
		Nodes:        nodes,
		Edges:        edges,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("upload failed: %v", err)), nil
	}

	return toolResultJSON(map[string]any{
		"project_slug":  slug,
		"files_scanned": stats.FilesScanned,
		"nodes_upserted": stats.NodesUpserted,
		"edges_created":  stats.EdgesCreated,
	})
}

// intOf convierte un valor numerico de JSON a int, con default 0.
func intOf(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}
