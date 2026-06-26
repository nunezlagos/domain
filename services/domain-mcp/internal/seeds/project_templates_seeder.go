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
func (s *ProjectTemplatesSeeder) Version() int    { return 2 }
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
			Settings:    map[string]any{"language": "any"},
		},
		{
			Slug: "go-backend", Name: "Go Backend",
			Description: "Backend Go: clean architecture, pgx, testcontainers",
			Settings: map[string]any{
				"language": "go", "test_framework": "testing+testify",
				"conventions": []string{"clean-architecture-by-feature", "migration-safety"},
			},
			DefaultSkills: []string{"summarize", "code-review"},
		},
		{
			Slug: "python-data", Name: "Python Data",
			Description: "Data pipelines y análisis en Python",
			Settings: map[string]any{
				"language": "python", "test_framework": "pytest",
			},
			DefaultSkills: []string{"summarize"},
		},
		{
			Slug: "frontend-web", Name: "Frontend Web",
			Description: "Aplicaciones web frontend (TS/React u otros)",
			Settings: map[string]any{
				"language": "typescript", "test_framework": "vitest",
			},
			DefaultSkills: []string{"summarize", "code-review"},
		},
		// ── Templates v2: extraídos del análisis cross-project (9 proyectos Saargo) ──
		{
			Slug: "laravel-backend", Name: "Laravel Backend",
			Description: "Laravel 12 + PHP 8.3+ + PostgreSQL: service layer, Pest, Pint, Larastan, Scramble, serverless (Bref/Lambda). Estándar Saargo Curriculum y MJ-Observatorio.",
			Settings: map[string]any{
				"language":        "php",
				"framework":       "laravel",
				"version":         "12.x",
				"test_framework":  "pest",
				"db":              "postgresql",
				"deployment":      "serverless-lambda",
				"package_manager": "composer",
				"conventions": []string{
					"file-size-limit", "yagni-simplicity", "no-n-plus-one",
					"migration-safety", "openspec-spec-format", "openspec-naming-convention",
					"audit-tasks-checklist",
				},
				"patterns": []string{
					"service-layer", "form-request-validation",
					"soft-deletes", "uuid-primary-keys", "observer-side-effects",
				},
				"tooling": map[string]string{
					"linter":          "pint",
					"static_analysis": "larastan",
					"api_docs":        "scramble",
				},
			},
			DefaultSkills: []string{"laravel-specialist", "summarize", "sql-explain-impact", "requesting-code-review"},
		},
		{
			Slug: "adonisjs-legacy", Name: "AdonisJS Legacy (ACE)",
			Description: "AdonisJS 4.1 + Lucid ORM + Vue 2 + Element UI + MySQL: stack ACE (DID/DIDE). Factory pattern para generación de documentos. Playwright E2E.",
			Settings: map[string]any{
				"language":       "javascript",
				"framework":      "adonisjs",
				"version":        "4.x",
				"frontend":       "vue2",
				"ui_library":     "element-ui",
				"db":             "mysql",
				"test_framework": "japa+playwright",
				"deployment":     "pm2-server",
				"docker":         true,
				"conventions": []string{
					"file-size-limit", "conventional-commits-spanish",
					"openspec-spec-format", "openspec-naming-convention",
				},
				"patterns": []string{
					"factory-pattern-documents", "mvc-thin-controllers",
					"docker-first-dev",
				},
				"gotchas": []string{
					"curly-quotes-break-vue",
					"lucid-computed-use-getRelated",
					"always-run-inside-container",
				},
			},
			DefaultSkills: []string{"adonisjs-patterns", "summarize", "requesting-code-review"},
		},
		{
			Slug: "astro-cloudflare", Name: "Astro + Cloudflare Workers",
			Description: "Astro 5 (server output) + Cloudflare Workers + Drizzle ORM + D1 (SQLite) + R2: stack quien-sabe-de-web. Vitest + Playwright E2E.",
			Settings: map[string]any{
				"language":        "typescript",
				"framework":       "astro",
				"version":         "5.x",
				"runtime":         "cloudflare-workers",
				"orm":             "drizzle",
				"db":              "d1-sqlite",
				"storage":         "r2",
				"test_framework":  "vitest+playwright",
				"package_manager": "bun",
				"conventions": []string{
					"file-size-limit", "yagni-simplicity",
					"openspec-spec-format", "openspec-naming-convention",
					"audit-tasks-checklist",
				},
			},
			DefaultSkills: []string{"nextjs-app-router", "summarize"},
		},
		{
			Slug: "fastapi-serverless", Name: "FastAPI + AWS Lambda",
			Description: "FastAPI async + SQLAlchemy 2 + Alembic + Mangum (ASGI→Lambda) + Pydantic v2 + structlog + AWS CDK. Stack ACE-Perfilador.",
			Settings: map[string]any{
				"language":        "python",
				"framework":       "fastapi",
				"version":         "0.115+",
				"deployment":      "aws-lambda-mangum",
				"orm":             "sqlalchemy2",
				"db":              "postgresql",
				"test_framework":  "pytest",
				"infrastructure":  "aws-cdk-typescript",
				"package_manager": "pip+requirements.txt",
				"conventions": []string{
					"file-size-limit", "no-n-plus-one", "migration-safety",
					"openspec-spec-format",
				},
				"patterns": []string{
					"async-everywhere", "pydantic-validation",
					"structlog-structured", "alembic-migrations",
					"soft-delete-is-active",
				},
			},
			DefaultSkills: []string{"fastapi-async", "summarize", "sql-explain-impact"},
		},
		{
			Slug: "nextjs-fullstack", Name: "Next.js Fullstack",
			Description: "Next.js 15 App Router + TypeScript + TanStack Query + shadcn/ui + Tailwind v4 + Bun. Deploy como static export en S3+CloudFront.",
			Settings: map[string]any{
				"language":        "typescript",
				"framework":       "nextjs",
				"version":         "15.x",
				"router":          "app-router",
				"state":           "tanstack-query",
				"ui":              "shadcn-ui",
				"styles":          "tailwind-v4",
				"test_framework":  "vitest+playwright",
				"package_manager": "bun",
				"deployment":      "s3-cloudfront-static",
				"conventions": []string{
					"file-size-limit", "yagni-simplicity", "no-n-plus-one",
					"openspec-spec-format", "openspec-naming-convention",
					"audit-tasks-checklist",
				},
				"patterns": []string{
					"server-components-default", "tanstack-query-client",
					"shadcn-primitives", "static-export-search-params",
				},
			},
			DefaultSkills: []string{"nextjs-app-router", "summarize", "wcag-audit", "requesting-code-review"},
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
