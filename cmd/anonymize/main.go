// Command anonymize copia datos desde un Postgres source a uno dest
// aplicando reglas de anonimización (issue-25.11).
//
// Uso:
//
//	anonymize -src "postgres://..." -dst "postgres://..." -seed 42
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/anonymizer"
)

func main() {
	src := flag.String("src", "", "DSN Postgres source (lectura)")
	dst := flag.String("dst", "", "DSN Postgres dest (escritura)")
	seed := flag.Int64("seed", 1, "Seed para fakers determinísticos")
	flag.Parse()

	if *src == "" || *dst == "" {
		fmt.Fprintln(os.Stderr, "usage: anonymize -src <DSN> -dst <DSN> [-seed N]")
		os.Exit(2)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := context.Background()

	srcPool, err := pgxpool.New(ctx, *src)
	if err != nil {
		logger.Error("src pool", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer srcPool.Close()
	dstPool, err := pgxpool.New(ctx, *dst)
	if err != nil {
		logger.Error("dst pool", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer dstPool.Close()

	cfg := anonymizer.DefaultConfig()
	cfg.Seed = *seed
	a := &anonymizer.Anonymizer{
		Src: srcPool, Dst: dstPool, Cfg: cfg, Logger: logger,
	}
	if err := a.Run(ctx); err != nil {
		logger.Error("anonymize", slog.String("err", err.Error()))
		os.Exit(1)
	}
	logger.Info("anonymize done")
}
