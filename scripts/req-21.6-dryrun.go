

















//go:build req_21_6_dryrun

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/services/domain-backend/internal/migrate"
)

func main() {
	preCounts := flag.String("pre-counts", "", "path a row-counts-pre-N.txt de prod")
	migration := flag.String("migration", "", "path al .up.sql de la migración a dry-run")
	migDir := flag.String("mig-dir", "", "directorio de migraciones (para dmigrate.Up)")
	flag.Parse()

	if *preCounts == "" || *migration == "" || *migDir == "" {
		log.Fatal("flags requeridos: --pre-counts, --migration, --mig-dir")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("dryrun"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		log.Fatalf("postgres.Run: %v", err)
	}
	defer func() { _ = pgC.Terminate(ctx) }()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("ConnectionString: %v", err)
	}





	if err := dmigrate.Up(dsn); err != nil {
		log.Fatalf("dmigrate.Up: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()


	migSQL, err := os.ReadFile(*migration)
	if err != nil {
		log.Fatalf("ReadFile migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(migSQL)); err != nil {
		log.Fatalf("Exec migration: %v", err)
	}


	postCounts, err := countRows(ctx, pool)
	if err != nil {
		log.Fatalf("countRows post: %v", err)
	}


	preData, err := os.ReadFile(*preCounts)
	if err != nil {
		log.Fatalf("ReadFile pre-counts: %v", err)
	}
	preMap := parseCounts(string(preData))


	var diffs []string
	for table, post := range postCounts {
		pre, hasPre := preMap[table]
		if !hasPre {
			diffs = append(diffs, fmt.Sprintf("+ %s: nueva tabla con %d filas", table, post))
			continue
		}
		if pre != post {
			diffs = append(diffs, fmt.Sprintf("! %s: pre=%d post=%d (diff=%d)", table, pre, post, post-pre))
		}
	}
	for table := range preMap {
		if _, hasPost := postCounts[table]; !hasPost {
			diffs = append(diffs, fmt.Sprintf("- %s: tabla desaparecida", table))
		}
	}


	fmt.Println(dsn) // primera línea: DSN (el bash script lo lee)
	if len(diffs) == 0 {
		fmt.Println("DRY_RUN_OK: 0 diferencias en conteo de filas")
		os.Exit(0)
	}
	fmt.Println("DRY_RUN_FAIL: diferencias encontradas:")
	for _, d := range diffs {
		fmt.Println("  ", d)
	}
	os.Exit(1)
}

func countRows(ctx context.Context, pool *pgxpool.Pool) (map[string]int, error) {
	rows, err := pool.Query(ctx, `
		SELECT table_name,
		       (xpath('/row/c/text()',
		         query_to_xml(format('SELECT COUNT(*) AS c FROM %I.%I', 'public', table_name),
		                      false, true, '')))[1]::text::int AS n
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			return nil, err
		}
		out[name] = n
	}
	return out, rows.Err()
}

func parseCounts(raw string) map[string]int {
	out := make(map[string]int)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		var n int
		fmt.Sscanf(parts[1], "%d", &n)
		out[parts[0]] = n
	}
	return out
}
