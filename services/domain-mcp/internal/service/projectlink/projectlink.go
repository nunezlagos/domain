package projectlink

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/projectlink/projectlinkdb"
	"nunezlagos/domain/internal/store/txctx"
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

func (s *Service) q(ctx context.Context) *projectlinkdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return projectlinkdb.New(tx)
	}
	return projectlinkdb.New(s.Pool)
}

func (s *Service) LinkByGitRemote(ctx context.Context, remote string) (projectID, orgID, slug string, err error) {
	candidates := remoteCandidates(remote)
	if len(candidates) == 0 {
		return "", "", "", fmt.Errorf("empty remote")
	}

	result, err := s.q(ctx).LinkByGitRemote(ctx, candidates)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", nil
		}
		return "", "", "", err
	}
	return result.ID, result.OrganizationID, result.Slug, nil
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

	pid, err := uuid.Parse(projectID)
	if err != nil {
		return fmt.Errorf("invalid projectID: %w", err)
	}

	return s.q(ctx).UpdateBranch(ctx, projectlinkdb.UpdateBranchParams{
		ProjectID: pid,
		Branch:    &branch,
	})
}

func (s *Service) GetRules(ctx context.Context, projectID string) ([]string, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid projectID: %w", err)
	}

	rules, err := s.q(ctx).GetRules(ctx, pid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	return rules, nil
}

func (s *Service) SetRules(ctx context.Context, projectID string, rules []string) error {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return fmt.Errorf("invalid projectID: %w", err)
	}

	if rules == nil {
		rules = []string{}
	}

	return s.q(ctx).SetRules(ctx, projectlinkdb.SetRulesParams{
		ProjectID: pid,
		Rules:     rules,
	})
}

func normalizeRemote(remote string) string {
	r := strings.TrimSpace(remote)
	r = strings.TrimSuffix(r, ".git")
	return r
}
