// Command domain-schema-drift — issue-25.4 detecta drift entre schema real
// y schema esperado por las migraciones.
//
// Dos modos de operación (DOMAINSERV-105):
//
//  1. Default (CI/superuser): materializa el "expected" en una DB temporal
//     (CREATE DATABASE + migrations), lo dumpea y lo compara contra el real.
//  2. --expected-schema <path> (prod-safe): compara el schema real contra un
//     golden versionado en disco, SIN crear DB temporal → no requiere el
//     privilegio CREATE DATABASE (app_admin del VPS no lo tiene). El lado prod
//     queda read-only (solo SELECT sobre pg_catalog).
//  3. --generate-golden <path>: materializa el expected vía DB temporal y lo
//     escribe a archivo (para regenerar el golden en CI). No compara.
//
// Estrategia de comparación:
//  1. Conecta al DB real (DOMAIN_DATABASE_URL).
//  2. Volcado lógico normalizado (query read-only sobre pg_catalog).
//  3. Diff normalizado contra el expected (temp DB o golden).
//  4. Exit 0 si idénticos, 1 si difieren (con report), 2 en error.
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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	dmigrate "nunezlagos/domain/internal/migrate"
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
	generateGolden := flag.String("generate-golden", "", "materializa el schema esperado (crea DB temporal, requiere CREATE DATABASE / superuser de CI) y lo escribe al path indicado; no compara ni consulta prod")
	expectedSchemaPath := flag.String("expected-schema", "", "compara el schema real contra un golden versionado (prod-safe, solo lectura sobre pg_catalog); evita crear DB temporal y el privilegio CREATE DATABASE")
	flag.Parse()

	start := time.Now()
	dsn := os.Getenv("DOMAIN_DATABASE_URL")
	if dsn == "" {
		fail("DOMAIN_DATABASE_URL no seteada", *out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
	defer cancel()

	// Modo --generate-golden: materializa el expected vía DB temporal y lo
	// escribe a archivo. Corre solo en CI (necesita CREATE DATABASE). No
	// consulta prod ni compara. DOMAINSERV-105.
	if *generateGolden != "" {
		expected, _, err := buildExpectedSchema(ctx, dsn)
		if err != nil {
			fail(fmt.Sprintf("generate golden: %v", err), *out)
		}
		golden := normalize(expected)
		if err := os.WriteFile(*generateGolden, []byte(golden), 0o644); err != nil {
			fail(fmt.Sprintf("escribir golden %s: %v", *generateGolden, err), *out)
		}
		fmt.Fprintf(os.Stderr, "golden schema escrito en %s (%d bytes)\n", *generateGolden, len(golden))
		return
	}

	primarySchema, err := dumpSchema(ctx, dsn)
	if err != nil {
		fail(fmt.Sprintf("dump primary: %v", err), *out)
	}

	// El schema esperado sale de un golden versionado (--expected-schema,
	// prod-safe, sin CREATE DATABASE) o se materializa en una DB temporal
	// (default, requiere superuser → CI). DOMAINSERV-105.
	var diff, expectedDSN string
	if *expectedSchemaPath != "" {
		diff, err = compareToGolden(primarySchema, *expectedSchemaPath)
		if err != nil {
			fail(fmt.Sprintf("%v", err), *out)
		}
		expectedDSN = "golden-file:" + *expectedSchemaPath
	} else {
		expected, tmpDSN, berr := buildExpectedSchema(ctx, dsn)
		if berr != nil {
			fail(fmt.Sprintf("%v", berr), *out)
		}
		diff = diffSchemas(normalize(primarySchema), normalize(expected))
		expectedDSN = obfuscate(tmpDSN)
	}

	res := result{
		Timestamp:    time.Now().UTC(),
		Drift:        diff != "",
		PrimaryDSN:   obfuscate(dsn),
		ExpectedDSN:  expectedDSN,
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

// buildExpectedSchema materializa el schema esperado aplicando TODAS las
// migraciones embebidas en una DB temporal, y devuelve el dump crudo + el DSN
// temporal (para el reporte). Requiere privilegio CREATE DATABASE (superuser de
// CI); NO usar contra prod (app_admin no lo tiene). DOMAINSERV-105.
func buildExpectedSchema(ctx context.Context, dsn string) (string, string, error) {
	tmpName := fmt.Sprintf("domain_drift_check_%d", time.Now().Unix())
	if err := createTempDB(ctx, dsn, tmpName); err != nil {
		return "", "", fmt.Errorf("create temp db: %w", err)
	}
	defer dropTempDB(context.Background(), dsn, tmpName)

	tmpDSN := replaceDBName(dsn, tmpName)
	if err := applyMigrations(tmpDSN); err != nil {
		return "", "", fmt.Errorf("apply migrations: %w", err)
	}
	expected, err := dumpSchema(ctx, tmpDSN)
	if err != nil {
		return "", "", fmt.Errorf("dump expected: %w", err)
	}
	return expected, tmpDSN, nil
}

// compareToGolden compara el dump real contra un golden versionado en disco.
// Prod-safe: no crea DB temporal ni requiere CREATE DATABASE — solo lee el
// archivo y lo compara contra el dump (que sale de un SELECT read-only sobre
// pg_catalog). Un golden ausente es error (fail-closed), nunca un falso OK.
// DOMAINSERV-105.
func compareToGolden(primaryDump, goldenPath string) (string, error) {
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		return "", fmt.Errorf("leer golden %s: %w", goldenPath, err)
	}
	return diffSchemas(normalize(primaryDump), normalize(string(data))), nil
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
			c.relkind::text AS relkind,
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

// applyMigrations aplica las migraciones compiladas en ESTE binario (in-tree)
// vía dmigrate.Up. DOMAINSERV-88: antes shelleaba a `domain migrate up`, lo que
// dependía de un binario externo en PATH (frágil en CI) y podía usar una fuente
// de migraciones distinta a la del repo → falso drift.
func applyMigrations(dsn string) error {
	return dmigrate.Up(dsn)
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
