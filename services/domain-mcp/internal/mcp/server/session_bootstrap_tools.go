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
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	projectreposvc "nunezlagos/domain/internal/service/projectrepo"
)

func registerSessionBootstrapTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	rls := func(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, h)
	}
	return []mcpgo.ServerTool{
		{Tool: toolSessionBootstrap(), Handler: wrap.Wrap("domain_session_bootstrap", rls(deps.handleSessionBootstrap))},
		{Tool: toolSessionRegister(), Handler: wrap.Wrap("domain_session_register", rls(deps.handleSessionRegister))},
	}
}

func toolSessionBootstrap() mcp.Tool {
	return mcp.NewTool("domain_session_bootstrap",
		mcp.WithDescription("Llamar al PRIMER turn de cada sesión nueva. Manda cwd + git_remote + git_branch + git_head + (opcional) existing_rules_files al server. Devuelve overlay con datos del proyecto (si es conocido) o un cuestionario para registrarlo (si no). El client debe seguir el next_step indicado."),
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
func (d *Deps) matchProjectFromCwdOrRemote(ctx context.Context, orgID uuid.UUID, cwd, gitRemote string) (uuid.UUID, string, bool) {
	// 1. project_repositories.url
	if gitRemote != "" {
		var pid uuid.UUID
		var pslug string
		err := d.q(ctx).QueryRow(ctx,
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
		err := d.q(ctx).QueryRow(ctx,
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
		err := d.q(ctx).QueryRow(ctx,
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

func (d *Deps) handleSessionBootstrap(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.Pool == nil {
		return mcp.NewToolResultError("pool not configured"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
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

	projID, projSlug, matched := d.matchProjectFromCwdOrRemote(ctx, orgID, cwd, gitRemote)
	if !matched {
		// El project_id se resuelve SIEMPRE: si no hay match, se auto-crea desde
		// el git de la sesión y se enlazan las skills de plataforma. Si el alta
		// falla, caemos al cuestionario manual (comportamiento previo).
		newID, newSlug, cerr := d.autoCreateProject(ctx, orgID, cwd, gitRemote, gitBranch, gitHead)
		if cerr != nil {
			suggestionSlug := sanitizeSlug(filepath.Base(strings.TrimRight(cwd, "/")))
			resp := map[string]any{
				"known":              false,
				"auto_create_failed": cerr.Error(),
				"suggestion": map[string]any{
					"slug":            suggestionSlug,
					"remote_detected": gitRemote,
					"branch_detected": gitBranch,
					"cwd":             cwd,
				},
				"existing_rules_files": rulesFiles,
				"next_step":            "No se pudo auto-crear el proyecto. Consulte al usuario slug/remote/workflow y llame domain_session_register.",
			}
			return toolResultJSON(resp)
		}
		linked, _ := d.linkPlatformSkills(ctx, newID)
		resp := map[string]any{
			"known":        true,
			"auto_created": true,
			"project": map[string]any{
				"id":   newID.String(),
				"slug": newSlug,
				"name": newSlug,
			},
			"linked_skills":        linked,
			"existing_rules_files": rulesFiles,
			"next_step":            fmt.Sprintf("Proyecto auto-creado (slug=%s) con %d skills de plataforma enlazadas. Use project_id=%s en domain_prompt/domain_orchestrate. Si hay archivos AI-rules detectados, importelos con domain_project_policy_import_from_text.", newSlug, linked, newID.String()),
		}
		if len(rulesFiles) > 0 {
			resp["suggested_imports_note"] = "Se detectaron " + fmt.Sprintf("%d", len(rulesFiles)) + " archivos AI-rules en el repo. Lealos con la tool Read y llame domain_project_policy_import_from_text por cada uno."
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
	err := d.q(ctx).QueryRow(ctx,
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
	if _, err := d.q(ctx).Exec(ctx,
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
	rows, qerr := d.q(ctx).Query(ctx,
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
	if err := d.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM project_policies
		   WHERE project_id = $1
		     AND is_active = TRUE AND deleted_at IS NULL`,
		projID,
	).Scan(&projPoliciesCount); err != nil {
		_ = err // count opcional, no abortar el bootstrap
	}

	// platform_policies no tiene RLS — usa el service.
	if d.Policies != nil {
		if pols, perr := d.Policies.List(ctx, ""); perr == nil {
			platformPoliciesCount = len(pols)
		}
	}

	if err := d.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM project_repositories
		   WHERE project_id = $1 AND deleted_at IS NULL`,
		projID,
	).Scan(&projRepoCount); err != nil {
		_ = err
	}

	nextStep := "OK — proyecto conocido. Antes de actuar: si head_changed=true, considerá leer git log <last_known_head>..<current_head> y refrescar memorias relevantes con domain_mem_save. Llamá domain_policy_list o domain_project_policy_list según corresponda."
	if headChanged {
		nextStep = "HEAD cambió desde la última sesión. Ejecutá `git log --oneline " + safeDeref(lastHead) + ".." + gitHead + "` para ver qué cambió; si hay decisiones/bugfixes nuevos, persistilos con domain_mem_save antes de seguir."
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
		"counts": map[string]any{
			"project_policies":  projPoliciesCount,
			"platform_policies": platformPoliciesCount,
			"project_repos":     projRepoCount,
			"existing_rules":    len(rulesFiles),
		},
		"next_step": nextStep,
	})
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
	// El slug debe empezar con letra (regex de projects.slug).
	if out[0] >= '0' && out[0] <= '9' {
		out = "p-" + out
	}
	if len(out) > 100 {
		out = strings.Trim(out[:100], "-")
	}
	return out
}

// autoCreateProject crea un proyecto desde el contexto git de la sesión cuando
// el bootstrap no encontró match (decisión: el project_id se resuelve SIEMPRE;
// si no existe, se crea). Idempotente por slug. Corre dentro de la tx del
// handler (d.q usa la tx). Registra el remoto y deja una observación.
func (d *Deps) autoCreateProject(ctx context.Context, orgID uuid.UUID, cwd, gitRemote, gitBranch, gitHead string) (uuid.UUID, string, error) {
	slug := sanitizeSlug(filepath.Base(strings.TrimRight(cwd, "/")))

	// Idempotencia: si el slug ya existe (no matcheado por remote/basename),
	// reusarlo en vez de fallar por UNIQUE.
	if d.Projects != nil {
		if existing, _ := d.Projects.GetBySlug(ctx, orgID, slug); existing != nil {
			return existing.ID, existing.Slug, nil
		}
	}

	var projID uuid.UUID
	if err := d.q(ctx).QueryRow(ctx,
		`INSERT INTO projects
		   (slug, name, description, repository_url,
		    last_known_head, last_seen_branch, last_seen_cwd, last_seen_at)
		 VALUES ($1,$2,'',NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NOW())
		 RETURNING id`,
		slug, slug, gitRemote, gitHead, gitBranch, cwd,
	).Scan(&projID); err != nil {
		return uuid.Nil, "", fmt.Errorf("insert project: %w", err)
	}

	// Registrar el remoto detectado (best-effort, no aborta el alta).
	if gitRemote != "" && d.ProjectRepos != nil {
		_, _ = d.ProjectRepos.Add(ctx, projectreposvc.AddInput{
			ProjectID:     projID,
			Name:          "origin",
			URL:           gitRemote,
			BranchDefault: gitBranch,
		})
	}

	// Observación de auditoría (best-effort).
	_, _ = d.q(ctx).Exec(ctx,
		`INSERT INTO knowledge_observations (project_id, observation_type, content, tags)
		 VALUES ($1, 'discovery', $2, ARRAY['bootstrap','auto_created'])`,
		projID,
		fmt.Sprintf("# Proyecto auto-creado vía session_bootstrap\n\nslug: %s\ncwd: %s\nremote: %s\nbranch: %s",
			slug, cwd, gitRemote, gitBranch),
	)

	return projID, slug, nil
}

// linkPlatformSkills enlaza todas las skills de plataforma (globales, sin
// project_id) al proyecto, vía project_skills. Es el "auto-enlace al crear"
// (decisión del usuario): un proyecto nuevo arranca con el catálogo de
// plataforma habilitado; después se desenlaza lo que no se use. Idempotente.
func (d *Deps) linkPlatformSkills(ctx context.Context, projID uuid.UUID) (int64, error) {
	tag, err := d.q(ctx).Exec(ctx,
		`INSERT INTO project_skills (project_id, skill_id)
		   SELECT $1, id FROM skills
		   WHERE project_id IS NULL AND deleted_at IS NULL AND proposed = FALSE
		 ON CONFLICT (project_id, skill_id) DO NOTHING`,
		projID,
	)
	if err != nil {
		return 0, fmt.Errorf("link platform skills: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (d *Deps) handleSessionRegister(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.Projects == nil || d.Pool == nil {
		return mcp.NewToolResultError("projects service / pool not configured"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	if slug == "" || name == "" {
		return mcp.NewToolResultError("slug y name requeridos"), nil
	}

	// Idempotencia: si slug ya existe, devolver known=true en lugar de duplicar.
	if existing, _ := d.Projects.GetBySlug(ctx, orgID, slug); existing != nil {
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
	err := d.q(ctx).QueryRow(ctx,
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
	if remoteURL != "" && d.ProjectRepos != nil {
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
		if r, rerr := d.ProjectRepos.Add(ctx, repoIn); rerr == nil {
			createdRepo = r
		}
	}

	// Persistir observación inicial de registro (audit).
	// observations schema: (project_id, observation_type,
	// content). No tiene title — lo embebemos en content como first-line.
	if _, oerr := d.q(ctx).Exec(ctx,
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
	proj, _ := d.Projects.GetByID(ctx, projID)
	return toolResultJSON(map[string]any{
		"known":   false, // recién creado
		"created": true,
		"project": proj,
		"repo":    createdRepo,
	})
}
