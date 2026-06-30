// Command domain-schema-drift — issue-25.4 detecta drift entre schema real
// y schema esperado por las migraciones.
//
// Estrategia:
//  1. Conecta al DB real (DOMAIN_DATABASE_URL).
//  2. Crea DB temporal y aplica todas las migrations embebidas.
//  3. Volcado lógico de cada DB (pg_dump --schema-only --no-owner).
//  4. Diff normalizado entre ambos.
//  5. Exit 0 si idénticos, 1 si difieren (con report).
//
// Diseñado para correr en cron diario. Output JSON parseable para feed a
// alertmanager (issue-17.1 metrics).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type result struct {
	Timestamp    time.Time `json:"timestamp"`
	Drift        bool      `json:"drift"`
	Summary      string    `json:"summary"`
	DiffLines    int       `json:"diff_lines"`
	DiffSample   string    `json:"diff_sample,omitempty"`
	PrimaryDSN   string    `json:"primary_dsn_obfuscated"`
	ExpectedDSN  string    `json:"expected_dsn_obfuscated"`
	DurationSecs float64   `json:"duration_secs"`
}

func main() {
	out := flag.String("output", "", "ruta para escribir reporte JSON (stdout si vacío)")
	timeoutSec := flag.Int("timeout-sec", 300, "timeout total")
	flag.Parse()

	start := time.Now()
	dsn := os.Getenv("DOMAIN_DATABASE_URL")
	if dsn == "" {
		fail("DOMAIN_DATABASE_URL no seteada", *out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
	defer cancel()

	primarySchema, err := dumpSchema(ctx, dsn)
	if err != nil {
		fail(fmt.Sprintf("dump primary: %v", err), *out)
	}


	tmpName := fmt.Sprintf("domain_drift_check_%d", time.Now().Unix())
	if err := createTempDB(ctx, dsn, tmpName); err != nil {
		fail(fmt.Sprintf("create temp db: %v", err), *out)
	}
	defer dropTempDB(context.Background(), dsn, tmpName)

	tmpDSN := replaceDBName(dsn, tmpName)
	if err := applyMigrations(tmpDSN); err != nil {
		fail(fmt.Sprintf("apply migrations: %v", err), *out)
	}

	expectedSchema, err := dumpSchema(ctx, tmpDSN)
	if err != nil {
		fail(fmt.Sprintf("dump expected: %v", err), *out)
	}

	diff := diffSchemas(normalize(primarySchema), normalize(expectedSchema))

	res := result{
		Timestamp:    time.Now().UTC(),
		Drift:        diff != "",
		PrimaryDSN:   obfuscate(dsn),
		ExpectedDSN:  obfuscate(tmpDSN),
		DurationSecs: time.Since(start).Seconds(),
	}
	if res.Drift {
		res.Summary = "DRIFT DETECTED — schema real difiere de migrations expected"
		res.DiffLines = strings.Count(diff, "\n")
		const sampleMax = 4000
		if len(diff) > sampleMax {
			res.DiffSample = diff[:sampleMax] + "\n... (truncated)"
		} else {
			res.DiffSample = diff
		}
	} else {
		res.Summary = "OK — schema real coincide con migrations"
	}

	writeReport(res, *out)
	if res.Drift {
		os.Exit(1)
	}
}

func dumpSchema(ctx context.Context, dsn string) (string, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)



	rows, err := conn.Query(ctx, `
		SELECT
			n.nspname AS schema,
			c.relname AS table,
			c.relkind,
			a.attname AS column,
			format_type(a.atttypid, a.atttypmod) AS data_type,
			a.attnotnull AS notnull,
			pg_get_expr(d.adbin, d.adrelid) AS default_expr
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
		LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		WHERE n.nspname = 'public'
		  AND c.relkind IN ('r','v','m')
		ORDER BY 1, 2, a.attnum`,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var b strings.Builder
	for rows.Next() {
		var schema, table, kind, column, dtype string
		var notnull bool
		var def *string
		if err := rows.Scan(&schema, &table, &kind, &column, &dtype, &notnull, &def); err != nil {
			return "", err
		}
		nn := ""
		if notnull {
			nn = " NOT NULL"
		}
		d := ""
		if def != nil {
			d = " DEFAULT " + *def
		}
		fmt.Fprintf(&b, "%s %s.%s %s %s%s%s\n", kind, schema, table, column, dtype, nn, d)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func normalize(s string) string {

	lines := strings.Split(strings.TrimSpace(s), "\n")

	return strings.Join(lines, "\n")
}

func diffSchemas(a, b string) string {
	aLines := strings.Split(a, "\n")
	bLines := strings.Split(b, "\n")
	aMap := map[string]bool{}
	for _, l := range aLines {
		aMap[l] = true
	}
	bMap := map[string]bool{}
	for _, l := range bLines {
		bMap[l] = true
	}

	var diff strings.Builder
	for l := range aMap {
		if !bMap[l] {
			fmt.Fprintf(&diff, "- %s\n", l)
		}
	}
	for l := range bMap {
		if !aMap[l] {
			fmt.Fprintf(&diff, "+ %s\n", l)
		}
	}
	return diff.String()
}

func createTempDB(ctx context.Context, dsn, name string) error {
	adminDSN := replaceDBName(dsn, "postgres")
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	return err
}

func dropTempDB(ctx context.Context, dsn, name string) {
	adminDSN := replaceDBName(dsn, "postgres")
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return
	}
	defer conn.Close(ctx)
	_, _ = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize())
}

func replaceDBName(dsn, newDB string) string {

	idx := strings.LastIndex(dsn, "/")
	if idx < 0 {
		return dsn
	}
	rest := dsn[idx+1:]
	q := strings.Index(rest, "?")
	if q < 0 {
		return dsn[:idx+1] + newDB
	}
	return dsn[:idx+1] + newDB + rest[q:]
}

func obfuscate(dsn string) string {
	at := strings.Index(dsn, "@")
	if at < 0 {
		return dsn
	}
	sl := strings.Index(dsn, "://")
	if sl < 0 {
		return dsn
	}
	return dsn[:sl+3] + "***@" + dsn[at+1:]
}

func applyMigrations(dsn string) error {

	cmd := exec.Command("domain", "migrate", "up")
	cmd.Env = append(os.Environ(), "DOMAIN_DATABASE_URL="+dsn)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

func writeReport(r result, path string) {
	data, _ := json.MarshalIndent(r, "", "  ")
	if path == "" {
		fmt.Println(string(data))
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
	}
}

func fail(reason, outPath string) {
	r := result{
		Timestamp: time.Now().UTC(),
		Drift:     true,
		Summary:   "ERROR: " + reason,
	}
	writeReport(r, outPath)
	os.Exit(2)
}
