// Package dbconvlint — issue-25.13 linter de convenciones SQL para migrations.
//
// Reglas core verificadas (todas configurables vía // domain-lint-ignore-next):
//
//	prefer-jsonb           — JSON sin B prohibido (usar JSONB)
//	prefer-timestamptz     — TIMESTAMP sin tz prohibido
//	require-created-at     — CREATE TABLE debe declarar created_at TIMESTAMPTZ
//	require-updated-at     — si UPDATE-able, declarar updated_at + trigger
//	fk-naming-suffix       — columnas FK terminan en _id
//	prefer-jsonb-money     — float/real prohibido para columnas *_usd/*_amount/price*
//	header-required        — header completo (issue, author, description, breaking)
//	naming-snake-case      — tablas y columnas snake_case
//	naming-plural-table    — nombre de tabla en plural
//	require-table-prefix   — CREATE TABLE debe empezar con prefijo de dominio (taxonomía)
//	fk-on-delete-strategy  — REFERENCES debe declarar ON DELETE
package dbconvlint

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Issue es un hallazgo del linter.
type Issue struct {
	File    string
	Line    int
	Rule    string
	Message string
}

func (i Issue) String() string {
	return fmt.Sprintf("%s:%d: [%s] %s", i.File, i.Line, i.Rule, i.Message)
}

// Lint analiza un archivo SQL y devuelve issues.
// file es el path (para reportes); src es el contenido.
func Lint(file, src string) []Issue {
	lines := strings.Split(src, "\n")
	overrides := parseOverrides(lines)

	var issues []Issue
	add := func(line int, rule, msg string) {
		if overrides[overrideKey{line, rule}] || overrides[overrideKey{line, "*"}] {
			return
		}
		issues = append(issues, Issue{File: file, Line: line, Rule: rule, Message: msg})
	}

	checkHeader(file, lines, add)
	checkProhibitedTypes(lines, add)
	checkFKNaming(lines, add)
	checkCreateTableConventions(src, lines, add)
	checkMigrationSafety(src, lines, add)

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Line != issues[j].Line {
			return issues[i].Line < issues[j].Line
		}
		return issues[i].Rule < issues[j].Rule
	})
	return issues
}

type overrideKey struct {
	line int
	rule string
}

// parseOverrides busca comments `-- domain-lint-ignore-next: rule[,rule]` y mapea
// que la siguiente línea efectiva (no-blank, no-comment) ignore esas reglas.
// `*` ignora todas.
var overrideRe = regexp.MustCompile(`(?i)--\s*domain-lint-ignore-next:\s*([a-z0-9*,_\- ]+)`)

func parseOverrides(lines []string) map[overrideKey]bool {
	out := map[overrideKey]bool{}
	for i, l := range lines {
		m := overrideRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		rules := strings.Split(m[1], ",")
		next := nextEffectiveLine(lines, i+1)
		for _, r := range rules {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			out[overrideKey{next + 1, r}] = true // line index 1-based
		}
	}
	return out
}

func nextEffectiveLine(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		return i
	}
	return from
}

// === Header ===

var headerFields = []string{"migration:", "author:", "issue:", "description:", "breaking:", "estimated_duration:"}

func checkHeader(file string, lines []string, add func(int, string, string)) {
	if !strings.HasSuffix(file, ".up.sql") {
		return // header solo en up migrations
	}
	headerBlock := strings.Join(lines[:min(20, len(lines))], "\n")
	for _, f := range headerFields {
		if !strings.Contains(strings.ToLower(headerBlock), f) {
			add(1, "header-required",
				fmt.Sprintf("missing required header field '%s' in first 20 lines", strings.TrimSuffix(f, ":")))
		}
	}
}

// === Tipos prohibidos ===

var (
	// JSON (no JSONB): \b en cada lado evita matchear JSONB (B es word char,
	// no hay boundary entre N y B).
	reJSONNoB = regexp.MustCompile(`(?i)\bJSON\b`)
	// TIMESTAMP sin tz: \b descarta TIMESTAMPTZ.
	reTimestampPlain = regexp.MustCompile(`(?i)\bTIMESTAMP\b`)
	// Cuando aparece TIMESTAMP debemos verificar que no sea "TIMESTAMP WITH TIME ZONE".
	reTimestampWithTZ = regexp.MustCompile(`(?i)\bTIMESTAMP\s+WITH\s+TIME\s+ZONE\b`)
	// FLOAT / REAL / DOUBLE PRECISION sospechosos cuando la columna parece money.
	reMoneyCol = regexp.MustCompile(`(?i)\b([a-z_]*(_usd|_amount|price[a-z_]*))\b\s+(FLOAT|REAL|DOUBLE PRECISION)\b`)
)

func checkProhibitedTypes(lines []string, add func(int, string, string)) {
	for i, l := range lines {
		if isCommentLine(l) {
			continue
		}
		stripped := stripInlineComment(l)
		if reJSONNoB.MatchString(stripped) {
			add(i+1, "prefer-jsonb", "use JSONB instead of JSON")
		}
		if reTimestampPlain.MatchString(stripped) && !reTimestampWithTZ.MatchString(stripped) {
			add(i+1, "prefer-timestamptz", "use TIMESTAMPTZ instead of TIMESTAMP (without timezone)")
		}
		if m := reMoneyCol.FindStringSubmatch(stripped); m != nil {
			add(i+1, "prefer-numeric-money",
				fmt.Sprintf("column '%s' looks monetary; use NUMERIC(N,M) instead of %s", m[1], strings.ToUpper(m[3])))
		}
	}
}

// === FK naming ===

// REFERENCES table(col) sin _id terminal en la columna previa.
var reFKLine = regexp.MustCompile(`(?i)^\s*([a-z_][a-z0-9_]*)\s+UUID\b[^,]*REFERENCES\s+`)

func checkFKNaming(lines []string, add func(int, string, string)) {
	for i, l := range lines {
		if isCommentLine(l) {
			continue
		}
		m := reFKLine.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		col := m[1]
		if col == "id" {
			continue
		}
		// _id es el ideal; _by (created_by, triggered_by) se acepta por convención
		// de actor-tracking. Cualquier otro sufijo es violación.
		if !strings.HasSuffix(col, "_id") && !strings.HasSuffix(col, "_by") {
			add(i+1, "fk-naming-suffix",
				fmt.Sprintf("FK column '%s' should end with '_id' (or '_by' for actor FKs)", col))
		}
	}
}

// === CREATE TABLE conventions ===

var reCreateTableHeader = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)

// extractCreateTables encuentra los CREATE TABLE y devuelve (name, body, startIdx).
// Balance manual de paréntesis (defaults con gen_random_uuid() rompen regex naïve).
func extractCreateTables(src string) []struct {
	Name     string
	Body     string
	StartIdx int
} {
	var out []struct {
		Name     string
		Body     string
		StartIdx int
	}
	for _, m := range reCreateTableHeader.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		openParen := m[1] - 1 // posición del '('
		// Encontrar el ')' que cierra el balance
		depth := 1
		j := openParen + 1
		for j < len(src) && depth > 0 {
			switch src[j] {
			case '(':
				depth++
			case ')':
				depth--
			case '\'':
				// saltar string literal
				j++
				for j < len(src) && src[j] != '\'' {
					if src[j] == '\\' {
						j++
					}
					j++
				}
			}
			j++
		}
		body := src[openParen+1 : j-1]
		out = append(out, struct {
			Name     string
			Body     string
			StartIdx int
		}{name, body, m[2]})
	}
	return out
}

var commonNonPluralAllowed = map[string]bool{
	"schema_migrations":  true,
	"audit_log":          true, // log es plural-like (lat. plural)
	"audit_activity_log": true,
	"feature_flags":      true,
}

// Sufijos que ya implican colectivo (no requieren pluralización terminal en s).
var pluralEquivalentSuffixes = []string{
	"_log", "_history", "_data", "_metadata", "_status", "_config",
	"_settings", "_audit",
}

// Prefijos de funcionalidad válidos (taxonomía objetivo). Incluye el underscore
// final para forzar agrupación real (flow_, no que pase "flowers"). Mantener
// sincronizado con la taxonomía del proyecto.
var validTablePrefixes = []string{
	"agent_", "audit_", "auth_", "cron_", "external_", "file_",
	"flow_", "issue_", "knowledge_", "mcp_", "notification_",
	"platform_", "project_", "prompt_", "runner_", "sdd_", "seed_",
	"skill_", "tdd_", "usage_", "users_", "webhook_",
	"enrollment_", // enrollment_tokens (single-org; ver risks)
}

// Nombres canónicos RESUELTOS (allowlist del lint): nombre = grupo, excepción
// documentada a la regla "toda tabla lleva prefijo" (estilo Rails/Postgres).
// Decisión CANÓNICA RESUELTA de REQ-42 (no open_question). NO requieren prefijo.
var canonicalTableExceptions = map[string]bool{
	"users":             true, // grupo users_, nombre canónico (REQ-42.8)
	"roles":             true, // grupo users_, catálogo RBAC (REQ-42.8)
	"user_roles":        true, // grupo users_, tabla puente (REQ-42.8)
	"issues":            true, // grupo issue_, nombre canónico
	"schema_migrations": true, // tooling interno golang-migrate
}

func hasValidTablePrefix(name string) bool {
	for _, p := range validTablePrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func checkCreateTableConventions(src string, lines []string, add func(int, string, string)) {
	for _, t := range extractCreateTables(src) {
		name := t.Name
		body := t.Body
		line := lineOf(src, t.StartIdx)

		// snake_case
		if !isSnakeCase(name) {
			add(line, "naming-snake-case",
				fmt.Sprintf("table '%s' should be snake_case", name))
		}
		// plural heuristic
		if !commonNonPluralAllowed[name] && !looksPlural(name) {
			add(line, "naming-plural-table",
				fmt.Sprintf("table '%s' should be plural (e.g. '%ss')", name, name))
		}
		// require-table-prefix: la tabla debe agruparse por dominio
		if !canonicalTableExceptions[name] && !hasValidTablePrefix(name) {
			add(line, "require-table-prefix",
				fmt.Sprintf("table '%s' must start with a functional-domain prefix (%s) "+
					"or be a documented canonical name. "+
					"Override: -- domain-lint-ignore-next: require-table-prefix",
					name, strings.Join(validTablePrefixes, ", ")))
		}
		// required created_at (o equivalente: cualquier *_at TIMESTAMPTZ NOT NULL DEFAULT NOW
		// — audit_log usa 'occurred_at', agent_runs usa 'started_at', etc.).
		if !hasTimestamptzDefaultNow(body) {
			add(line, "require-created-at",
				fmt.Sprintf("table '%s' missing required column 'created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()' (or equivalent *_at)", name))
		}
		// FK ON DELETE strategy
		for j, bl := range strings.Split(body, "\n") {
			if matched, _ := regexp.MatchString(`(?i)REFERENCES\s+`, bl); !matched {
				continue
			}
			if matched, _ := regexp.MatchString(`(?i)ON\s+DELETE\s+(CASCADE|SET\s+NULL|RESTRICT|NO\s+ACTION|SET\s+DEFAULT)`, bl); !matched {
				add(line+j, "fk-on-delete-strategy",
					"FK should declare ON DELETE strategy explicitly (CASCADE | SET NULL | RESTRICT)")
			}
		}
	}
	_ = lines
}

// === Helpers ===

func isCommentLine(l string) bool {
	return strings.HasPrefix(strings.TrimSpace(l), "--")
}

func stripInlineComment(l string) string {
	if i := strings.Index(l, "--"); i >= 0 {
		return l[:i]
	}
	return l
}

var reTimestampDefaultNow = regexp.MustCompile(`(?i)[a-z_]+_at\s+TIMESTAMPTZ[^,]*DEFAULT\s+NOW\(\)`)

func hasTimestamptzDefaultNow(body string) bool {
	return reTimestampDefaultNow.MatchString(body)
}

var reSnakeCase = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func isSnakeCase(s string) bool {
	return reSnakeCase.MatchString(s) && !strings.Contains(s, "__")
}

func looksPlural(s string) bool {
	if strings.HasSuffix(s, "s") {
		return true
	}
	for _, suf := range pluralEquivalentSuffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

func lineOf(src string, idx int) int {
	return strings.Count(src[:idx], "\n") + 1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Fix aplica transformaciones automáticas para reglas auto-fixables.
// Retorna el SQL modificado y true si hubo cambios.
//
// Reglas auto-fixables:
//   - prefer-jsonb: JSON → JSONB
//   - prefer-timestamptz: TIMESTAMP → TIMESTAMPTZ
//   - naming-snake-case: pascal case → snake case en nombres de tabla
func Fix(src string) (string, bool) {
	fixed := false
	lines := strings.Split(src, "\n")
	overrides := parseOverrides(lines)

	// prefer-jsonb: reemplazar JSON no-comment donde no sea JSONB ya.
	for i, l := range lines {
		if isCommentLine(l) {
			continue
		}
		if overrides[overrideKey{i + 1, "prefer-jsonb"}] || overrides[overrideKey{i + 1, "*"}] {
			continue
		}
		stripped := stripInlineComment(l)
		if !reJSONNoB.MatchString(stripped) {
			continue
		}
		// Reemplazar JSON solitario (word boundary asegura no tocar JSONB)
		newL := reJSONNoB.ReplaceAllString(l, "JSONB")
		if newL != l {
			lines[i] = newL
			fixed = true
		}
	}

	// prefer-timestamptz: reemplazar TIMESTAMP solo donde no sea "WITH TIME ZONE".
	for i, l := range lines {
		if isCommentLine(l) {
			continue
		}
		if overrides[overrideKey{i + 1, "prefer-timestamptz"}] || overrides[overrideKey{i + 1, "*"}] {
			continue
		}
		stripped := stripInlineComment(l)
		if !reTimestampPlain.MatchString(stripped) {
			continue
		}
		if reTimestampWithTZ.MatchString(stripped) {
			continue
		}
		// Reemplazar TIMESTAMP -> TIMESTAMPTZ uno por uno evitando WITH TIME ZONE.
		newL := reTimestampPlain.ReplaceAllString(l, "TIMESTAMPTZ")
		if newL != l {
			lines[i] = newL
			fixed = true
		}
	}

	return strings.Join(lines, "\n"), fixed
}

// === Migration Safety (issue-25.3) ===
//
// Reglas que detectan patrones peligrosos para producción:
//   * CREATE INDEX sin CONCURRENTLY → bloquea writes en tablas grandes
//   * ALTER TABLE ADD COLUMN NOT NULL sin DEFAULT → rewrite full table
//   * DROP TABLE/COLUMN sin IF EXISTS → fail si already removed (bloquea deploy)
//   * VACUUM FULL → exclusive lock, downtime
//   * LOCK TABLE explícito → uso disciplinado solamente, requiere override
//   * ALTER TABLE ADD FOREIGN KEY sin NOT VALID → table-wide lock durante validación

var (
	reCreateIndex = regexp.MustCompile(`(?i)^\s*CREATE\s+(UNIQUE\s+)?INDEX\s`)
	// reCreateIndexStmt captura el statement completo CREATE INDEX ... ON table
	// (multilínea, hasta ;) — el grupo capturado es el nombre de la tabla.
	reCreateIndexStmt  = regexp.MustCompile(`(?is)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+NOT\s+EXISTS\s+)?[a-zA-Z_][a-zA-Z0-9_]*\s+ON\s+([a-zA-Z_][a-zA-Z0-9_]*)[^;]*;`)
	reConcurrently     = regexp.MustCompile(`(?i)\bCONCURRENTLY\b`)
	reAddColumnNotNull = regexp.MustCompile(`(?i)\bADD\s+COLUMN\s+(?:IF\s+NOT\s+EXISTS\s+)?[a-z_]+\s+[a-zA-Z_(),0-9]+(?:\s+[a-zA-Z_(),0-9 ]+?)?\s+NOT\s+NULL\b`)
	reDefaultClause    = regexp.MustCompile(`(?i)\bDEFAULT\b`)
	reDropTable        = regexp.MustCompile(`(?i)^\s*DROP\s+TABLE\s`)
	reDropColumn       = regexp.MustCompile(`(?i)\bDROP\s+COLUMN\s`)
	reIfExists         = regexp.MustCompile(`(?i)\bIF\s+EXISTS\b`)
	reVacuumFull       = regexp.MustCompile(`(?i)^\s*VACUUM\s+FULL\b`)
	reLockTable        = regexp.MustCompile(`(?i)^\s*LOCK\s+(TABLE\s+)?`)
	reAddFK            = regexp.MustCompile(`(?i)\bADD\s+(CONSTRAINT\s+\S+\s+)?FOREIGN\s+KEY\b`)
	reNotValid         = regexp.MustCompile(`(?i)\bNOT\s+VALID\b`)
)

func checkMigrationSafety(src string, lines []string, add func(int, string, string)) {
	// Tablas creadas en este mismo archivo: sus indices iniciales NO requieren
	// CONCURRENTLY (tabla está vacía durante la creación).
	tablesInFile := map[string]bool{}
	for _, t := range extractCreateTables(src) {
		tablesInFile[strings.ToLower(t.Name)] = true
	}

	// Multiline scan: CREATE INDEX puede estar split en varias líneas.
	// Buscamos statements completos sobre src y mapeamos a línea inicial.
	indexStmts := reCreateIndexStmt.FindAllStringSubmatchIndex(src, -1)
	for _, m := range indexStmts {
		stmt := src[m[0]:m[1]]
		tableName := strings.ToLower(src[m[2]:m[3]])
		if reConcurrently.MatchString(stmt) {
			continue
		}
		if tablesInFile[tableName] {
			continue
		}
		line := lineOf(src, m[0])
		add(line, "require-concurrent-index",
			"CREATE INDEX on existing table must use CONCURRENTLY (override allowed via -- domain-lint-ignore-next: require-concurrent-index)")
	}

	for i, l := range lines {
		if isCommentLine(l) {
			continue
		}
		stripped := stripInlineComment(l)
		// ADD COLUMN NOT NULL sin DEFAULT
		if reAddColumnNotNull.MatchString(stripped) && !reDefaultClause.MatchString(stripped) {
			add(i+1, "require-default-for-not-null",
				"ADD COLUMN ... NOT NULL must have DEFAULT (else rewrites whole table; use backfill + ALTER COLUMN SET NOT NULL pattern instead)")
		}
		// DROP TABLE sin IF EXISTS
		if reDropTable.MatchString(stripped) && !reIfExists.MatchString(stripped) {
			add(i+1, "require-if-exists-drop",
				"DROP TABLE should use IF EXISTS for idempotent down migrations")
		}
		// DROP COLUMN sin IF EXISTS
		if reDropColumn.MatchString(stripped) && !reIfExists.MatchString(stripped) {
			add(i+1, "require-if-exists-drop",
				"DROP COLUMN should use IF EXISTS")
		}
		// VACUUM FULL — error
		if reVacuumFull.MatchString(stripped) {
			add(i+1, "no-vacuum-full",
				"VACUUM FULL takes exclusive lock; use pg_repack or routine VACUUM instead")
		}
		// LOCK TABLE explícito
		if reLockTable.MatchString(stripped) {
			add(i+1, "no-explicit-lock-table",
				"explicit LOCK TABLE is rarely safe in migrations; statement_timeout (issue-25.8) protects against runaways")
		}
		// ADD FOREIGN KEY sin NOT VALID
		if reAddFK.MatchString(stripped) && !reNotValid.MatchString(stripped) {
			add(i+1, "require-not-valid-fk",
				"ALTER TABLE ADD FOREIGN KEY should use NOT VALID + separate VALIDATE CONSTRAINT to avoid full table scan lock")
		}
	}
	_ = src
}
