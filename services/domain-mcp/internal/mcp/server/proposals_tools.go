// REQ-49 — proposals de policies/skills auto-generadas por el LLM.
//
// Workflow:
//  1. LLM detecta un patron recurrente del proyecto (workflow git,
//     convention de migrations, tech stack constraint, etc).
//  2. Llama domain_propose_policy o domain_propose_skill con
//     source='llm_generated' + proposed=true. Queda invisible para los
//     resolvers (policy_get, skill_search) hasta que el usuario apruebe.
//  3. domain_proposal_list muestra las propuestas pendientes.
//  4. domain_proposal_review(kind, id, action: accept|reject) decide.
//
// Por que no aprobacion automatica: el LLM puede malinterpretar un
// pattern. Mantener al humano en el loop evita reglas equivocadas que
// despues confunden al propio LLM.
package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	projsvc "nunezlagos/domain/internal/service/project"
	projectpolicysvc "nunezlagos/domain/internal/service/projectpolicy"
	"nunezlagos/domain/internal/store/txctx"
)

type proposalsPoliciesStore interface {
	Create(ctx context.Context, in projectpolicysvc.CreateInput) (*projectpolicysvc.Policy, error)
}

type projectLookup interface {
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*projsvc.Project, error)
}

type proposalsHandlers struct {
	projectPolicies proposalsPoliciesStore
	projects        projectLookup
	pool            *pgxpool.Pool
	principal       *apikey.Principal
}

func (h *proposalsHandlers) q(ctx context.Context) interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
} {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return h.pool
}

func registerProposalsTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &proposalsHandlers{
		projectPolicies: deps.ProjectPolicies,
		projects:        deps.Projects,
		pool:            deps.Pool,
		principal:       deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolProposePolicy(), Handler: wrap.Wrap("domain_propose_policy", rls(h.handleProposePolicy))},
		{Tool: toolProposeSkill(), Handler: wrap.Wrap("domain_propose_skill", rls(h.handleProposeSkill))},
		{Tool: toolProposalList(), Handler: wrap.Wrap("domain_proposal_list", rls(h.handleProposalList))},
		{Tool: toolProposalReview(), Handler: wrap.Wrap("domain_proposal_review", rls(h.handleProposalReview))},
	}
}

func toolProposePolicy() mcp.Tool {
	return mcp.NewTool("domain_propose_policy",
		mcp.WithDescription("SOLO modo headless/batch (sin humano presente para confirmar). Propone una project_policy en estado proposed=true — invisible para policy_get hasta domain_proposal_review. Con usuario presente NO uses esto: confirmá el contenido en el momento (AskUserQuestion) y creá activa con domain_project_policy_set. NO usar para reglas obvias (ej. 'usar git') — usar para patterns especificos del repo (workflow, migrations, convention)."),
		mcp.WithString("project_slug", mcp.Description("Proyecto al que aplica"), mcp.Required()),
		mcp.WithString("slug", mcp.Description("Slug de la policy propuesta"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Nombre legible"), mcp.Required()),
		mcp.WithString("kind", mcp.Description("convention|security_rule|architecture|sdd_workflow|observability|migration_rule|linter_config|agent_protocol|git_workflow|tech_stack|test_strategy"), mcp.Required()),
		mcp.WithString("body_md", mcp.Description("Cuerpo Markdown — que es la regla y por que"), mcp.Required()),
		mcp.WithString("rationale", mcp.Description("Por que propones esta regla: que pattern observaste, en cuantos archivos/turns lo viste. Esto le da contexto al humano que aprueba.")),
	)
}

func toolProposeSkill() mcp.Tool {
	return mcp.NewTool("domain_propose_skill",
		mcp.WithDescription("SOLO modo headless/batch (sin humano presente para confirmar). Propone una skill en estado proposed=true — invisible hasta aprobacion. Con usuario presente NO uses esto: confirmá el contenido en el momento (AskUserQuestion) y creá activa con domain_project_skill_register (interna) o domain_skill_create (global). Util cuando haces N veces el mismo comando con variantes (ej. 'php artisan migrate manual + reload cache + clear views' = una skill 'reset-db')."),
		mcp.WithString("project_slug", mcp.Description("Proyecto al que aplica (null = global de la org)")),
		mcp.WithString("slug", mcp.Description("Slug de la skill"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Nombre legible"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Para que sirve, que inputs/outputs espera"), mcp.Required()),
		mcp.WithString("skill_type", mcp.Description("prompt|code|api|mcp_tool"), mcp.Required()),
		mcp.WithString("content", mcp.Description("Cuerpo de la skill (template, codigo, etc)"), mcp.Required()),
		mcp.WithString("rationale", mcp.Description("Por que la propones: que hiciste manualmente N veces.")),
	)
}

func toolProposalList() mcp.Tool {
	return mcp.NewTool("domain_proposal_list",
		mcp.WithDescription("Lista proposals pendientes (proposed=true, sin review todavia). El usuario revisa y decide con domain_proposal_review."),
		mcp.WithString("kind", mcp.Description("Filtrar: policy | skill | all (default)")),
		mcp.WithString("project_slug", mcp.Description("Filtrar proposals de un proyecto especifico (solo afecta policies)")),
	)
}

func toolProposalReview() mcp.Tool {
	return mcp.NewTool("domain_proposal_review",
		mcp.WithDescription("Acepta o rechaza una proposal. accept → proposed=false (queda visible y activa). reject → soft-delete (deleted_at=NOW). El review queda en audit."),
		mcp.WithString("kind", mcp.Description("policy | skill"), mcp.Required()),
		mcp.WithString("id", mcp.Description("UUID del row a revisar"), mcp.Required()),
		mcp.WithString("action", mcp.Description("accept | reject"), mcp.Required()),
		mcp.WithString("reason", mcp.Description("Razon del review (opcional, queda como audit). Util cuando rechazas para que el LLM aprenda.")),
	)
}

func (h *proposalsHandlers) handleProposePolicy(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.projectPolicies == nil || h.projects == nil {
		return mcp.NewToolResultError("project_policy service not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	kind, _ := args["kind"].(string)
	body, _ := args["body_md"].(string)
	if projSlug == "" || slug == "" || name == "" || kind == "" || body == "" {
		return mcp.NewToolResultError("project_slug, slug, name, kind y body_md requeridos"), nil
	}
	rationale, _ := args["rationale"].(string)
	if rationale != "" {
		body = body + "\n\n---\n_Rationale (propuesto por LLM)_: " + rationale
	}

	proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	created, err := h.projectPolicies.Create(ctx, projectpolicysvc.CreateInput{
		OrganizationID: orgID,
		ProjectID:      proj.ID,
		Slug:           slug,
		Name:           name,
		Kind:           kind,
		BodyMD:         body,
		Source:         "llm_generated",
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create proposal failed: %v", err)), nil
	}

	if _, err := h.q(ctx).Exec(ctx,
		`UPDATE project_policies SET proposed = true
		   WHERE id = $1`,
		created.ID,
	); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("mark proposed failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"kind":      "policy",
		"id":        created.ID.String(),
		"slug":      created.Slug,
		"project":   projSlug,
		"proposed":  true,
		"next_step": "Esta propuesta queda invisible para domain_policy_get hasta que el usuario la apruebe con domain_proposal_review.",
	})
}

func (h *proposalsHandlers) handleProposeSkill(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.pool == nil {
		return mcp.NewToolResultError("pool not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	desc, _ := args["description"].(string)
	skillType, _ := args["skill_type"].(string)
	content, _ := args["content"].(string)
	if slug == "" || name == "" || desc == "" || skillType == "" || content == "" {
		return mcp.NewToolResultError("slug, name, description, skill_type y content son requeridos"), nil
	}
	rationale, _ := args["rationale"].(string)
	if rationale != "" {
		desc = desc + "\n\nRationale (propuesto por LLM): " + rationale
	}

	var projectID *uuid.UUID
	if projSlug, _ := args["project_slug"].(string); projSlug != "" && h.projects != nil {
		if proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug); perr == nil {
			pid := proj.ID
			projectID = &pid
		}
	}

	var id uuid.UUID
	err := h.q(ctx).QueryRow(ctx,
		`INSERT INTO skills
		   (project_id, slug, name, description,
		    skill_type, content, input_schema, output_schema, proposed)
		 VALUES ($1,$2,$3,$4,$5,$6,'{}','{}',true)
		 RETURNING id`,
		projectID, slug, name, desc, skillType, content,
	).Scan(&id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create skill proposal failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"kind":      "skill",
		"id":        id.String(),
		"slug":      slug,
		"proposed":  true,
		"next_step": "Esta propuesta queda invisible para domain_skill_search/list hasta que el usuario la apruebe.",
	})
}

func (h *proposalsHandlers) handleProposalList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.pool == nil {
		return mcp.NewToolResultError("pool not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	kind := strings.ToLower(strings.TrimSpace(asString(args["kind"])))
	if kind == "" {
		kind = "all"
	}
	projSlug, _ := args["project_slug"].(string)

	var projectFilter *uuid.UUID
	if projSlug != "" && h.projects != nil {
		if proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug); perr == nil {
			pid := proj.ID
			projectFilter = &pid
		}
	}

	policies := []map[string]any{}
	skills := []map[string]any{}

	const tsFmt = "to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"')"

	if kind == "policy" || kind == "all" {
		q := `SELECT id::text, slug, name, kind, ` + tsFmt + `
		        FROM project_policies
		        WHERE proposed = true AND deleted_at IS NULL`
		queryArgs := []any{}
		if projectFilter != nil {
			q += " AND project_id = $1"
			queryArgs = append(queryArgs, *projectFilter)
		}
		q += " ORDER BY created_at DESC LIMIT 50"
		if rows, err := h.q(ctx).Query(ctx, q, queryArgs...); err == nil {
			for rows.Next() {
				var id, slug, name, k, ts string
				if err := rows.Scan(&id, &slug, &name, &k, &ts); err == nil {
					policies = append(policies, map[string]any{
						"id": id, "slug": slug, "name": name, "kind": k, "created_at": ts,
					})
				}
			}
			rows.Close()
		}
	}

	if kind == "skill" || kind == "all" {
		if rows, err := h.q(ctx).Query(ctx,
			`SELECT id::text, slug, name, skill_type, project_id, `+tsFmt+`
			   FROM skills
			   WHERE proposed = true AND deleted_at IS NULL
			   ORDER BY created_at DESC LIMIT 50`,
		); err == nil {
			for rows.Next() {
				var id, slug, name, st, ts string
				var pid *uuid.UUID
				if err := rows.Scan(&id, &slug, &name, &st, &pid, &ts); err == nil {
					item := map[string]any{
						"id": id, "slug": slug, "name": name, "skill_type": st, "created_at": ts,
					}
					if pid != nil {
						item["project_id"] = pid.String()
					}
					skills = append(skills, item)
				}
			}
			rows.Close()
		}
	}

	return toolResultJSON(map[string]any{
		"policies": policies,
		"skills":   skills,
		"total":    len(policies) + len(skills),
	})
}

func (h *proposalsHandlers) handleProposalReview(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.pool == nil {
		return mcp.NewToolResultError("pool not configured"), nil
	}
	args := req.GetArguments()
	kind := strings.ToLower(strings.TrimSpace(asString(args["kind"])))
	idStr, _ := args["id"].(string)
	action := strings.ToLower(strings.TrimSpace(asString(args["action"])))
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido (UUID requerido)"), nil
	}
	if action != "accept" && action != "reject" {
		return mcp.NewToolResultError("action debe ser 'accept' o 'reject'"), nil
	}

	table := ""
	switch kind {
	case "policy":
		table = "project_policies"
	case "skill":
		table = "skills"
	default:
		return mcp.NewToolResultError("kind debe ser 'policy' o 'skill'"), nil
	}

	var sql string
	if action == "accept" {
		sql = "UPDATE " + table + ` SET proposed = false
		         WHERE id = $1 AND proposed = true AND deleted_at IS NULL`
	} else {
		sql = "UPDATE " + table + ` SET deleted_at = NOW()
		         WHERE id = $1 AND proposed = true AND deleted_at IS NULL`
	}
	tag, err := h.q(ctx).Exec(ctx, sql, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("review failed: %v", err)), nil
	}
	if tag.RowsAffected() == 0 {
		return mcp.NewToolResultError("proposal no encontrada o ya revisada"), nil
	}
	return toolResultJSON(map[string]any{
		"kind":   kind,
		"id":     id.String(),
		"action": action,
	})
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
