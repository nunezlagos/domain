package anonymizer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Anonymizer copia rows desde un pool source a uno dest, aplicando
// transformaciones por columna.
type Anonymizer struct {
	Src    *pgxpool.Pool
	Dst    *pgxpool.Pool
	Cfg    Config
	Logger *slog.Logger
}

// Run procesa todas las tablas listadas en Cfg.Tables. Para tablas no listadas,
// el caller debe usar pg_dump directo (este package no copia tablas sin policy).
//
// Asume que el schema del destino YA existe (migrations corridas en dst).
// Las rows se INSERTAN con los mismos IDs para preservar foreign keys.
func (a *Anonymizer) Run(ctx context.Context) error {
	for table, cfg := range a.Cfg.Tables {
		if cfg.Skip {
			if a.Logger != nil {
				a.Logger.InfoContext(ctx, "anonymizer skip table",
					slog.String("table", table))
			}
			continue
		}
		if err := a.copyTable(ctx, table, cfg); err != nil {
			return fmt.Errorf("table %s: %w", table, err)
		}
	}
	return nil
}

// copyTable describe el flujo: enumera columnas, SELECT * en batch, aplica
// transform por columna, INSERT en dst. Idempotente por id (ON CONFLICT DO NOTHING).
func (a *Anonymizer) copyTable(ctx context.Context, table string, cfg TableConfig) error {
	colsRows, err := a.Src.Query(ctx,
		`SELECT column_name FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = $1
		 ORDER BY ordinal_position`, table)
	if err != nil {
		return fmt.Errorf("list cols: %w", err)
	}
	defer colsRows.Close()
	var cols []string
	for colsRows.Next() {
		var c string
		if err := colsRows.Scan(&c); err != nil {
			return err
		}
		cols = append(cols, c)
	}
	if err := colsRows.Err(); err != nil {
		return err
	}
	if len(cols) == 0 {
		return nil
	}

	selectSQL := fmt.Sprintf(`SELECT %s FROM %s`, strings.Join(quoteAll(cols), ","), table)
	rows, err := a.Src.Query(ctx, selectSQL)
	if err != nil {
		return fmt.Errorf("select src: %w", err)
	}
	defer rows.Close()

	insertSQL := buildInsertSQL(table, cols)
	processed := 0
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return err
		}
		for i, col := range cols {
			rule, ok := cfg.Columns[col]
			if !ok {
				continue
			}
			values[i] = a.applyRule(rule, values[i], processed)
		}
		if _, err := a.Dst.Exec(ctx, insertSQL, values...); err != nil {
			return fmt.Errorf("insert dst row %d: %w", processed, err)
		}
		processed++
	}
	if a.Logger != nil {
		a.Logger.InfoContext(ctx, "anonymizer table done",
			slog.String("table", table),
			slog.Int("rows", processed))
	}
	return rows.Err()
}

// applyRule transforma un valor según la rule. Si la rule no aplica al tipo
// real, retorna el valor original (best-effort).
func (a *Anonymizer) applyRule(rule Rule, v any, idx int) any {
	switch rule {
	case RuleNullify:
		return nil
	case RuleFakerEmail:
		return FakerEmail(a.Cfg.Seed, idx)
	case RuleFakerName:
		return FakerName(a.Cfg.Seed, idx)
	case RuleFakerRUT:
		return FakerRUT(a.Cfg.Seed, idx)
	case RuleFakerPhone:
		return FakerPhone(a.Cfg.Seed, idx)
	case RuleRedactContent:
		s, ok := v.(string)
		if !ok {
			return v
		}
		return RedactContentTag(s)
	case RuleJSONRedact:
		raw, ok := v.([]byte)
		if !ok {
			// pgx puede devolver el JSONB como string también.
			if s, ok := v.(string); ok {
				raw = []byte(s)
			} else {
				return v
			}
		}
		return RedactJSON(raw, a.Cfg.SensitiveJSONKeys)
	default:
		return v
	}
}

func quoteAll(cols []string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = `"` + c + `"`
	}
	return out
}

func buildInsertSQL(table string, cols []string) string {
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING`,
		table,
		strings.Join(quoteAll(cols), ","),
		strings.Join(placeholders, ","),
	)
}
