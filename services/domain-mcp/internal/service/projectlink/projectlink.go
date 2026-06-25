





package projectlink

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool}
}

var (
	ErrProjectNotFound = errors.New("project not found by git_remote")
	ErrInvalidBranch   = errors.New("invalid branch name")
)

func (s *Service) LinkByGitRemote(ctx context.Context, remote string) (projectID, orgID, slug string, err error) {
	candidates := remoteCandidates(remote)
	if len(candidates) == 0 {
		return "", "", "", fmt.Errorf("empty remote")
	}




	err = s.Pool.QueryRow(ctx, `
		SELECT id::text, slug
		FROM projects
		WHERE deleted_at IS NULL
		  AND repository_url = ANY($1::text[])
		ORDER BY array_position($1::text[], repository_url)
		LIMIT 1
	`, candidates).Scan(&projectID, &slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", nil
		}
		return "", "", "", err
	}
	return projectID, orgID, slug, nil
}

func remoteCandidates(remote string) []string {
	r := strings.TrimSpace(remote)
	if r == "" {
		return nil
	}
	out := []string{r, strings.TrimSuffix(r, ".git")}

	if strings.HasPrefix(r, "git@") {
		rest := strings.TrimPrefix(r, "git@")
		if idx := strings.Index(rest, ":"); idx >= 0 {
			host := rest[:idx]
			path := rest[idx+1:]
			out = append(out, "https://"+host+"/"+path, "https://"+host+"/"+strings.TrimSuffix(path, ".git"))
		}
	}
	return out
}

func (s *Service) UpdateBranch(ctx context.Context, projectID, branch string) error {
	if strings.ContainsAny(branch, " \t\n;") {
		return ErrInvalidBranch
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE projects
		SET current_branch = $2, updated_at = NOW()
		WHERE id = $1::uuid AND deleted_at IS NULL
	`, projectID, branch)
	return err
}

func (s *Service) GetRules(ctx context.Context, projectID string) ([]string, error) {
	var rules []string
	err := s.Pool.QueryRow(ctx, `
		SELECT rules FROM projects WHERE id = $1::uuid AND deleted_at IS NULL
	`, projectID).Scan(&rules)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	return rules, nil
}

func (s *Service) SetRules(ctx context.Context, projectID string, rules []string) error {
	if rules == nil {
		rules = []string{}
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE projects
		SET rules = $2, updated_at = NOW()
		WHERE id = $1::uuid AND deleted_at IS NULL
	`, projectID, rules)
	return err
}

func normalizeRemote(remote string) string {
	r := strings.TrimSpace(remote)
	r = strings.TrimSuffix(r, ".git")
	return r
}
