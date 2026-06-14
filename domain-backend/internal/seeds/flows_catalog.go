package seeds

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FlowCatalogEntry — issue-08.10 seed-003: declaración built-in de un
// flow seedeable per-org. Hoy el único entry es `sdd-pipeline-v1`, pero
// el shape es reutilizable para futuros built-in flows (project bootstrap,
// nightly maintenance, etc.).
type FlowCatalogEntry struct {
	Slug                string
	Name                string
	Description         string
	Spec                FlowSpecJSON
	IsActive            bool
	DeterministicReplay bool
}

// FlowSpecJSON es el shape serializable a JSONB que se persiste en
// flows.spec. Coincide con la estructura `internal/service/flow.Spec`
// (Version + Steps + Step) pero no la importa para evitar acoplamiento
// del seeder con el service layer (los seeders escriben SQL directo).
type FlowSpecJSON struct {
	Version int                `json:"version"`
	Steps   []FlowSpecStepJSON `json:"steps"`
}

// FlowSpecStepJSON es un step individual del DAG.
//
// El campo Type sigue los step types soportados por internal/service/flow
// (agent_run, skill_run, condition, parallel, …). Para sdd-pipeline-v1
// usamos type='agent_run' como tag canónico, pero el flow NO se ejecuta
// vía internal/runner/flow.Runner.Run — el orquestador SDD
// (internal/service/orchestrator) implementa su propio dispatcher porque
// las fases corren en el cliente IDE, no server-side.
//
// El config trae claves específicas para el orquestador:
//   - agent_template_slug: el sdd-* template del catálogo v3
//   - phase: el PhaseSlug correspondiente
//   - retry_policy: ""|"re-emit"|"require-cleanup" (RFC 0006)
//
// Si en algún momento se quisiera ejecutar este flow con
// flowrunner.Run, fallaría limpio porque agent_run espera "agent_slug"
// (instance), no "agent_template_slug" (template). Esa divergencia es
// intencional: previene ejecuciones accidentales server-side de fases
// que tienen que correr en el cliente.
type FlowSpecStepJSON struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Config  map[string]any `json:"config"`
	OnError string         `json:"on_error,omitempty"`
}

// SDDPipelineFlowSlug es el identificador canónico del flow del
// orquestador SDD (RFC 0006). Exportado para que callers externos
// (orquestador, MCP tools) puedan referenciarlo por const en vez de
// string-literal.
const SDDPipelineFlowSlug = "sdd-pipeline-v1"

// SDDPipelinePhaseSlugs — orden canónico de las 10 fases SDD.
// Coincide con internal/service/orchestrator/types.go.PhaseSlug consts
// pero replicado para evitar el cycle import.
var SDDPipelinePhaseSlugs = []string{
	"sdd-explore",
	"sdd-spec",
	"sdd-propose",
	"sdd-design",
	"sdd-tasks",
	"sdd-apply",
	"sdd-verify",
	"sdd-judge",
	"sdd-archive",
	"sdd-onboard",
}

// FlowsCatalog devuelve el set de flows seedeables per-org. El catalog
// es estable; bumps de seed_version sólo cuando el spec cambia.
func FlowsCatalog() []FlowCatalogEntry {
	return []FlowCatalogEntry{
		{
			Slug:        SDDPipelineFlowSlug,
			Name:        "SDD Pipeline v1",
			Description: "Pipeline canónico del orquestador SDD (RFC 0006). 10 fases: explore→spec→propose→design→tasks→apply→verify→judge→archive→onboard.",
			Spec:        buildSDDPipelineSpec(),
			IsActive:    true,
			// deterministic_replay=false: las fases corren en el cliente IDE
			// con LLM no-determinista (Claude/etc). El replay determinista
			// no aplica a flows con steps tipo agent_run con modelos en vivo.
			DeterministicReplay: false,
		},
	}
}

// buildSDDPipelineSpec construye el spec JSONB del flow sdd-pipeline-v1
// con un step por fase. Cada step lleva la retry_policy alineada con
// RFC 0006 mapping a saga events (heartbeat-watcher issue-08.11).
func buildSDDPipelineSpec() FlowSpecJSON {
	steps := make([]FlowSpecStepJSON, len(SDDPipelinePhaseSlugs))
	for i, slug := range SDDPipelinePhaseSlugs {
		steps[i] = FlowSpecStepJSON{
			ID:   slug,
			Type: "agent_run",
			Config: map[string]any{
				"agent_template_slug": slug,
				"phase":               slug,
				"retry_policy":        retryPolicyForPhase(slug),
			},
			OnError: "fail",
		}
	}
	return FlowSpecJSON{Version: 1, Steps: steps}
}

// retryPolicyForPhase mapea cada fase a su retry_policy default.
// Las políticas vienen del análisis de RFC 0006 ADR-1:
//   - apply muta workspace: cleanup_required
//   - verify es read-only: re-emit (idempotente)
//   - resto: default (auto-retry, idempotent reasoning)
func retryPolicyForPhase(phase string) string {
	switch phase {
	case "sdd-apply":
		return "require-cleanup"
	case "sdd-verify":
		return "re-emit"
	default:
		return ""
	}
}

// flowsSeedVersion bump por cambio semántico del spec (Version del
// FlowSpecJSON refleja la API; este int controla el seeder dedup).
const flowsSeedVersion = 1

// SeedFlowsForOrg materializa el catalog en una org específica.
// Idempotente (UPSERT) y respeta is_user_modified=true; cleanup
// defensivo borra rows seed_managed con slugs ya no presentes en el
// catalog, salvo que tengan flow_runs activos (status pending/running)
// — borrar un flow con runs activos rompería FK + perdería traza.
//
// Sigue el patrón de SeedAgentTemplatesForOrg (chunk 28fddeb): xmax=0
// distingue INSERT real vs DO UPDATE para reportar Created/Updated.
func SeedFlowsForOrg(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) (Report, error) {
	var rep Report
	catalog := FlowsCatalog()
	for _, e := range catalog {
		specJSON, err := json.Marshal(e.Spec)
		if err != nil {
			return rep, fmt.Errorf("marshal spec for %s: %w", e.Slug, err)
		}
		var inserted bool
		err = pool.QueryRow(ctx, `
			INSERT INTO flows
			  (organization_id, slug, name, description, spec, is_active,
			   deterministic_replay, seed_managed, seed_version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,true,$8)
			ON CONFLICT (organization_id, slug) DO UPDATE
			SET name                  = CASE WHEN flows.is_user_modified THEN flows.name ELSE EXCLUDED.name END,
			    description           = CASE WHEN flows.is_user_modified THEN flows.description ELSE EXCLUDED.description END,
			    spec                  = CASE WHEN flows.is_user_modified THEN flows.spec ELSE EXCLUDED.spec END,
			    is_active             = CASE WHEN flows.is_user_modified THEN flows.is_active ELSE EXCLUDED.is_active END,
			    deterministic_replay  = CASE WHEN flows.is_user_modified THEN flows.deterministic_replay ELSE EXCLUDED.deterministic_replay END,
			    seed_version          = EXCLUDED.seed_version
			RETURNING (xmax = 0)`,
			orgID, e.Slug, e.Name, nullStrSeed(e.Description), specJSON,
			e.IsActive, e.DeterministicReplay, flowsSeedVersion,
		).Scan(&inserted)
		if err != nil {
			return rep, fmt.Errorf("upsert flow %s: %w", e.Slug, err)
		}
		if inserted {
			rep.Created++
		} else {
			// Distinguimos preserved (user-modified, no se tocó) vs updated
			// (se aplicaron cambios del catálogo). Para eso releemos la
			// row: si is_user_modified=true → preserved, else updated.
			var userModified bool
			if scanErr := pool.QueryRow(ctx,
				`SELECT is_user_modified FROM flows WHERE organization_id=$1 AND slug=$2`,
				orgID, e.Slug,
			).Scan(&userModified); scanErr == nil && userModified {
				rep.Preserved++
			} else {
				rep.Updated++
			}
		}
	}

	// Cleanup defensivo: borra seed_managed=true con slugs ya no en el
	// catálogo actual Y sin flow_runs activos. Respeta is_user_modified=true.
	currentSlugs := make([]string, len(catalog))
	for i, e := range catalog {
		currentSlugs[i] = e.Slug
	}
	cleanupTag, err := pool.Exec(ctx, `
		DELETE FROM flows f
		WHERE f.organization_id = $1
		  AND f.seed_managed = true
		  AND f.is_user_modified = false
		  AND NOT (f.slug = ANY($2))
		  AND NOT EXISTS (
		    SELECT 1 FROM flow_runs r
		    WHERE r.flow_id = f.id
		      AND r.status IN ('pending','running','paused',
		                       'paused_awaiting_signal','paused_awaiting_human')
		  )`,
		orgID, currentSlugs)
	if err != nil {
		return rep, fmt.Errorf("cleanup legacy flows: %w", err)
	}
	rep.Deleted = int(cleanupTag.RowsAffected())
	return rep, nil
}

// nullStrSeed helper local — Go vanilla sin sql.NullString para
// mantener el seeder simple. Devuelve nil interface si vacío para
// que pgx inserte NULL.
func nullStrSeed(s string) any {
	if s == "" {
		return nil
	}
	return s
}
