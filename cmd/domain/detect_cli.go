// F2+F3+F4: runDomainDetect — handler para `domain` sin args.
// Detecta el proyecto en el CWD, hace auto-link por git_remote si no existe
// por slug, muestra el inventario COMPLETO de capabilities, update branch
// si difiere, e inicia session automáticamente.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/inventory"
	"nunezlagos/domain/internal/service/projectdetect"
	"nunezlagos/domain/internal/service/projectlink"
)

func runDomainDetect(ctx context.Context) int {
	meta, err := projectdetect.Detect("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "domain: no se detectó proyecto (%v)\n", err)
		fmt.Fprintln(os.Stderr, "tip: ejecutá desde la raíz de un repo git o donde haya go.mod/composer.json/package.json")
		return 1
	}

	pool, err := openPool()
	if err != nil {
		fmt.Fprintln(os.Stderr, "db:", err)
		return 1
	}
	defer pool.Close()

	link := projectlink.New(pool)
	projectID, orgID, resolvedSlug, linkNotes := resolveProject(ctx, link, pool, meta)

	loadIn := inventory.LoadInput{}
	if orgID != "" {
		loadIn.OrgID = &orgID
	}
	inv, err := inventory.New(pool).Load(ctx, loadIn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inventory: %v\n", err)
		return 1
	}

	var sessionID string
	if projectID != "" && meta.CurrentBranch != "" {
		if uerr := link.UpdateBranch(ctx, projectID, meta.CurrentBranch); uerr != nil {
			fmt.Fprintf(os.Stderr, "branch update: %v\n", uerr)
		}
	}
	if projectID != "" {
		sessionID, _ = startSession(ctx, pool, projectID, resolvedSlug)
	}

	out := map[string]any{
		"project":       meta,
		"project_id":    projectID,
		"org_id":        orgID,
		"resolved_slug": resolvedSlug,
		"link_notes":    linkNotes,
		"inventory":     inv,
		"session":       sessionID,
	}
	encoded, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(encoded))
	return 0
}

func resolveProject(ctx context.Context, link *projectlink.Service, pool *pgxpool.Pool, meta *projectdetect.Metadata) (projectID, orgID, slug string, notes []string) {
	notes = append(notes, fmt.Sprintf("detected_slug=%s", meta.ProjectSlug))

	if pid, oid, s, err := lookupBySlug(ctx, pool, meta.ProjectSlug); err == nil && pid != "" {
		notes = append(notes, "linked by slug")
		return pid, oid, s, notes
	}
	notes = append(notes, fmt.Sprintf("not found by slug %q", meta.ProjectSlug))

	if meta.GitRemoteURL == "" {
		notes = append(notes, "no git_remote to link by")
		return "", "", "", notes
	}
	pid, oid, s, err := link.LinkByGitRemote(ctx, meta.GitRemoteURL)
	if err != nil {
		notes = append(notes, fmt.Sprintf("link-by-remote error: %v", err))
		return "", "", "", notes
	}
	if pid == "" {
		notes = append(notes, fmt.Sprintf("no project matches git_remote %q", meta.GitRemoteURL))
		return "", "", "", notes
	}
	notes = append(notes, fmt.Sprintf("linked by git_remote → %s", s))
	return pid, oid, s, notes
}

func lookupBySlug(ctx context.Context, pool *pgxpool.Pool, projectSlug string) (projectID, orgID, slug string, err error) {
	err = pool.QueryRow(ctx, `
		SELECT id::text, organization_id::text, slug
		FROM projects
		WHERE slug = $1 AND deleted_at IS NULL
		LIMIT 1
	`, projectSlug).Scan(&projectID, &orgID, &slug)
	return
}

func startSession(ctx context.Context, pool *pgxpool.Pool, projectID, projectSlug string) (string, error) {
	var userID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM users LIMIT 1`).Scan(&userID); err != nil {
		return "", fmt.Errorf("no users to attribute session: %w", err)
	}
	var sessionID string
	err := pool.QueryRow(ctx, `
		INSERT INTO sessions (organization_id, project_id, user_id, title, tags)
		SELECT organization_id, id, $2::uuid, $3, ARRAY['auto-detect']
		FROM projects WHERE id = $1::uuid AND deleted_at IS NULL
		RETURNING id::text
	`, projectID, userID, "auto-detect: "+projectSlug).Scan(&sessionID)
	if err != nil {
		return "", err
	}
	return sessionID, nil
}
