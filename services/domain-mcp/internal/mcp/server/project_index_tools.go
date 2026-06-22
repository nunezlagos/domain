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
//      submitea con domain_project_index_submit(run_id, files[]).
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
//   3. domain_project_index_status(project_slug) → ultimo run + counts.
//
// Aspecto Cursor-like: una sola call al LLM "indexar este proyecto" deja
// persistido todo el contexto del repo en project_policies + knowledge.
// Despues domain_policy_get, mem_search etc lo recuperan sin volver al fs.
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
		mcp.WithDescription("Inicia un indexing run del proyecto. Server devuelve un manifest de paths/patterns relevantes que usted (LLM) tiene que leer del repo con tu tool Read nativa. Despues submitea con domain_project_index_submit. Resultado: docs/conventions/stack del repo quedan persistidos como project_policies + knowledge_docs para que futuras sesiones los tengan en BD sin volver al filesystem (RAG del proyecto)."),
		mcp.WithString("project_slug", mcp.Description("Proyecto a indexar"), mcp.Required()),
		mcp.WithString("git_head", mcp.Description("SHA1 del HEAD actual. Opcional pero recomendado para audit.")),
		mcp.WithBoolean("force", mcp.Description("Re-indexar aunque haya run reciente. Default false.")),
	)
}

func toolProjectIndexSubmit() mcp.Tool {
	return mcp.NewTool("domain_project_index_submit",
		mcp.WithDescription("Submitea N archivos del repo al server para clasificacion + persistencia. Cada archivo: {path, content}. El server clasifica el archivo segun su path/contenido (AGENTS.md→policy, README→knowledge, package.json→tech_stack, Makefile→comandos comunes, etc) y persiste en la tabla apropiada con source='seed_imported'. NO modifica el archivo original. Idempotente: re-submitear el mismo path actualiza la version."),
		mcp.WithString("run_id", mcp.Description("UUID del run devuelto por start"), mcp.Required()),
		mcp.WithArray("files", mcp.Description("Array de {path: 'relativo al repo', content: 'texto del archivo'}"), mcp.Required()),
		mcp.WithBoolean("complete", mcp.Description("Si true, marca el run como completed. Si false, queda running para mas submits. Default: false (ultimo submit debe pasar complete=true).")),
	)
}

func toolProjectIndexStatus() mcp.Tool {
	return mcp.NewTool("domain_project_index_status",
		mcp.WithDescription("Devuelve el ultimo indexing run del proyecto + summary (counts de que se persistio). Util para saber si vale re-indexar (last run hace >7 dias o git_head distinto) o si ya esta fresco."),
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
		   (project_id, started_by, git_head)
		 VALUES ($1,$2,NULLIF($3,''))
		 RETURNING id`,
		proj.ID, userID, gitHead,
	).Scan(&runID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create run failed: %v", err)), nil
	}

	manifest := buildIndexManifest()
	return toolResultJSON(map[string]any{
		"run_id":      runID.String(),
		"project_id":  proj.ID.String(),
		"project_slug": slug,
		"manifest":    manifest,
		"next_step":   "Lee con tu tool Read los archivos que matcheen los patterns del manifest. Despues llama domain_project_index_submit con un batch de {path, content}. En el ultimo batch pasa complete=true para cerrar el run.",
	})
}

// buildIndexManifest: paths/patterns ordenados por prioridad (los mas
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
			Body:  "Comandos comunes del proyecto extraidos de `" + base + "`:\n\n```\n" + truncate(content, 4000) + "\n```",
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

// techStackSummary: extrae info util de un manifest. Conservador — devuelve
// el archivo completo truncated si no podemos parsear.
func techStackSummary(base, content string) string {
	header := "Stack manifest detectado: `" + base + "`\n\n"
	return header + "```\n" + truncate(content, 3000) + "\n```\n\nLLM: extrae lenguaje + version + deps relevantes segun el formato del archivo."
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
		return mcp.NewToolResultError("run_id invalido"), nil
	}
	rawFiles, _ := args["files"].([]any)
	if len(rawFiles) == 0 {
		return mcp.NewToolResultError("files requerido (no vacio)"), nil
	}
	complete, _ := args["complete"].(bool)

	// Resolver project del run
	var projectID uuid.UUID
	if err := d.q(ctx).QueryRow(ctx,
		`SELECT project_id FROM project_index_runs
		   WHERE id = $1`,
		runID,
	).Scan(&projectID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("run not found: %v", err)), nil
	}

	policiesCreated, knowledgeCreated, ignored := 0, 0, 0
	ignoredPaths := []string{}

	// DOMAINSERV-1: cada archivo se procesa dentro de su propio SAVEPOINT.
	// Si un statement falla (ej: unique violation, subquery multi-row),
	// la tx queda "aborted" (SQLSTATE 25P02) y mata todos los statements
	// subsiguientes. Con SAVEPOINT/RELEASE/ROLLBACK TO, un fallo se
	// revierte al punto pre-iteracion y la tx sigue sana para los
	// siguientes archivos.
	for i, raw := range rawFiles {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path, _ := m["path"].(string)
		content, _ := m["content"].(string)
		if path == "" || content == "" {
			continue
		}

		spName := fmt.Sprintf("sp_idx_%d", i)
		if _, err := d.q(ctx).Exec(ctx, "SAVEPOINT "+spName); err != nil {
			// Si ni el SAVEPOINT funciona, la tx esta rota de verdad.
			// Marcamos el archivo como ignored y continuamos con los
			// siguientes (cada uno va a tener el mismo problema, pero
			// al menos el handler no crashea).
			ignored++
			ignoredPaths = append(ignoredPaths, path+" (savepoint failed: "+err.Error()+")")
			continue
		}

		cls := classifyFile(path, content)
		var fileErr error
		switch cls.Category {
		case "policy":
			// upsert via service.Create (si slug existe, falla — manejamos
			// con ROLLBACK TO SAVEPOINT y luego UPDATE manual).
			_, serr := d.ProjectPolicies.Create(ctx, projectpolicysvc.CreateInput{
				OrganizationID: orgID, ProjectID: projectID,
				Slug: cls.Slug, Name: cls.Title, Kind: cls.Kind,
				BodyMD: cls.Body, Source: "seed_imported",
			})
			if serr == nil {
				policiesCreated++
			} else {
				// Revertimos el Create fallido y probamos con UPDATE.
				if _, rerr := d.q(ctx).Exec(ctx, "ROLLBACK TO SAVEPOINT "+spName); rerr != nil {
					fileErr = fmt.Errorf("create+rollback: %w / %v", serr, rerr)
				} else {
					// UPDATE usando CTE para evitar subquery multi-row que
					// podia romper la tx con error de subquery.
					ctag, uerr := d.q(ctx).Exec(ctx,
						`WITH existing AS (
						   SELECT id FROM project_policies
						   WHERE project_id = $1
						     AND slug = $2 AND is_active = true
						   LIMIT 1
						 )
						 UPDATE project_policies
						    SET body_md = $3, name = $4, updated_at = NOW()
						   FROM existing
						   WHERE project_policies.id = existing.id`,
						projectID, cls.Slug, cls.Body, cls.Title,
					)
					if uerr != nil {
						// Update fallo — el savepoint sigue rolled back,
						// marcamos ignored.
						fileErr = fmt.Errorf("update policy: %w (create was: %v)", uerr, serr)
					} else if ctag.RowsAffected() == 0 {
						// No habia policy previa — el Create fallo por otra
						// razon (ej: validation). Lo registramos como ignored.
						fileErr = fmt.Errorf("policy create failed y no habia previa: %v", serr)
					} else {
						policiesCreated++
					}
				}
			}
		case "knowledge":
			// INSERT en knowledge_docs con upsert manual por metadata->>'slug'.
			metaJSON, _ := json.Marshal(map[string]any{
				"slug":        cls.Slug,
				"source_path": path,
				"kind":        cls.Kind,
			})
			tags := []string{"seed_imported", cls.Kind}
			var existingID uuid.UUID
			qerr := d.q(ctx).QueryRow(ctx,
				`SELECT id FROM knowledge_docs
				   WHERE project_id = $1
				     AND metadata->>'slug' = $2 AND deleted_at IS NULL
				   LIMIT 1`,
				projectID, cls.Slug,
			).Scan(&existingID)
			if qerr != nil && qerr.Error() != "no rows in result set" {
				fileErr = fmt.Errorf("lookup knowledge: %w", qerr)
			} else {
				if existingID != uuid.Nil {
					_, uerr := d.q(ctx).Exec(ctx,
						`UPDATE knowledge_docs
						   SET title=$2, body=$3, metadata=$4, tags=$5, updated_at=NOW()
						   WHERE id=$1`,
						existingID, cls.Title, cls.Body, metaJSON, tags,
					)
					if uerr != nil {
						fileErr = fmt.Errorf("update knowledge: %w", uerr)
					} else {
						knowledgeCreated++
					}
				} else {
					_, ierr := d.q(ctx).Exec(ctx,
						`INSERT INTO knowledge_docs
						   (project_id, created_by, title, body,
						    source, tags, metadata)
						 VALUES ($1, $2, $3, $4, 'seed_imported', $5, $6)`,
						projectID, userID, cls.Title, cls.Body, tags, metaJSON,
					)
					if ierr != nil {
						fileErr = fmt.Errorf("insert knowledge: %w", ierr)
					} else {
						knowledgeCreated++
					}
				}
			}
		default:
			ignored++
			ignoredPaths = append(ignoredPaths, path)
		}

		// Si hubo error en este archivo, lo registramos como ignored
		// pero la tx ya esta en el savepoint (rolled back), lista
		// para el siguiente archivo.
		if fileErr != nil {
			ignored++
			errMsg := fileErr.Error()
			if len(errMsg) > 60 {
				errMsg = errMsg[:60]
			}
			ignoredPaths = append(ignoredPaths, path+" ("+errMsg+")")
			// Asegurar que la tx esta limpia para el proximo archivo.
			// Si el savepoint sigue activo (caso CREATE OK que no fue),
			// ROLLBACK. Si ya esta rolled back, esto es no-op.
			_, _ = d.q(ctx).Exec(ctx, "ROLLBACK TO SAVEPOINT "+spName)
		} else {
			// Todo OK en este archivo — liberar el savepoint.
			_, _ = d.q(ctx).Exec(ctx, "RELEASE SAVEPOINT "+spName)
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
		   SET files_submitted = files_submitted + $2,
		       summary = $3::jsonb,
		       status = $4`+completedClause+`
		   WHERE id = $1`,
		runID, len(rawFiles), summaryJSON, status,
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
		   WHERE project_id = $1
		   ORDER BY started_at DESC LIMIT 1`,
		proj.ID,
	).Scan(&id, &status, &gitHead, &summaryRaw, &filesSubmitted, &startedAt, &completedAt)
	if err != nil {
		return toolResultJSON(map[string]any{
			"has_run":     false,
			"recommendation": "No hay indexing previo. Llama domain_project_index_start para crear el primer index.",
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
