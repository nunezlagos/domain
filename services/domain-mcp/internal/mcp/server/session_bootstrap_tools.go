// REQ-45 — Ola C: auto-registro de proyectos al abrir + detección de
// cambios significativos.
//
// El cliente IDE (Claude Code/OpenCode) llama domain_session_bootstrap
// al PRIMER turn de cada sesión nueva (instrucción que va en
// platform_policies/agent-protocol). El server detecta si el cwd +
// git_remote corresponde a un project conocido y:
//
//  1. Conocido → devuelve overlay: { project, last_known_head, current_head,
//     head_changed, recent_observations, policies_summary }.
//     Si head_changed=true, sugiere refrescar memorias.
//  2. Desconocido → known=false + suggestion: { slug, remote_detected }
//     + next_step: "domain_session_register".
//
// domain_session_register crea el project, registra el remoto en
// project_repositories, persiste el HEAD inicial y graba una
// observation de bootstrap. Idempotente: si el slug ya existe, lo
// reutiliza (devuelve known=true en lugar de duplicar).
package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	policysvc "nunezlagos/domain/internal/service/policy"
	projsvc "nunezlagos/domain/internal/service/project"
	projectreposvc "nunezlagos/domain/internal/service/projectrepo"
	"nunezlagos/domain/internal/store/txctx"
)

type bootstrapPoliciesLister interface {
	List(ctx context.Context, kind string) ([]policysvc.Policy, error)
}

type bootstrapProjectsService interface {
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*projsvc.Project, error)
	GetByID(ctx context.Context, id uuid.UUID) (*projsvc.Project, error)
}

type bootstrapProjectRepos interface {
	Add(ctx context.Context, in projectreposvc.AddInput) (*projectreposvc.Repo, error)
}

type sessionBootstrapHandlers struct {
	policies     bootstrapPoliciesLister
	projects     bootstrapProjectsService
	projectRepos bootstrapProjectRepos
	pool         *pgxpool.Pool
	principal    *apikey.Principal
}

func (h *sessionBootstrapHandlers) q(ctx context.Context) interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
} {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return h.pool
}

func registerSessionBootstrapTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &sessionBootstrapHandlers{
		policies:     deps.Policies,
		projects:     deps.Projects,
		projectRepos: deps.ProjectRepos,
		pool:         deps.Pool,
		principal:    deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolSessionBootstrap(), Handler: wrap.Wrap("domain_session_bootstrap", rls(h.handleSessionBootstrap))},
		{Tool: toolSessionRegister(), Handler: wrap.Wrap("domain_session_register", rls(h.handleSessionRegister))},
	}
}

func toolSessionBootstrap() mcp.Tool {
	return mcp.NewTool("domain_session_bootstrap",
		mcp.WithDescription("Llamar al PRIMER turn de cada sesión nueva. Manda cwd + git_remote + git_branch + git_head + (opcional) existing_rules_files al server. Devuelve overlay con datos del proyecto (si es conocido) o un cuestionario para registrarlo (si no). Si hay .md de rules sin importar, devuelve import_candidates [{path, suggested_slug, suggested_kind, suggested_scope}] — el agente los propone al usuario (AskUserQuestion) antes de importar como policy (REQ-55). El client debe seguir el next_step indicado."),
		mcp.WithString("cwd",
			mcp.Description("Working directory absoluto del cliente (ej. /home/user/Proyectos/acme-web). El basename se usa como slug-candidate si no hay match."),
			mcp.Required(),
		),
		mcp.WithString("git_remote",
			mcp.Description("URL del remote git (de `git remote get-url origin` o equivalente). Vacío si no es repo git."),
		),
		mcp.WithString("git_branch",
			mcp.Description("Branch actual (`git branch --show-current`)."),
		),
		mcp.WithString("git_head",
			mcp.Description("SHA1 del HEAD actual (`git rev-parse HEAD`)."),
		),
		mcp.WithArray("existing_rules_files",
			mcp.Description("Lista de paths (relativos a cwd) de archivos AI-rules detectados por el cliente: AGENTS.md, CLAUDE.md, .claude/CLAUDE.md, .cursorrules, .windsurf/rules, .github/copilot-instructions.md, openspec/. El server los reporta en su response como suggested_imports — el LLM puede leerlos con su tool Read y llamar domain_project_policy_import_from_text para volcarlos como policies del proyecto sin perder lo que el repo ya documenta."),
		),
	)
}

func toolSessionRegister() mcp.Tool {
	return mcp.NewTool("domain_session_register",
		mcp.WithDescription("Registra un proyecto NUEVO en domain tras el cuestionario al usuario. Idempotente: si slug ya existe en la org, devuelve el existente (known=true). Crea el project + registra el remoto en project_repositories + persiste HEAD inicial."),
		mcp.WithString("slug", mcp.Description("Slug del proyecto (kebab-case, único por org)."), mcp.Required()),
		mcp.WithString("name", mcp.Description("Nombre legible del proyecto."), mcp.Required()),
		mcp.WithString("description", mcp.Description("Descripción opcional 1-2 líneas.")),
		mcp.WithString("remote_url", mcp.Description("URL del remote default. Si vacío, no se crea project_repository.")),
		mcp.WithString("remote_name", mcp.Description("Nombre del remoto (origin/upstream). Default: origin.")),
		mcp.WithString("branch_default", mcp.Description("Rama principal del remoto.")),
		mcp.WithString("workflow", mcp.Description("merge|pr|mr|trunk_based|rebase.")),
		mcp.WithString("kind", mcp.Description("github|gitlab|bitbucket|gitea|other.")),
		mcp.WithString("git_head", mcp.Description("SHA1 HEAD inicial.")),
		mcp.WithString("git_branch", mcp.Description("Branch actual.")),
		mcp.WithString("cwd", mcp.Description("Cwd donde se está registrando.")),
	)
}

// matchProjectFromCwdOrRemote intenta resolver project_id usando:
//  1. project_repositories.url match exacto al git_remote.
//  2. projects.slug match al basename(cwd).
//  3. projects.repository_url match exacto al git_remote (compat).
func (h *sessionBootstrapHandlers) matchProjectFromCwdOrRemote(ctx context.Context, orgID uuid.UUID, cwd, gitRemote string) (uuid.UUID, string, bool) {
	// 1. project_repositories.url
	if gitRemote != "" {
		var pid uuid.UUID
		var pslug string
		err := h.q(ctx).QueryRow(ctx,
			`SELECT p.id, p.slug
			   FROM project_repositories pr
			   JOIN projects p ON p.id = pr.project_id
			   WHERE pr.url = $1
			     AND pr.deleted_at IS NULL AND p.deleted_at IS NULL
			   LIMIT 1`,
			gitRemote,
		).Scan(&pid, &pslug)
		if err == nil {
			return pid, pslug, true
		}
	}
	// 2. projects.slug = basename(cwd)
	base := filepath.Base(strings.TrimRight(cwd, "/"))
	if base != "" && base != "." && base != "/" {
		var pid uuid.UUID
		err := h.q(ctx).QueryRow(ctx,
			`SELECT id FROM projects
			   WHERE slug = $1 AND deleted_at IS NULL`,
			base,
		).Scan(&pid)
		if err == nil {
			return pid, base, true
		}
	}
	// 3. projects.repository_url (compat con columna legacy)
	if gitRemote != "" {
		var pid uuid.UUID
		var pslug string
		err := h.q(ctx).QueryRow(ctx,
			`SELECT id, slug FROM projects
			   WHERE repository_url = $1 AND deleted_at IS NULL
			   LIMIT 1`,
			gitRemote,
		).Scan(&pid, &pslug)
		if err == nil {
			return pid, pslug, true
		}
	}
	return uuid.Nil, "", false
}

func (h *sessionBootstrapHandlers) handleSessionBootstrap(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.pool == nil {
		return mcp.NewToolResultError("pool not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	cwd, _ := args["cwd"].(string)
	gitRemote, _ := args["git_remote"].(string)
	gitBranch, _ := args["git_branch"].(string)
	gitHead, _ := args["git_head"].(string)
	if cwd == "" {
		return mcp.NewToolResultError("cwd requerido"), nil
	}

	// existing_rules_files: el cliente IDE lista qué archivos AI-rules existen
	// en el cwd. El server NO lee el filesystem del cliente — solo reporta
	// estos paths en el response para que el LLM los lea con su tool Read y
	// (opcional) los importe como project_policies con
	// domain_project_policy_import_from_text.
	rulesFiles := []string{}
	if v, ok := args["existing_rules_files"].([]any); ok {
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				rulesFiles = append(rulesFiles, s)
			}
		}
	}

	projID, projSlug, matched := h.matchProjectFromCwdOrRemote(ctx, orgID, cwd, gitRemote)
	if !matched {
		suggestionSlug := strings.ToLower(filepath.Base(strings.TrimRight(cwd, "/")))
		resp := map[string]any{
			"known": false,
			"suggestion": map[string]any{
				"slug":            suggestionSlug,
				"remote_detected": gitRemote,
				"branch_detected": gitBranch,
				"cwd":             cwd,
			},
			"existing_rules_files": rulesFiles,
			"next_step":            "Preguntale al usuario: (1) confirmá slug='" + suggestionSlug + "' o pasame otro; (2) confirmá remote=" + gitRemote + " (origin) o pasame otro; (3) ¿hay otros remotos (mirror/upstream)?; (4) qué workflow usan (pr/mr/merge/trunk_based); (5) ¿algo crítico sobre estructura (mono-repo, multi-servicio, migrations manuales, stack)? Después llamá domain_session_register.",
		}
		if len(rulesFiles) > 0 {
			resp["suggested_imports_note"] = "Detecté " + fmt.Sprintf("%d", len(rulesFiles)) + " archivos AI-rules en el repo. Después de registrar el proyecto, leelos con tu tool Read y llamá domain_project_policy_import_from_text por cada uno para que domain herede lo que el repo ya documenta sin pisar nada."
		}
		return toolResultJSON(resp)
	}

	// Proyecto conocido: leer estado previo + last_known_head
	var (
		lastHead   *string
		lastBranch *string
		lastCwd    *string
		lastSeen   *string
		name       string
		desc       *string
	)
	err := h.q(ctx).QueryRow(ctx,
		`SELECT name, description, last_known_head, last_seen_branch,
		        last_seen_cwd, to_char(last_seen_at AT TIME ZONE 'UTC',
		                               'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		   FROM projects
		   WHERE id = $1`,
		projID,
	).Scan(&name, &desc, &lastHead, &lastBranch, &lastCwd, &lastSeen)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read project state: %v", err)), nil
	}

	headChanged := false
	if lastHead != nil && *lastHead != "" && gitHead != "" && *lastHead != gitHead {
		headChanged = true
	}

	// Bump last_seen_*
	if _, err := h.q(ctx).Exec(ctx,
		`UPDATE projects
		   SET last_known_head  = COALESCE(NULLIF($2,''), last_known_head),
		       last_seen_branch = COALESCE(NULLIF($3,''), last_seen_branch),
		       last_seen_cwd    = COALESCE(NULLIF($4,''), last_seen_cwd),
		       last_seen_at     = NOW()
		   WHERE id = $1`,
		projID, gitHead, gitBranch, cwd,
	); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("update last_seen: %v", err)), nil
	}

	// Recent observations (top 5). observations no tiene `title` ni `type`
	// — usa observation_type + content (truncamos a 80 chars para preview).
	// CRÍTICO: cerrar rows EXPLÍCITO antes de las siguientes queries dentro
	// de la misma tx; pgx no permite queries concurrentes en una sola tx,
	// y un `defer rows.Close()` al final del handler dejaría las queries
	// posteriores en estado "conn busy" → fallan silenciosamente con
	// nuestros `if err == nil` y los counts quedan en 0.
	recentObs := []map[string]any{}
	rows, qerr := h.q(ctx).Query(ctx,
		`SELECT id::text, observation_type,
		        substring(content from 1 for 80),
		        to_char(created_at AT TIME ZONE 'UTC',
		                'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		   FROM knowledge_observations
		   WHERE project_id = $1
		     AND deleted_at IS NULL
		   ORDER BY created_at DESC LIMIT 5`,
		projID,
	)
	if qerr == nil {
		for rows.Next() {
			var id, otype, preview, createdAt string
			if err := rows.Scan(&id, &otype, &preview, &createdAt); err == nil {
				recentObs = append(recentObs, map[string]any{
					"id": id, "type": otype, "preview": preview, "created_at": createdAt,
				})
			}
		}
		rows.Close() // explícito ANTES de las próximas queries en la misma tx
	}

	// Counts policies + project_repos. COUNT(*) en pg devuelve bigint;
	// usar int64 para evitar scan mismatch silencioso.
	var (
		projPoliciesCount     int64
		platformPoliciesCount int
		projRepoCount         int64
	)
	if err := h.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM project_policies
		   WHERE project_id = $1
		     AND is_active = TRUE AND deleted_at IS NULL`,
		projID,
	).Scan(&projPoliciesCount); err != nil {
		_ = err // count opcional, no abortar el bootstrap
	}

	// platform_policies no tiene RLS — usa el service.
	if h.policies != nil {
		if pols, perr := h.policies.List(ctx, ""); perr == nil {
			platformPoliciesCount = len(pols)
		}
	}

	if err := h.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM project_repositories
		   WHERE project_id = $1 AND deleted_at IS NULL`,
		projID,
	).Scan(&projRepoCount); err != nil {
		_ = err
	}

	var projectSkillCount int64
	if err := h.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM skills
		   WHERE project_id = $1 AND deleted_at IS NULL`,
		projID,
	).Scan(&projectSkillCount); err != nil {
		_ = err
	}

	// Resumen de trabajo pendiente: tickets/issues abiertos + flow_run en curso.
	// Solo counts y un QueryRow del último flow_run activo — sin rows abiertos
	// concurrentes (mismo cuidado que recent_observations en la misma tx).
	var openTickets, openIssues int64
	_ = h.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM project_tickets
		   WHERE project_id = $1 AND deleted_at IS NULL
		     AND status NOT IN ('done','cancelled')`,
		projID,
	).Scan(&openTickets)
	_ = h.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM issues
		   WHERE project_id = $1
		     AND status IN ('proposed','active')`,
		projID,
	).Scan(&openIssues)

	// flow_run en curso (running o pausado esperando algo): lo más relevante
	// para "retomar lo último". NULL si no hay tarea SDD a medias.
	var (
		activeRunID     *string
		activeRunStatus *string
		activeRunAt     *string
	)
	_ = h.q(ctx).QueryRow(ctx,
		`SELECT id::text, status,
		        to_char(COALESCE(started_at, created_at) AT TIME ZONE 'UTC',
		                'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		   FROM flow_runs
		   WHERE project_id = $1
		     AND status IN ('running','paused','paused_awaiting_signal','paused_awaiting_human')
		   ORDER BY created_at DESC LIMIT 1`,
		projID,
	).Scan(&activeRunID, &activeRunStatus, &activeRunAt)

	activeRun := map[string]any(nil)
	if activeRunID != nil {
		activeRun = map[string]any{
			"id":         safeDeref(activeRunID),
			"status":     safeDeref(activeRunStatus),
			"started_at": safeDeref(activeRunAt),
		}
	}

	// Estado del grafo de CÓDIGO: ¿existe? ¿está desactualizado respecto al
	// HEAD actual? Esto NO dispara un build (sería caro y bloqueante en el
	// primer turn: walk del filesystem + parse AST + writes masivos). Solo
	// detecta staleness comparando el git_head con el que quedó registrado en
	// code_index_files y devuelve una sugerencia accionable. El build sigue
	// siendo explícito vía domain_code_build.
	codeGraph := h.codeGraphStaleness(ctx, projID, gitHead)

	nextStep := "OK — proyecto conocido. Antes de actuar: si head_changed=true, considerá leer git log <last_known_head>..<current_head> y refrescar memorias relevantes con domain_mem_save. Llamá domain_policy_list o domain_project_policy_list según corresponda."
	if headChanged {
		nextStep = "HEAD cambió desde la última sesión. Ejecutá `git log --oneline " + safeDeref(lastHead) + ".." + gitHead + "` para ver qué cambió; si hay decisiones/bugfixes nuevos, persistilos con domain_mem_save antes de seguir."
	}
	if activeRun != nil {
		nextStep = "Hay un flow_run SDD en estado '" + safeDeref(activeRunStatus) + "' — quedó una tarea a medias. Llamá domain_orchestrate_status para ver en qué fase está y retomá; o si el usuario ordena suspenderla/archivarla, cambiá su estado en vez de reiniciar. " + nextStep
	}
	if cgSug, _ := codeGraph["suggestion"].(string); cgSug != "" {
		nextStep = cgSug + " " + nextStep
	}

	// REQ-55 issue-55.4: candidatos de auto-import. Comparamos los
	// existing_rules_files reportados por el cliente contra las policies ya
	// importadas (slug determinístico imported-<path>). Los que faltan son
	// CANDIDATOS: el agente los lee, el server los clasifica, y se presentan al
	// usuario con AskUserQuestion (confirmar/scope) antes de persistir. Esto
	// hace VISIBLE el auto-skill/policy en vez de depender de que el agente
	// recuerde el protocolo. Best-effort: si la query falla, candidatos vacío.
	importCandidates := h.importCandidates(ctx, projID, rulesFiles)
	if len(importCandidates) > 0 {
		nextStep = "Hay " + fmt.Sprintf("%d", len(importCandidates)) +
			" archivo(s) AI-rules del repo SIN importar a domain (ver import_candidates). " +
			"Leelos, y proponé al usuario con AskUserQuestion cuáles importar como " +
			"project_policy (o platform si aplica a toda la org) antes de persistir. " + nextStep
	}

	return toolResultJSON(map[string]any{
		"known": true,
		"project": map[string]any{
			"id":          projID.String(),
			"slug":        projSlug,
			"name":        name,
			"description": safeDeref(desc),
		},
		"head": map[string]any{
			"last_known":   safeDeref(lastHead),
			"current":      gitHead,
			"changed":      headChanged,
			"last_branch":  safeDeref(lastBranch),
			"last_cwd":     safeDeref(lastCwd),
			"last_seen_at": safeDeref(lastSeen),
		},
		"recent_observations":  recentObs,
		"existing_rules_files": rulesFiles,
		"import_candidates":    importCandidates,
		"counts": map[string]any{
			"project_policies":    projPoliciesCount,
			"platform_policies":   platformPoliciesCount,
			"project_repos":       projRepoCount,
			"existing_rules":      len(rulesFiles),
			"project_skill_count": projectSkillCount,
		},
		// Mini-resumen de "dónde quedamos": pendientes + tarea SDD en curso.
		// El LLM lo usa para abrir la sesión con contexto sin pedir nada más.
		"work_summary": map[string]any{
			"open_tickets":    openTickets,
			"open_issues":     openIssues,
			"active_flow_run": activeRun,
		},
		// Estado del grafo de código (existe / stale / git_head) + sugerencia.
		// El client decide si correr domain_code_build; no se dispara acá.
		"code_graph": codeGraph,
		"next_step":  nextStep,
	})
}

// codeGraphStaleness inspecciona el estado del grafo de CÓDIGO del project
// (tablas code_index_files / mig 000178) y devuelve un mapa con:
//   - built: ¿hay al menos un archivo indexado?
//   - indexed_files: cuántos archivos hay en el índice.
//   - indexed_head: el git_head con el que se construyó el grafo (el del último
//     archivo indexado; en una corrida normal de domain_code_build todos los
//     archivos comparten el mismo HEAD).
//   - current_head: el git_head reportado por el client en este bootstrap.
//   - stale: true si current_head != indexed_head (ambos no vacíos) → el grafo
//     no refleja el árbol actual.
//   - last_indexed_at: timestamp del último indexado.
//   - suggestion: texto accionable (prefijo del next_step) o "" si está fresco.
//
// NO dispara un build: detectar staleness es barato (un solo QueryRow), pero un
// rebuild incremental (walk del filesystem + parse AST + writes) sería caro y
// bloqueante en el primer turn de la sesión. El build sigue siendo explícito
// vía domain_code_build; acá solo sugerimos correrlo cuando hace falta.
//
// importCandidates devuelve los existing_rules_files que AÚN NO tienen una
// project_policy importada (slug determinístico imported-<path>). Son los
// candidatos que el agente debe proponer al usuario para importar (REQ-55
// issue-55.4). Best-effort: si la query falla, devuelve nil (sin candidatos)
// para no bloquear el bootstrap.
func (h *sessionBootstrapHandlers) importCandidates(ctx context.Context, projID uuid.UUID, rulesFiles []string) []map[string]any {
	if len(rulesFiles) == 0 {
		return nil
	}
	// Slugs de policies ya importadas para este proyecto.
	imported := map[string]bool{}
	rows, err := h.q(ctx).Query(ctx,
		`SELECT slug FROM project_policies
		   WHERE project_id = $1 AND source = 'seed_imported'
		     AND is_active = TRUE AND deleted_at IS NULL`,
		projID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var slug string
			if rows.Scan(&slug) == nil {
				imported[slug] = true
			}
		}
	}
	var candidates []map[string]any
	for _, f := range rulesFiles {
		// openspec/ es un directorio de specs, no una policy importable directa.
		if f == "openspec/" {
			continue
		}
		if imported[slugFromSourcePath(f)] {
			continue
		}
		candidates = append(candidates, map[string]any{
			"path":            f,
			"suggested_slug":  slugFromSourcePath(f),
			"suggested_kind":  "convention",
			"suggested_scope": "project",
		})
	}
	return candidates
}

// Single-tenant: filtra exclusivamente por project_id. Best-effort: si la query
// falla (p.ej. el grafo nunca se usó), reporta built=false sin abortar el
// bootstrap.
func (h *sessionBootstrapHandlers) codeGraphStaleness(ctx context.Context, projID uuid.UUID, currentHead string) map[string]any {
	out := map[string]any{
		"built":        false,
		"current_head": currentHead,
	}

	var (
		fileCount   int64
		indexedHead *string
		lastIndexed *string
	)
	// MAX(indexed_at) + el git_head de la fila más reciente. Un único QueryRow:
	// sin rows abiertos concurrentes en la tx (mismo cuidado que el resto del
	// handler).
	err := h.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*),
		        (SELECT git_head FROM code_index_files
		           WHERE project_id = $1
		           ORDER BY indexed_at DESC LIMIT 1),
		        to_char(MAX(indexed_at) AT TIME ZONE 'UTC',
		                'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		   FROM code_index_files
		   WHERE project_id = $1`,
		projID,
	).Scan(&fileCount, &indexedHead, &lastIndexed)
	if err != nil {
		// Grafo inexistente o no consultable: sugerir construirlo una vez.
		out["suggestion"] = "El grafo de código de este proyecto todavía no existe. Cuando vayas a navegar el código, corré domain_code_build (root_path = cwd) para poder usar domain_code_explore/path/graph."
		return out
	}

	out["indexed_files"] = fileCount
	out["indexed_head"] = safeDeref(indexedHead)
	out["last_indexed_at"] = safeDeref(lastIndexed)

	if fileCount == 0 {
		out["suggestion"] = "El grafo de código de este proyecto todavía no existe. Cuando vayas a navegar el código, corré domain_code_build (root_path = cwd) para poder usar domain_code_explore/path/graph."
		return out
	}

	out["built"] = true

	stale := codeGraphIsStale(safeDeref(indexedHead), currentHead)
	out["stale"] = stale
	if stale {
		out["suggestion"] = "El grafo de código está desactualizado (se construyó en " + safeDeref(indexedHead) + " y el HEAD actual es " + currentHead + "). Si vas a navegar el código, corré domain_code_build (root_path = cwd) para refrescarlo incrementalmente."
	}
	return out
}

// codeGraphIsStale decide si el grafo está desactualizado: lo está cuando el
// HEAD con el que se construyó y el HEAD actual son ambos conocidos y distintos.
// Si alguno es vacío (el client no mandó git_head, o el índice no guardó uno),
// NO se marca stale: sin la info no se puede afirmar desactualización, y marcar
// stale a ciegas generaría ruido (sugerir rebuild en cada bootstrap).
func codeGraphIsStale(indexedHead, currentHead string) bool {
	return indexedHead != "" && currentHead != "" && indexedHead != currentHead
}

func safeDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// sanitizeSlug deriva un slug válido (kebab-case, empieza con letra) desde un
// texto libre (ej. basename del cwd). Matchea el patrón de projects.slug.
func sanitizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case !prevDash && b.Len() > 0:
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "proyecto"
	}

	if out[0] >= '0' && out[0] <= '9' {
		out = "p-" + out
	}
	if len(out) > 100 {
		out = strings.Trim(out[:100], "-")
	}
	return out
}

func (h *sessionBootstrapHandlers) handleSessionRegister(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.projects == nil || h.pool == nil {
		return mcp.NewToolResultError("projects service / pool not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	if slug == "" || name == "" {
		return mcp.NewToolResultError("slug y name requeridos"), nil
	}

	// Idempotencia: si slug ya existe, devolver known=true en lugar de duplicar.
	if existing, _ := h.projects.GetBySlug(ctx, orgID, slug); existing != nil {
		return toolResultJSON(map[string]any{
			"known":   true,
			"project": existing,
			"note":    "El slug ya existía; se reutiliza en lugar de duplicar.",
		})
	}

	remoteURL, _ := args["remote_url"].(string)
	gitHead, _ := args["git_head"].(string)
	gitBranch, _ := args["git_branch"].(string)
	cwd, _ := args["cwd"].(string)
	desc, _ := args["description"].(string)

	// Crear project con INSERT directo (Service.Create requiere muchos campos
	// que no aplican acá — slug/name/description bastan para bootstrap).
	var projID uuid.UUID
	err := h.q(ctx).QueryRow(ctx,
		`INSERT INTO projects
		   (slug, name, description,
		    repository_url, last_known_head, last_seen_branch,
		    last_seen_cwd, last_seen_at)
		 VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),
		         NULLIF($6,''),NULLIF($7,''),NOW())
		 RETURNING id`,
		slug, name, desc, remoteURL, gitHead, gitBranch, cwd,
	).Scan(&projID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create project failed: %v", err)), nil
	}

	// Si hay remote_url, registrarlo en project_repositories.
	var createdRepo *projectreposvc.Repo
	if remoteURL != "" && h.projectRepos != nil {
		remoteName, _ := args["remote_name"].(string)
		if remoteName == "" {
			remoteName = "origin"
		}
		repoIn := projectreposvc.AddInput{
			ProjectID: projID,
			Name:      remoteName,
			URL:       remoteURL,
		}
		if v, _ := args["branch_default"].(string); v != "" {
			repoIn.BranchDefault = v
		} else if gitBranch != "" {
			repoIn.BranchDefault = gitBranch
		}
		if v, _ := args["kind"].(string); v != "" {
			repoIn.Kind = v
		}
		if v, _ := args["workflow"].(string); v != "" {
			repoIn.Workflow = v
		}
		if r, rerr := h.projectRepos.Add(ctx, repoIn); rerr == nil {
			createdRepo = r
		}
	}

	// Persistir observación inicial de registro (audit).
	// observations schema: (project_id, observation_type,
	// content). No tiene title — lo embebemos en content como first-line.
	if _, oerr := h.q(ctx).Exec(ctx,
		`INSERT INTO knowledge_observations
		   (project_id, observation_type, content, tags)
		 VALUES ($1, 'discovery', $2, ARRAY['bootstrap','project_registered'])`,
		projID,
		fmt.Sprintf("# Proyecto registrado vía session_bootstrap\n\nProyecto %s creado desde sesión MCP.\n- cwd: %s\n- remote: %s\n- branch: %s\n- head: %s",
			slug, cwd, remoteURL, gitBranch, gitHead),
	); oerr != nil {
		// Log silencioso: el insert de observation NO debe abortar el registro
		// del proyecto. Si falla, seguimos — la tx no se aborta porque el caller
		// quiere que el proyecto quede creado igual.
		_ = oerr
	}

	// Refetch para devolver el project completo.
	proj, _ := h.projects.GetByID(ctx, projID)
	// REQ-56 issue-56.3: al registrar, el proyecto queda "conocido" pero SIN grafo
	// de código. El server no tiene el FS del cliente (por eso no corre code_build
	// acá), así que emite una señal explícita para que el cliente encadene el build
	// del code_graph + project index apenas registra, en vez de dejarlo manual.
	return toolResultJSON(map[string]any{
		"known":   false, // recién creado
		"created": true,
		"project": proj,
		"repo":    createdRepo,
		"next_action": map[string]any{
			"kind":   "build_code_graph",
			"reason": "proyecto recién registrado sin grafo de código; el cliente debe construirlo",
			"hint":   "correr el script cliente domain-code-graph.sh <cwd> <slug> (el server no tiene FS del cliente)",
			"slug":   slug,
			"cwd":    cwd,
		},
	})
}
