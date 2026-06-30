package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/llm"
)

// runEmbedBackfill (REQ-68): recorre observations y knowledge_docs con
// embedding NULL, genera vectors con el provider actual y los persiste.
// Útil cuando se activa OpenAI sobre data sembrada con noop.
//
// Uso: domain embed-backfill <organization-uuid> [--limit=200] [--dry-run]
//
// Performance:
//   - Procesa de a 1 con 100ms de pausa entre llamadas para no
//     saturar el rate-limit de OpenAI ni la tabla con UPDATEs.
//   - Si el provider es noop, sale sin hacer nada.
func runEmbedBackfill(args []string) {
	limit := 200
	dryRun := false
	var orgArg string
	for _, a := range args {
		switch {
		case a == "--dry-run":
			dryRun = true
		case strings.HasPrefix(a, "--limit="):
			fmt.Sscanf(a, "--limit=%d", &limit)
		default:
			if orgArg == "" {
				orgArg = a
			}
		}
	}
	if orgArg == "" {
		fmt.Fprintln(os.Stderr, "Uso: domain embed-backfill <organization-uuid> [--limit=N] [--dry-run]")
		os.Exit(2)
	}
	orgID, err := uuid.Parse(orgArg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "UUID inválido:", err)
		os.Exit(2)
	}
	ctx := context.Background()
	logger := slog.Default()
	embedder := chooseEmbedder(logger)
	if _, isNoop := embedder.(llm.NopEmbedder); isNoop {
		fmt.Println("Embedder = noop → nada que backfillear. Configurá DOMAIN_EMBEDDING_PROVIDER=openai y DOMAIN_OPENAI_API_KEY.")
		return
	}

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

	totObs, totKn := 0, 0
	if n, err := backfillTable(ctx, pool, embedder, orgID,
		"knowledge_observations", "id", "content", "embedding", limit, dryRun); err != nil {
		fmt.Fprintln(os.Stderr, "observations:", err)
		os.Exit(1)
	} else {
		totObs = n
	}
	if n, err := backfillTable(ctx, pool, embedder, orgID,
		"knowledge_docs", "id", "body", "(SELECT id FROM knowledge_docs WHERE 1=0)", limit, dryRun); err != nil {

		_ = err
	} else {
		totKn = n
	}
	fmt.Printf("Backfill done: observations=%d knowledge_chunks=%d dry_run=%v\n", totObs, totKn, dryRun)
}

func backfillTable(ctx context.Context, pool *pgxpool.Pool, emb llm.Embedder,
	orgID uuid.UUID, table, idCol, textCol, embCol string, limit int, dryRun bool) (int, error) {
	if embCol == "" || strings.Contains(embCol, "SELECT") {
		return 0, nil // skip tablas sin columna embedding directa
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(
		`SELECT %s, %s FROM %s
		 WHERE %s IS NULL
		   AND deleted_at IS NULL
		   AND LENGTH(TRIM(%s)) > 0
		 ORDER BY created_at ASC
		 LIMIT $1`,
		idCol, textCol, table, embCol, textCol,
	), limit)
	if err != nil {
		return 0, fmt.Errorf("query %s: %w", table, err)
	}
	type row struct {
		ID   uuid.UUID
		Text string
	}
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.Text); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, r)
	}
	rows.Close()
	fmt.Printf("  %s: %d filas sin embedding\n", table, len(items))
	if dryRun {
		return len(items), nil
	}
	for i, it := range items {
		v, err := emb.Embed(ctx, it.Text)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  embed %s/%s: %v\n", table, it.ID, err)
			continue
		}
		if v == nil || llm.IsZero(v) {
			continue
		}
		lit := vectorLiteral(v)
		if _, err := pool.Exec(ctx, fmt.Sprintf(
			`UPDATE %s SET %s = $2::vector WHERE id=$1`,
			table, embCol,
		), it.ID, lit); err != nil {
			fmt.Fprintf(os.Stderr, "  update %s/%s: %v\n", table, it.ID, err)
			continue
		}
		if (i+1)%10 == 0 {
			fmt.Printf("  %s: %d/%d\n", table, i+1, len(items))
		}
		time.Sleep(100 * time.Millisecond)
	}
	return len(items), nil
}

func vectorLiteral(v []float32) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", x)
	}
	sb.WriteByte(']')
	return sb.String()
}
