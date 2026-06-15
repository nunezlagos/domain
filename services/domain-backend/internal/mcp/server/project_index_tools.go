// REQ-62 — Project indexer estilo Cursor.
//
// Flow:
//
//   1. LLM: domain_project_index_start(project_slug, git_head, force?)
//      → server crea project_index_runs(status=running) + devuelve un
//        manifest de path patterns que el LLM debe leer del repo
//        (AGENTS.md, README.md, .claude/CLAUDE.md, package.json, etc).
//
//   2. LLM lee cada archivo del manifest con su tool nativa Read y los
//      submitéa con domain_project_index_submit(run_id, files[]).
//      Server clasifica cada archivo:
//        - AGENTS.md / CLAUDE.md / .claude/CLAUDE.md → project_policy
//          (kind=agent_protocol, slug=imported-<name>, source=seed_imported)
//        - .claude/rules/*.md → project_policy (kind=convention)
//        - README.md → knowledge_doc (category=readme)
//        - docs/**.md, doc/*.md → knowledge_doc (category=docs)
//        - package.json/go.mod/composer.json/pyproject.toml → project_policy
//          (kind=tech_stack) extrayendo nombre del lenguaje + version
//        - Makefile / Taskfile.yml → project_policy (kind=convention)
//          listando los targets como "comandos comunes del proyecto"
//        - .github/workflows/*.yml → knowledge_doc (category=ci)
//
//   3. domain_project_index_status(project_slug) → último run + counts.
//
// Aspecto Cursor-like: una sola call al LLM "indexar este proyecto" deja
// persistido todo el contexto del repo en project_policies + knowledge.
// Después domain_policy_get, mem_search etc lo recuperan sin volver al fs.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	projectpolicysvc "nunezlagos/domain/internal/service/projectpolicy"
)

func registerProjectIndexTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	rls := func(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, h)
	}
	return []mcpgo.ServerTool{
		{Tool: toolProjectIndexStart(), Handler: wrap.Wrap("domain_project_index_start", rls(deps.handleProjectIndexStart))},
		{Tool: toolProjectIndexSubmit(), Handler: wrap.Wrap("domain_project_index_submit", rls(deps.handleProjectIndexSubmit))},
		{Tool: toolProjectIndexStatus(), Handler: wrap.Wrap("domain_project_index_status", rls(deps.handleProjectIndexStatus))},
	}
}

func toolProjectIndexStart() mcp.Tool {
	return mcp.NewTool("domain_project_index_start",
		mcp.WithDescription("Inicia un indexing run del proyecto. Server devuelve un manifest de paths/patterns relevantes que vos (LLM) tenés que leer del repo con tu tool Read nativa. Después submitea con domain_project_index_submit. Resultado: docs/conventions/stack del repo quedan persistidos como project_policies + knowledge_docs para que futuras sesiones los tengan en BD sin volver al filesystem (RAG del proyecto)."),
		mcp.WithString("project_slug", mcp.Description("Proyecto a indexar"), mcp.Required()),
		mcp.WithString("git_head", mcp.Description("SHA1 del HEAD actual. Opcional pero recomendado para audit.")),
		mcp.WithBoolean("force", mcp.Description("Re-indexar aunque haya run reciente. Default false.")),
	)
}

func toolProjectIndexSubmit() mcp.Tool {
	return mcp.NewTool("domain_project_index_submit",
		mcp.WithDescription("Submitea N archivos del repo al server para clasificación + persistencia. Cada archivo: {path, content}. El server clasifica el archivo según su path/contenido (AGENTS.md→policy, README→knowledge, package.json→tech_stack, Makefile→comandos comunes, etc) y persiste en la tabla apropiada con source='seed_imported'. NO modifica el archivo original. Idempotente: re-submitear el mismo path actualiza la versión."),
		mcp.WithString("run_id", mcp.Description("UUID del run devuelto por start"), mcp.Required()),
		mcp.WithArray("files", mcp.Description("Array de {path: 'relativo al repo', content: 'texto del archivo'}"), mcp.Required()),
		mcp.WithBoolean("complete", mcp.Description("Si true, marca el run como completed. Si false, queda running para más submits. Default: false (último submit debe pasar complete=true).")),
	)
}

func toolProjectIndexStatus() mcp.Tool {
	return mcp.NewTool("domain_project_index_status",
		mcp.WithDescription("Devuelve el último indexing run del proyecto + summary (counts de qué se persistió). Útil para saber si vale re-indexar (last run hace >7 días o git_head distinto) o si ya está fresco."),
		mcp.WithString("project_slug", mcp.Description("Proyecto a consultar"), mcp.Required()),
	)
}

// --- handlers ---

func (d *Deps) handleProjectIndexStart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Projects == nil {
		return mcp.NewToolResultError("principal o projects service no configurado"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)
	args := req.GetArguments()
	slug, _ := args["project_slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("project_slug requerido"), nil
	}
	proj, err := d.Projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
	}
	gitHead, _ := args["git_head"].(string)

	var runID uuid.UUID
	if err := d.q(ctx).QueryRow(ctx,
		`INSERT INTO project_index_runs
		   (organization_id, project_id, started_by, git_head)
		 VALUES ($1,$2,$3,NULLIF($4,''))
		 RETURNING id`,
		orgID, proj.ID, userID, gitHead,
	).Scan(&runID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create run failed: %v", err)), nil
	}

	manifest := buildIndexManifest()
	return toolResultJSON(map[string]any{
		"run_id":      runID.String(),
		"project_id":  proj.ID.String(),
		"project_slug": slug,
		"manifest":    manifest,
		"next_step":   "Leé con tu tool Read los archivos que matcheen los patterns del manifest. Después llamá domain_project_index_submit con un batch de {path, content}. En el último batch pasá complete=true para cerrar el run.",
	})
}

// buildIndexManifest: paths/patterns ordenados por prioridad (los más
// informativos primero). El LLM no necesita leer TODO — leer lo que
// encuentre.
func buildIndexManifest() []map[string]any {
	return []map[string]any{
		{"category": "agent_protocol", "patterns": []string{
			"AGENTS.md", "CLAUDE.md", ".claude/CLAUDE.md", "AGENT.md",
		}, "priority": "high"},
		{"category": "rules", "patterns": []string{
			".claude/rules/*.md", ".cursorrules", ".cursor/rules/*.mdc",
			".windsurf/rules/*", ".github/copilot-instructions.md",
		}, "priority": "high"},
		{"category": "readme_root", "patterns": []string{
			"README.md", "README.MD",
		}, "priority": "high"},
		{"category": "stack", "patterns": []string{
			"package.json", "go.mod", "composer.json", "pyproject.toml",
			"Cargo.toml", "Gemfile", "build.gradle", "pom.xml",
		}, "priority": "high"},
		{"category": "commands", "patterns": []string{
			"Makefile", "Taskfile.yml", "justfile", "package.json (scripts section)",
		}, "priority": "medium"},
		{"category": "docs", "patterns": []string{
			"docs/**.md", "doc/**.md", "documentation/**.md",
		}, "priority": "medium"},
		{"category": "ci", "patterns": []string{
			".github/workflows/*.yml", ".gitlab-ci.yml", ".circleci/config.yml",
		}, "priority": "low"},
		{"category": "openspec", "patterns": []string{
			"openspec/changes/*/proposal.md",
			"openspec/changes/*/design.md",
		}, "priority": "low"},
	}
}

// classifyFile: dado path + content, retorna (category, kind, slug, body).
// category es "policy", "knowledge", "skill" o "" (ignored).
type classifiedFile struct {
	Category string // policy | knowledge | skill | ignored
	Kind     string // sub-kind dentro de category
	Slug     string
	Title    string
	Body     string
}

func classifyFile(path, content string) classifiedFile {
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	low := strings.ToLower(base)

	// Agent protocol files
	if low == "agents.md" || low == "claude.md" || low == "agent.md" ||
		(strings.HasPrefix(path, ".claude/") && low == "claude.md") {
		return classifiedFile{
			Category: "policy", Kind: "agent_protocol",
			Slug: "imported-" + strings.TrimSuffix(strings.ToLower(strings.ReplaceAll(path, "/", "-")), ".md"),
			Title: "Imported: " + path, Body: content,
		}
	}
	// Rules files
	if strings.HasPrefix(path, ".claude/rules/") && strings.HasSuffix(low, ".md") {
		return classifiedFile{
			Category: "policy", Kind: "convention",
			Slug: "imported-rules-" + strings.TrimSuffix(low, ".md"),
			Title: "Rule: " + base, Body: content,
		}
	}
	if base == ".cursorrules" || strings.HasPrefix(path, ".cursor/rules/") ||
		strings.HasPrefix(path, ".windsurf/rules/") ||
		path == ".github/copilot-instructions.md" {
		return classifiedFile{
			Category: "policy", Kind: "convention",
			Slug: "imported-" + strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(path, "."), "/", "-")),
			Title: "Imported: " + path, Body: content,
		}
	}
	// README
	if strings.EqualFold(base, "README.md") && (dir == "." || dir == "" || dir == "/") {
		return classifiedFile{
			Category: "knowledge", Kind: "readme",
			Slug: "readme", Title: "README", Body: content,
		}
	}
	// Tech stack manifests
	if base == "package.json" || base == "go.mod" || base == "composer.json" ||
		base == "pyproject.toml" || base == "Cargo.toml" || base == "Gemfile" {
		return classifiedFile{
			Category: "policy", Kind: "tech_stack",
			Slug:  "imported-stack-" + strings.TrimSuffix(low, filepath.Ext(low)),
			Title: "Tech stack: " + base,
			Body:  techStackSummary(base, content),
		}
	}
	// Commands
	if base == "Makefile" || base == "makefile" || base == "Taskfile.yml" || base == "justfile" {
		return classifiedFile{
			Category: "policy", Kind: "convention",
			Slug: "imported-commands-" + strings.ToLower(base),
			Title: "Comandos: " + base,
			Body:  "Comandos comunes del proyecto extraídos de `" + base + "`:\n\n```\n" + truncate(content, 4000) + "\n```",
		}
	}
	// Docs
	if (strings.HasPrefix(path, "docs/") || strings.HasPrefix(path, "doc/") ||
		strings.HasPrefix(path, "documentation/")) && strings.HasSuffix(low, ".md") {
		title := strings.TrimSuffix(base, ".md")
		return classifiedFile{
			Category: "knowledge", Kind: "docs",
			Slug: "imported-docs-" + strings.ToLower(strings.ReplaceAll(strings.TrimSuffix(path, ".md"), "/", "-")),
			Title: title, Body: content,
		}
	}
	// CI workflows
	if strings.HasPrefix(path, ".github/workflows/") && (strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".yaml")) {
		return classifiedFile{
			Category: "knowledge", Kind: "ci",
			Slug: "imported-ci-" + strings.TrimSuffix(strings.TrimSuffix(low, ".yml"), ".yaml"),
			Title: "CI workflow: " + base, Body: content,
		}
	}
	// Openspec
	if strings.HasPrefix(path, "openspec/changes/") && strings.HasSuffix(low, ".md") {
		return classifiedFile{
			Category: "knowledge", Kind: "spec",
			Slug: "imported-spec-" + strings.ToLower(strings.ReplaceAll(strings.TrimSuffix(path, ".md"), "/", "-")),
			Title: "Spec: " + path, Body: content,
		}
	}
	return classifiedFile{Category: "ignored"}
}

// techStackSummary: extrae info útil de un manifest. Conservador — devuelve
// el archivo completo truncated si no podemos parsear.
func techStackSummary(base, content string) string {
	header := "Stack manifest detectado: `" + base + "`\n\n"
	return header + "```\n" + truncate(content, 3000) + "\n```\n\nLLM: extraé lenguaje + version + deps relevantes según el formato del archivo."
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n...[truncated]"
}

func (d *Deps) handleProjectIndexSubmit(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no principal"), nil
	}
	if d.ProjectPolicies == nil || d.Knowledge == nil {
		return mcp.NewToolResultError("project_policies o knowledge service no configurado"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)
	args := req.GetArguments()
	runIDStr, _ := args["run_id"].(string)
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		return mcp.NewToolResultError("run_id inválido"), nil
	}
	rawFiles, _ := args["files"].([]any)
	if len(rawFiles) == 0 {
		return mcp.NewToolResultError("files requerido (no vacío)"), nil
	}
	complete, _ := args["complete"].(bool)

	// Resolver project del run
	var projectID uuid.UUID
	if err := d.q(ctx).QueryRow(ctx,
		`SELECT project_id FROM project_index_runs
		   WHERE organization_id = $1 AND id = $2`,
		orgID, runID,
	).Scan(&projectID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("run not found: %v", err)), nil
	}

	policiesCreated, knowledgeCreated, ignored := 0, 0, 0
	ignoredPaths := []string{}

	for _, raw := range rawFiles {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path, _ := m["path"].(string)
		content, _ := m["content"].(string)
		if path == "" || content == "" {
			continue
		}
		cls := classifyFile(path, content)
		switch cls.Category {
		case "policy":
			// upsert via service.Create (si slug existe, falla — manejamos)
			_, err := d.ProjectPolicies.Create(ctx, projectpolicysvc.CreateInput{
				OrganizationID: orgID, ProjectID: projectID,
				Slug: cls.Slug, Name: cls.Title, Kind: cls.Kind,
				BodyMD: cls.Body, Source: "seed_imported",
			})
			if err == nil {
				policiesCreated++
			} else {
				// Probable conflict: ya existe. Intentar UPDATE manual.
				_, _ = d.q(ctx).Exec(ctx,
					`UPDATE project_policies
					   SET body_md=$3, name=$4, updated_at=NOW()
					   WHERE organization_id=$1 AND id=(SELECT id FROM project_policies
					     WHERE organization_id=$1 AND project_id=$2 AND slug=$5 AND is_active=true LIMIT 1)`,
					orgID, projectID, cls.Body, cls.Title, cls.Slug,
				)
				policiesCreated++ // contar como touched
			}
		case "knowledge":
			// INSERT en knowledge_docs. metadata.slug se usa como key de
			// idempotencia (re-import del mismo path UPDATE-a en lugar de
			// duplicar). source='seed_imported' lo marca.
			metaJSON, _ := json.Marshal(map[string]any{
				"slug":        cls.Slug,
				"source_path": path,
				"kind":        cls.Kind,
			})
			tags := []string{"seed_imported", cls.Kind}
			// Buscar existente por metadata->>'slug' para upsert manual.
			var existingID uuid.UUID
			_ = d.q(ctx).QueryRow(ctx,
				`SELECT id FROM knowledge_docs
				   WHERE organization_id = $1 AND project_id = $2
				     AND metadata->>'slug' = $3 AND deleted_at IS NULL
				   LIMIT 1`,
				orgID, projectID, cls.Slug,
			).Scan(&existingID)
			var kerr error
			if existingID != uuid.Nil {
				_, kerr = d.q(ctx).Exec(ctx,
					`UPDATE knowledge_docs
					   SET title=$3, body=$4, metadata=$5, tags=$6, updated_at=NOW()
					   WHERE organization_id=$1 AND id=$2`,
					orgID, existingID, cls.Title, cls.Body, metaJSON, tags,
				)
			} else {
				_, kerr = d.q(ctx).Exec(ctx,
					`INSERT INTO knowledge_docs
					   (organization_id, project_id, created_by, title, body,
					    source, tags, metadata)
					 VALUES ($1, $2, $3, $4, $5, 'seed_imported', $6, $7)`,
					orgID, projectID, userID, cls.Title, cls.Body, tags, metaJSON,
				)
			}
			if kerr == nil {
				knowledgeCreated++
			} else {
				ignored++
				ignoredPaths = append(ignoredPaths, path+" ("+kerr.Error()[:min(60, len(kerr.Error()))]+")")
			}
		default:
			ignored++
			ignoredPaths = append(ignoredPaths, path)
		}
	}

	// Update summary del run.
	summary := map[string]any{
		"policies_created":     policiesCreated,
		"knowledge_created":    knowledgeCreated,
		"ignored":              ignored,
		"ignored_paths_sample": firstN(ignoredPaths, 10),
	}
	summaryJSON, _ := json.Marshal(summary)
	status := "running"
	completedClause := ""
	if complete {
		status = "completed"
		completedClause = ", completed_at = NOW()"
	}
	if _, err := d.q(ctx).Exec(ctx,
		`UPDATE project_index_runs
		   SET files_submitted = files_submitted + $3,
		       summary = $4::jsonb,
		       status = $5`+completedClause+`
		   WHERE organization_id = $1 AND id = $2`,
		orgID, runID, len(rawFiles), summaryJSON, status,
	); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("update run: %v", err)), nil
	}

	return toolResultJSON(map[string]any{
		"run_id":              runID.String(),
		"status":              status,
		"files_submitted":     len(rawFiles),
		"policies_created":    policiesCreated,
		"knowledge_created":   knowledgeCreated,
		"ignored":             ignored,
		"ignored_paths":       firstN(ignoredPaths, 10),
	})
}

func (d *Deps) handleProjectIndexStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Projects == nil {
		return mcp.NewToolResultError("no configurado"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	slug, _ := args["project_slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("project_slug requerido"), nil
	}
	proj, err := d.Projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
	}
	var id, status, gitHead string
	var summaryRaw []byte
	var filesSubmitted int
	var startedAt, completedAt string
	err = d.q(ctx).QueryRow(ctx,
		`SELECT id::text, status, COALESCE(git_head,''), summary,
		        files_submitted,
		        to_char(started_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        COALESCE(to_char(completed_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),'')
		   FROM project_index_runs
		   WHERE organization_id = $1 AND project_id = $2
		   ORDER BY started_at DESC LIMIT 1`,
		orgID, proj.ID,
	).Scan(&id, &status, &gitHead, &summaryRaw, &filesSubmitted, &startedAt, &completedAt)
	if err != nil {
		return toolResultJSON(map[string]any{
			"has_run":     false,
			"recommendation": "No hay indexing previo. Llamá domain_project_index_start para crear el primer index.",
		})
	}
	var summary any
	_ = json.Unmarshal(summaryRaw, &summary)
	return toolResultJSON(map[string]any{
		"has_run":         true,
		"run_id":          id,
		"status":          status,
		"git_head":        gitHead,
		"files_submitted": filesSubmitted,
		"summary":         summary,
		"started_at":      startedAt,
		"completed_at":    completedAt,
	})
}

func firstN(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ context.Context
