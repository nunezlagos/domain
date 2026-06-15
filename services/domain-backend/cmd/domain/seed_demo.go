package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// runSeedDemo (REQ-64): inserta dataset realista que toca las tablas
// principales y permite disparar el grueso de los MCP tools.
// Idempotente: usa slugs/keys/contenidos estables + ON CONFLICT.
//
// Skipea workflow SDD (proposals→user_stories→requirements) porque
// requiere cadena de FKs que no aporta al benchmark MCP.
//
// Uso: domain seed-demo <organization-uuid>
func runSeedDemo(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Uso: domain seed-demo <organization-uuid>")
		os.Exit(2)
	}
	orgID, err := uuid.Parse(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "UUID inválido:", err)
		os.Exit(2)
	}
	ctx := context.Background()
	dsn := os.Getenv("DOMAIN_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DOMAIN_DATABASE_URL no seteado")
		os.Exit(1)
	}
	pool, err := pgxpoolNew(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open pool:", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := assertOrgExists(ctx, pool, orgID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("Seedeando demo data en org %s...\n", orgID)
	totals := &seedTotals{}
	steps := []struct {
		name string
		fn   func(context.Context, *pgxpool.Pool, uuid.UUID, *seedTotals) error
	}{
		{"users", seedDemoUsers},
		{"clients", seedDemoClients},
		{"projects", seedDemoProjects},
		{"project_repositories", seedDemoRepos},
		{"project_policies", seedDemoPolicies},
		{"knowledge_docs", seedDemoKnowledge},
		{"project_tickets+comments+history", seedDemoTickets},
		{"captured_prompts", seedDemoPrompts},
		{"observations", seedDemoObservations},
		{"verifications", seedDemoVerifications},
		{"project_index_runs", seedDemoIndexRuns},
	}
	for _, s := range steps {
		if err := s.fn(ctx, pool, orgID, totals); err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", s.name, err)
			os.Exit(1)
		}
		fmt.Printf("  ✓ %s\n", s.name)
	}
	fmt.Printf("Listo. Totales: %+v\n", totals)
}

type seedTotals struct {
	Users, Clients, Projects, Repos, Policies, Knowledge int
	Tickets, Comments, History                           int
	Prompts, Observations, Verifications, IndexRuns      int
}

func assertOrgExists(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) error {
	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM organizations WHERE id=$1 AND deleted_at IS NULL)`,
		orgID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check org: %w", err)
	}
	if !exists {
		return fmt.Errorf("org %s no existe (o está soft-deleted)", orgID)
	}
	return nil
}

func withOrg(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID.String()); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// --- helpers de lookup ---

func demoUserID(ctx context.Context, tx pgx.Tx, email string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM users WHERE email=$1 AND deleted_at IS NULL`, email).Scan(&id)
	return id, err
}

func demoProjectID(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, slug string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx,
		`SELECT id FROM projects WHERE organization_id=$1 AND slug=$2 AND deleted_at IS NULL`,
		orgID, slug,
	).Scan(&id)
	return id, err
}

func demoClientID(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, slug string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx,
		`SELECT id FROM clients WHERE organization_id=$1 AND slug=$2 AND deleted_at IS NULL`,
		orgID, slug,
	).Scan(&id)
	return id, err
}

// --- pasos ---

var demoUsers = []struct{ email, name, role string }{
	{"alice@demo.test", "Alice Dev", "admin"},
	{"bob@demo.test", "Bob PM", "member"},
	{"carol@demo.test", "Carol QA", "member"},
	{"dave@demo.test", "Dave Backend", "member"},
	{"eve@demo.test", "Eve Frontend", "member"},
}

func seedDemoUsers(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, tot *seedTotals) error {
	return withOrg(ctx, pool, orgID, func(tx pgx.Tx) error {
		for _, u := range demoUsers {
			ct, err := tx.Exec(ctx,
				`INSERT INTO users (organization_id, email, name, role)
				 VALUES ($1,$2,$3,$4)
				 ON CONFLICT (organization_id, email) DO NOTHING`,
				orgID, u.email, u.name, u.role,
			)
			if err != nil {
				return fmt.Errorf("user %s: %w", u.email, err)
			}
			if ct.RowsAffected() > 0 {
				tot.Users++
			}
		}
		return nil
	})
}

var demoClients = []struct{ slug, name string }{
	{"acme-corp", "Acme Corp"},
	{"beta-inc", "Beta Inc"},
}

func seedDemoClients(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, tot *seedTotals) error {
	return withOrg(ctx, pool, orgID, func(tx pgx.Tx) error {
		for _, c := range demoClients {
			ct, err := tx.Exec(ctx,
				`INSERT INTO clients (organization_id, slug, name, status)
				 VALUES ($1,$2,$3,'active')
				 ON CONFLICT (organization_id, slug) DO NOTHING`,
				orgID, c.slug, c.name,
			)
			if err != nil {
				return fmt.Errorf("client %s: %w", c.slug, err)
			}
			if ct.RowsAffected() > 0 {
				tot.Clients++
			}
		}
		return nil
	})
}

var demoProjects = []struct{ slug, name, clientSlug string }{
	{"acme-web", "Acme Web", "acme-corp"},
	{"acme-api", "Acme API", "acme-corp"},
	{"beta-mobile", "Beta Mobile", "beta-inc"},
	{"internal-ops", "Internal Ops", ""},
}

func seedDemoProjects(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, tot *seedTotals) error {
	return withOrg(ctx, pool, orgID, func(tx pgx.Tx) error {
		for _, p := range demoProjects {
			var clientID any
			if p.clientSlug != "" {
				cid, err := demoClientID(ctx, tx, orgID, p.clientSlug)
				if err != nil {
					return fmt.Errorf("lookup client %s: %w", p.clientSlug, err)
				}
				clientID = cid
			}
			ct, err := tx.Exec(ctx,
				`INSERT INTO projects (organization_id, client_id, slug, name)
				 VALUES ($1,$2,$3,$4)
				 ON CONFLICT (organization_id, slug) DO NOTHING`,
				orgID, clientID, p.slug, p.name,
			)
			if err != nil {
				return fmt.Errorf("project %s: %w", p.slug, err)
			}
			if ct.RowsAffected() > 0 {
				tot.Projects++
			}
		}
		return nil
	})
}

func seedDemoRepos(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, tot *seedTotals) error {
	repos := []struct{ projectSlug, name, url, branch, kind string }{
		{"acme-web", "origin", "git@github.com:acme/acme-web.git", "main", "github"},
		{"acme-api", "origin", "git@github.com:acme/acme-api.git", "main", "github"},
		{"beta-mobile", "origin", "git@gitlab.com:beta/beta-mobile.git", "develop", "gitlab"},
		{"internal-ops", "origin", "git@github.com:org/internal-ops.git", "main", "github"},
	}
	return withOrg(ctx, pool, orgID, func(tx pgx.Tx) error {
		for _, r := range repos {
			pid, err := demoProjectID(ctx, tx, orgID, r.projectSlug)
			if err != nil {
				return err
			}
			ct, err := tx.Exec(ctx,
				`INSERT INTO project_repositories
				   (organization_id, project_id, name, url, branch_default, kind, is_default)
				 VALUES ($1,$2,$3,$4,$5,$6,true)
				 ON CONFLICT (organization_id, project_id, name) DO NOTHING`,
				orgID, pid, r.name, r.url, r.branch, r.kind,
			)
			if err != nil {
				return fmt.Errorf("repo %s: %w", r.projectSlug, err)
			}
			if ct.RowsAffected() > 0 {
				tot.Repos++
			}
		}
		return nil
	})
}

// project_policies kind debe estar en el CHECK:
//   convention|security_rule|architecture|sdd_workflow|observability|
//   migration_rule|linter_config|agent_protocol|git_workflow|tech_stack|
//   test_strategy
func seedDemoPolicies(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, tot *seedTotals) error {
	policies := []struct{ projectSlug, slug, name, kind, body string }{
		{"acme-web", "tech-stack", "Stack frontend", "tech_stack", "React 19 + Vite + Tailwind"},
		{"acme-web", "eslint", "ESLint config", "linter_config", "Usar eslint-config-airbnb. Sin warnings permitidos."},
		{"acme-api", "tech-stack", "Stack backend", "tech_stack", "Go 1.22 + pgx/v5 + chi"},
		{"acme-api", "agent-protocol", "AGENTS.md", "agent_protocol", "Antes de tocar handlers, leer pkg/middleware/auth.go"},
		{"beta-mobile", "tech-stack", "Stack mobile", "tech_stack", "Flutter 3 + Riverpod"},
		{"internal-ops", "commits", "Conventional Commits", "git_workflow", "Commits en español, sin Co-Authored-By"},
	}
	return withOrg(ctx, pool, orgID, func(tx pgx.Tx) error {
		for _, p := range policies {
			pid, err := demoProjectID(ctx, tx, orgID, p.projectSlug)
			if err != nil {
				return err
			}
			ct, err := tx.Exec(ctx,
				`INSERT INTO project_policies
				   (organization_id, project_id, slug, name, kind, body_md, source)
				 VALUES ($1,$2,$3,$4,$5,$6,'seed_imported')
				 ON CONFLICT (organization_id, project_id, slug, is_active)
				 DO UPDATE SET name=EXCLUDED.name, body_md=EXCLUDED.body_md, updated_at=NOW()`,
				orgID, pid, p.slug, p.name, p.kind, p.body,
			)
			if err != nil {
				return fmt.Errorf("policy %s/%s: %w", p.projectSlug, p.slug, err)
			}
			if ct.RowsAffected() > 0 {
				tot.Policies++
			}
		}
		return nil
	})
}

func seedDemoKnowledge(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, tot *seedTotals) error {
	// knowledge_docs no tiene UNIQUE natural — usamos metadata.demo_slug
	// para detectar duplicados sin migration nueva.
	docs := []struct {
		projectSlug, slug, title, body, source string
		tags                                   []string
	}{
		{"acme-web", "readme", "Acme Web README", "Frontend Acme. npm install + npm run dev.", "manual", []string{"readme"}},
		{"acme-web", "deploy", "Acme Web deployment", "Vercel via push a main.", "manual", []string{"deploy"}},
		{"acme-api", "readme", "Acme API README", "Backend Acme. make run.", "manual", []string{"readme"}},
		{"acme-api", "rls", "RLS design", "Multi-tenant via app.current_org_id GUC + FORCE RLS.", "manual", []string{"security"}},
		{"acme-api", "ci", "CI pipeline", "GitHub Actions: build, test, deploy.", "manual", []string{"ci"}},
		{"beta-mobile", "readme", "Beta mobile README", "Flutter app.", "manual", []string{"readme"}},
		{"internal-ops", "restore-runbook", "Restore DB runbook", "1. Stop. 2. pg_restore. 3. Verify.", "manual", []string{"runbook"}},
		{"internal-ops", "auth-rework", "Auth rework spec", "Eliminamos sessions middleware. JWT short-lived.", "manual", []string{"spec"}},
	}
	return withOrg(ctx, pool, orgID, func(tx pgx.Tx) error {
		for _, d := range docs {
			pid, err := demoProjectID(ctx, tx, orgID, d.projectSlug)
			if err != nil {
				return err
			}
			meta := fmt.Sprintf(`{"demo_slug":"%s"}`, d.slug)
			ct, err := tx.Exec(ctx,
				`INSERT INTO knowledge_docs
				   (organization_id, project_id, title, body, source, tags, metadata)
				 SELECT $1,$2,$3,$4,$5,$6,$7::jsonb
				 WHERE NOT EXISTS (
				   SELECT 1 FROM knowledge_docs
				   WHERE organization_id=$1 AND project_id=$2
				     AND metadata->>'demo_slug' = $8
				     AND deleted_at IS NULL
				 )`,
				orgID, pid, d.title, d.body, d.source, d.tags, meta, d.slug,
			)
			if err != nil {
				return fmt.Errorf("knowledge %s/%s: %w", d.projectSlug, d.slug, err)
			}
			if ct.RowsAffected() > 0 {
				tot.Knowledge++
			}
		}
		return nil
	})
}

func seedDemoTickets(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, tot *seedTotals) error {
	statuses := []string{"backlog", "todo", "in_progress", "in_review", "blocked", "done", "cancelled"}
	types := []string{"bug", "feature", "requirement", "task", "epic", "improvement", "spike"}
	prios := []string{"trivial", "low", "medium", "high", "critical"}
	projects := []string{"acme-web", "acme-api", "beta-mobile", "internal-ops"}
	emails := []string{"alice@demo.test", "bob@demo.test", "carol@demo.test", "dave@demo.test", "eve@demo.test"}

	return withOrg(ctx, pool, orgID, func(tx pgx.Tx) error {
		reporter, err := demoUserID(ctx, tx, "alice@demo.test")
		if err != nil {
			return fmt.Errorf("alice not seeded: %w", err)
		}
		// Para no colisionar con tickets pre-existentes en cada proyecto,
		// usamos MAX(number)+1 como base y vamos incrementando por
		// proyecto. Idempotencia por (org, project, key) sigue funcionando
		// porque el key incluye un sufijo determinístico "demo-N".
		nextNum := map[string]int{}
		for _, slug := range projects {
			pid, err := demoProjectID(ctx, tx, orgID, slug)
			if err != nil {
				return err
			}
			var base int
			if err := tx.QueryRow(ctx,
				`SELECT COALESCE(MAX(number),0) FROM project_tickets
				 WHERE organization_id=$1 AND project_id=$2`,
				orgID, pid,
			).Scan(&base); err != nil {
				return fmt.Errorf("max number %s: %w", slug, err)
			}
			nextNum[slug] = base + 1
		}
		for i := 0; i < 30; i++ {
			projectSlug := projects[i%len(projects)]
			pid, err := demoProjectID(ctx, tx, orgID, projectSlug)
			if err != nil {
				return err
			}
			assignee, err := demoUserID(ctx, tx, emails[i%len(emails)])
			if err != nil {
				return err
			}
			number := nextNum[projectSlug]
			nextNum[projectSlug]++
			key := fmt.Sprintf("DEMO-%d", i+1)
			var ticketID uuid.UUID
			err = tx.QueryRow(ctx,
				`INSERT INTO project_tickets
				   (organization_id, project_id, key, number,
				    title, description_md, issue_type, priority, status,
				    assignee_id, reporter_id, labels)
				 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
				 ON CONFLICT (organization_id, project_id, key) DO UPDATE
				   SET title=EXCLUDED.title
				 RETURNING id`,
				orgID, pid, key, number,
				fmt.Sprintf("Demo ticket #%d (%s)", i+1, types[i%len(types)]),
				"Auto-generado por seed-demo (REQ-64).",
				types[i%len(types)], prios[i%len(prios)], statuses[i%len(statuses)],
				assignee, reporter, []string{"demo", prios[i%len(prios)]},
			).Scan(&ticketID)
			if err != nil {
				return fmt.Errorf("ticket %s: %w", key, err)
			}
			tot.Tickets++

			for j := 0; j < 2; j++ {
				body := fmt.Sprintf("Comentario %d en %s — demo", j+1, key)
				ct, err := tx.Exec(ctx,
					`INSERT INTO project_ticket_comments (ticket_id, author_id, body_md)
					 SELECT $1,$2,$3
					 WHERE NOT EXISTS (
					   SELECT 1 FROM project_ticket_comments
					   WHERE ticket_id=$1 AND body_md=$3 AND deleted_at IS NULL
					 )`,
					ticketID, assignee, body,
				)
				if err != nil {
					return fmt.Errorf("comment %s/%d: %w", key, j, err)
				}
				if ct.RowsAffected() > 0 {
					tot.Comments++
				}
			}

			ct, err := tx.Exec(ctx,
				`INSERT INTO project_ticket_status_history
				   (ticket_id, from_status, to_status, changed_by, note)
				 SELECT $1,'backlog',$2,$3,'demo transition'
				 WHERE NOT EXISTS (
				   SELECT 1 FROM project_ticket_status_history
				   WHERE ticket_id=$1 AND note='demo transition'
				 )`,
				ticketID, statuses[i%len(statuses)], assignee,
			)
			if err != nil {
				return err
			}
			if ct.RowsAffected() > 0 {
				tot.History++
			}
		}
		return nil
	})
}

func seedDemoPrompts(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, tot *seedTotals) error {
	return withOrg(ctx, pool, orgID, func(tx pgx.Tx) error {
		userID, err := demoUserID(ctx, tx, "alice@demo.test")
		if err != nil {
			return err
		}
		pid, err := demoProjectID(ctx, tx, orgID, "acme-web")
		if err != nil {
			return err
		}
		base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
		for i := 0; i < 50; i++ {
			content := fmt.Sprintf("Demo prompt #%d sobre testing y fixtures", i+1)
			charCount := len(content)
			var turnComplete any
			if i%3 == 0 {
				turnComplete = base.Add(time.Duration(i) * time.Minute)
			}
			ct, err := tx.Exec(ctx,
				`INSERT INTO captured_prompts
				   (organization_id, user_id, project_id, content,
				    client_kind, model, char_count, response_chars,
				    estimated_tokens_in, estimated_tokens_out, turn_completed_at)
				 SELECT $1,$2,$3,$4,'claude-code','claude-opus-4-7',$5,$6,$7,$8,$9
				 WHERE NOT EXISTS (
				   SELECT 1 FROM captured_prompts
				   WHERE organization_id=$1 AND content=$4
				 )`,
				orgID, userID, pid, content,
				charCount, charCount*3, charCount/4, charCount*3/4, turnComplete,
			)
			if err != nil {
				return fmt.Errorf("prompt %d: %w", i, err)
			}
			if ct.RowsAffected() > 0 {
				tot.Prompts++
			}
		}
		return nil
	})
}

func seedDemoObservations(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, tot *seedTotals) error {
	return withOrg(ctx, pool, orgID, func(tx pgx.Tx) error {
		pid, err := demoProjectID(ctx, tx, orgID, "acme-api")
		if err != nil {
			return err
		}
		types := []string{"decision", "bug", "convention", "gotcha"}
		for i := 0; i < 12; i++ {
			content := fmt.Sprintf("Demo observation #%d — pattern descubierto", i+1)
			ct, err := tx.Exec(ctx,
				`INSERT INTO observations
				   (organization_id, project_id, observation_type, content, tags)
				 SELECT $1,$2,$3,$4,$5
				 WHERE NOT EXISTS (
				   SELECT 1 FROM observations
				   WHERE organization_id=$1 AND content=$4 AND deleted_at IS NULL
				 )`,
				orgID, pid, types[i%len(types)], content, []string{"demo", "auto"},
			)
			if err != nil {
				return fmt.Errorf("obs %d: %w", i, err)
			}
			if ct.RowsAffected() > 0 {
				tot.Observations++
			}
		}
		return nil
	})
}

func seedDemoVerifications(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, tot *seedTotals) error {
	return withOrg(ctx, pool, orgID, func(tx pgx.Tx) error {
		pid, err := demoProjectID(ctx, tx, orgID, "acme-api")
		if err != nil {
			return err
		}
		userID, err := demoUserID(ctx, tx, "carol@demo.test")
		if err != nil {
			return err
		}
		kinds := []string{"build", "test", "lint", "smoke", "typecheck", "migration"}
		statuses := []string{"passed", "failed", "partial", "running", "pending", "passed"}
		for i := 0; i < 6; i++ {
			ctxNote := fmt.Sprintf("Demo verification ctx #%d", i+1)
			items := fmt.Sprintf(`[{"label":"check-%d","status":"%s","duration_ms":%d}]`,
				i+1, statuses[i], 100+i*50)
			ct, err := tx.Exec(ctx,
				`INSERT INTO verifications
				   (organization_id, project_id, user_id, kind, items, status, context)
				 SELECT $1,$2,$3,$4,$5::jsonb,$6,$7
				 WHERE NOT EXISTS (
				   SELECT 1 FROM verifications
				   WHERE organization_id=$1 AND context=$7
				 )`,
				orgID, pid, userID, kinds[i], items, statuses[i], ctxNote,
			)
			if err != nil {
				return fmt.Errorf("verif %d: %w", i, err)
			}
			if ct.RowsAffected() > 0 {
				tot.Verifications++
			}
		}
		return nil
	})
}

func seedDemoIndexRuns(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, tot *seedTotals) error {
	return withOrg(ctx, pool, orgID, func(tx pgx.Tx) error {
		pid, err := demoProjectID(ctx, tx, orgID, "acme-api")
		if err != nil {
			return err
		}
		userID, err := demoUserID(ctx, tx, "alice@demo.test")
		if err != nil {
			return err
		}
		ct, err := tx.Exec(ctx,
			`INSERT INTO project_index_runs
			   (organization_id, project_id, started_by, status,
			    git_head, files_submitted, summary, completed_at)
			 SELECT $1,$2,$3,'completed','abc123demo',42,
			        '{"policies_created":4,"knowledge_created":6,"skills_created":0,"observations_created":2,"ignored":[]}'::jsonb,
			        NOW()
			 WHERE NOT EXISTS (
			   SELECT 1 FROM project_index_runs
			   WHERE organization_id=$1 AND project_id=$2 AND git_head='abc123demo'
			 )`,
			orgID, pid, userID,
		)
		if err != nil {
			return err
		}
		if ct.RowsAffected() > 0 {
			tot.IndexRuns++
		}
		return nil
	})
}
