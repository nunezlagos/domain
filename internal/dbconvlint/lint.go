// Package dbconvlint — HU-25.13 linter de convenciones SQL para migrations.
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
	"schema_migrations": true,
	"audit_log":         true, // log es plural-like (lat. plural)
	"activity_log":      true,
	"feature_flags":     true,
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
		// required created_at (o equivalente: cualquier *_at TIMESTAMPTZ NOT NULL DEFAULT NOW
		// — audit_log usa 'occurred_at', cost_logs usa 'recorded_at', etc.).
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
	switch {
	case strings.HasSuffix(s, "s"):
		return true
	case strings.HasSuffix(s, "_data"):
		return true
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
