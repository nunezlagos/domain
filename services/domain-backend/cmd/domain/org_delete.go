package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/config"
	orgsvc "nunezlagos/domain/internal/service/org"
)

func runOrgCmd(args []string) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Println(`Usage: domain org <command> [options]

Commands:
  delete <slug> [--confirm] [--yes]  Delete an organization permanently

Examples:
  domain org delete acme-corp --confirm`)
		return 0
	}

	switch args[0] {
	case "delete":
		return runOrgDelete(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown org command: %s\n", args[0])
		return 2
	}
}

func runOrgDelete(args []string) int {
	confirm := false
	yes := false
	slug := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--confirm":
			confirm = true
		case "--yes", "-y":
			yes = true
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
				return 2
			}
			slug = args[i]
		}
	}

	if slug == "" {
		fmt.Fprintln(os.Stderr, "usage: domain org delete <slug> [--confirm] [--yes]")
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}

	dsn := cfg.DatabaseAuthURL
	if dsn == "" {
		dsn = cfg.DatabaseURL
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pool: %v\n", err)
		return 1
	}
	defer pool.Close()

	logger := slog.Default()
	svc := orgsvc.NewDeleteService(pool, logger)

	org, err := svc.GetOrgBySlug(ctx, slug)
	if err != nil {
		fmt.Printf("org %s not found (skipping)\n", slug)
		return 0
	}

	preCounts, err := svc.PreCountOrgData(ctx, org.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pre-count failed: %v\n", err)
		return 1
	}

	fmt.Printf("ABOUT TO DELETE org '%s' (id=%s):\n", org.Slug, org.ID)
	totalRows := int64(0)
	for _, tc := range preCounts {
		fmt.Printf("  - %d %s\n", tc.Count, tc.Table)
		totalRows += tc.Count
	}
	fmt.Printf("  Total: %d rows\n", totalRows)

	if !confirm && !yes {
		fmt.Printf("\nProceed? Type 'DELETE %s' to confirm:\n", org.Slug)
		var input string
		fmt.Scanln(&input)
		if input != fmt.Sprintf("DELETE %s", org.Slug) {
			fmt.Println("aborted")
			return 1
		}
	}

	actorID := uuid.Nil
	result, err := svc.DeleteOrg(ctx, org.ID, &actorID, "cli-admin")
	if err != nil {
		fmt.Fprintf(os.Stderr, "delete failed: %v\n", err)
		return 1
	}

	fmt.Printf("org %s deleted: %d rows affected, %dms\n",
		slug, result.RowsDeleted, result.DurationMs)
	return 0
}
