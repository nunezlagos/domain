package seeds

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ProjectTemplatesSeeder siembra templates built-in públicos (issue-01.4).
// organization_id NULL + is_public=TRUE → visibles para todas las orgs.
type ProjectTemplatesSeeder struct{}

func (s *ProjectTemplatesSeeder) Name() string    { return "project_templates" }
func (s *ProjectTemplatesSeeder) Version() int    { return 3 }
func (s *ProjectTemplatesSeeder) Order() int      { return 35 }
func (s *ProjectTemplatesSeeder) IsDevOnly() bool { return false }

type templateEntry struct {
	Slug, Name, Description string
	IsDefault               bool
	Settings                map[string]any
	DefaultSkills           []string
	DefaultAgents           []string
	DefaultFlows            []string
}

func (s *ProjectTemplatesSeeder) Run(ctx context.Context, tx pgx.Tx, env Env) (Report, error) {
	var rep Report

	templates := []templateEntry{
		{
			Slug: "default", Name: "Default", IsDefault: true,
			Description: "Proyecto general sin preconfiguración específica",
			Settings: map[string]any{
				"language": "any",
				"skill_generation_guide": "Detectá el stack del proyecto leyendo sus archivos de configuración (package.json, composer.json, go.mod, pyproject.toml, Cargo.toml, Gemfile, pom.xml). Llamá domain_project_skill_register con: slug '<framework>-<version-major>-stack', role (especialista en el stack+versión exacta), patrones_obligatorios (3-5 convenciones del framework), antipatrones_prohibidos, gotchas detectables, tooling (comandos de test/lint/build). Esta skill se crea UNA VEZ por proyecto.",
			},
		},
	}

	for _, t := range templates {
		settingsJSON, _ := json.Marshal(t.Settings)
		if t.DefaultSkills == nil {
			t.DefaultSkills = []string{}
		}
		if t.DefaultAgents == nil {
			t.DefaultAgents = []string{}
		}
		if t.DefaultFlows == nil {
			t.DefaultFlows = []string{}
		}



		tag, err := tx.Exec(ctx, `
			UPDATE project_templates
			SET name = $2, description = $3, is_default = $4, settings = $5,
			    default_skills = $6, default_agents = $7, default_flows = $8,
			    seed_version = $9
			WHERE slug = $1 AND NOT is_user_modified`,
			t.Slug, t.Name, t.Description, t.IsDefault, settingsJSON,
			t.DefaultSkills, t.DefaultAgents, t.DefaultFlows, s.Version())
		if err != nil {
			return rep, fmt.Errorf("update template %s: %w", t.Slug, err)
		}
		if tag.RowsAffected() > 0 {
			rep.Updated++
			continue
		}
		tag, err = tx.Exec(ctx, `
			INSERT INTO project_templates
			  (slug, name, description, is_default, is_public,
			   settings, default_skills, default_agents, default_flows,
			   seed_managed, seed_version)
			SELECT $1::varchar, $2::varchar, $3::text, $4::boolean, TRUE,
			       $5::jsonb, $6::text[], $7::text[], $8::text[], TRUE, $9::int
			WHERE NOT EXISTS (
			  SELECT 1 FROM project_templates
			  WHERE slug = $1)`,
			t.Slug, t.Name, t.Description, t.IsDefault, settingsJSON,
			t.DefaultSkills, t.DefaultAgents, t.DefaultFlows, s.Version())
		if err != nil {
			return rep, fmt.Errorf("insert template %s: %w", t.Slug, err)
		}
		if tag.RowsAffected() > 0 {
			rep.Created++
		} else {
			rep.Skipped++ // existe pero is_user_modified=TRUE
		}
	}
	return rep, nil
}
