// HU-25.13 unit tests del linter de convenciones SQL.

package dbconvlint

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func issueRules(issues []Issue) []string {
	out := make([]string, len(issues))
	for i, is := range issues {
		out[i] = is.Rule
	}
	return out
}

const validHeader = `-- migration: ok
-- author: me@x.com
-- issue: HU-XX.Y
-- description: test
-- breaking: false
-- estimated_duration: <1s
`

// Escenario 3: JSON sin B prohibido.
func TestLint_PreferJSONB(t *testing.T) {
	src := validHeader + `CREATE TABLE foos (
  id UUID PRIMARY KEY,
  data JSON,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	issues := Lint("000099_foo.up.sql", src)
	require.Contains(t, issueRules(issues), "prefer-jsonb")
}

func TestLint_AllowsJSONB(t *testing.T) {
	src := validHeader + `CREATE TABLE foos (
  id UUID PRIMARY KEY,
  data JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	require.NotContains(t, issueRules(Lint("000099_foo.up.sql", src)), "prefer-jsonb")
}

func TestLint_PreferTimestamptz(t *testing.T) {
	src := validHeader + `CREATE TABLE foos (
  id UUID PRIMARY KEY,
  ts TIMESTAMP,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	require.Contains(t, issueRules(Lint("000099_foo.up.sql", src)), "prefer-timestamptz")
}

func TestLint_AllowsTimestamptz(t *testing.T) {
	src := validHeader + `CREATE TABLE foos (
  id UUID PRIMARY KEY,
  ts TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	require.NotContains(t, issueRules(Lint("000099_foo.up.sql", src)), "prefer-timestamptz")
}

// Escenario 2: required created_at.
func TestLint_RequiresCreatedAt(t *testing.T) {
	src := validHeader + `CREATE TABLE foos (
  id UUID PRIMARY KEY,
  name TEXT
);`
	rules := issueRules(Lint("000099_foo.up.sql", src))
	require.Contains(t, rules, "require-created-at")
}

// Escenario 4: FK sin sufijo _id.
func TestLint_FKSuffix(t *testing.T) {
	src := validHeader + `CREATE TABLE memberships (
  id UUID PRIMARY KEY,
  org UUID REFERENCES organizations(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	rules := issueRules(Lint("000099_m.up.sql", src))
	require.Contains(t, rules, "fk-naming-suffix")
}

func TestLint_FKWithIdSuffix_OK(t *testing.T) {
	src := validHeader + `CREATE TABLE memberships (
  id UUID PRIMARY KEY,
  organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	require.NotContains(t, issueRules(Lint("000099_m.up.sql", src)), "fk-naming-suffix")
}

// Escenario 1: singular table name.
func TestLint_PluralTableName(t *testing.T) {
	src := validHeader + `CREATE TABLE user (
  id UUID PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	require.Contains(t, issueRules(Lint("000099_u.up.sql", src)), "naming-plural-table")
}

// Escenario 5: header missing.
func TestLint_HeaderMissing(t *testing.T) {
	src := `CREATE TABLE foos (id UUID PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());`
	rules := issueRules(Lint("000099_foo.up.sql", src))
	require.Contains(t, rules, "header-required")
}

// Down migrations no exigen header.
func TestLint_DownMigrationNoHeader(t *testing.T) {
	src := `DROP TABLE foos;`
	rules := issueRules(Lint("000099_foo.down.sql", src))
	require.NotContains(t, rules, "header-required")
}

// FK sin ON DELETE.
func TestLint_FKMissingOnDelete(t *testing.T) {
	src := validHeader + `CREATE TABLE memberships (
  id UUID PRIMARY KEY,
  organization_id UUID REFERENCES organizations(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	require.Contains(t, issueRules(Lint("000099_m.up.sql", src)), "fk-on-delete-strategy")
}

// Money con float.
func TestLint_MoneyColumnFloat(t *testing.T) {
	src := validHeader + `CREATE TABLE cost_logs (
  id UUID PRIMARY KEY,
  total_usd FLOAT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	require.Contains(t, issueRules(Lint("000099_c.up.sql", src)), "prefer-numeric-money")
}

// Escenario 6: override via comment.
func TestLint_OverrideNextLine(t *testing.T) {
	src := validHeader + `CREATE TABLE foos (
  id UUID PRIMARY KEY,
  -- domain-lint-ignore-next: prefer-jsonb
  data JSON,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	require.NotContains(t, issueRules(Lint("000099_foo.up.sql", src)), "prefer-jsonb")
}

func TestLint_OverrideWildcard(t *testing.T) {
	src := validHeader + `CREATE TABLE foos (
  id UUID PRIMARY KEY,
  -- domain-lint-ignore-next: *
  data JSON,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	require.NotContains(t, issueRules(Lint("000099_foo.up.sql", src)), "prefer-jsonb")
}

// Sabotaje: si comentamos la override, vuelve a fallar.
func TestSabotage_OverrideRemoved_RuleFiresAgain(t *testing.T) {
	src := validHeader + `CREATE TABLE foos (
  id UUID PRIMARY KEY,
  data JSON,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	require.Contains(t, issueRules(Lint("000099_foo.up.sql", src)), "prefer-jsonb",
		"sin override, prefer-jsonb DEBE saltar")
}

// --- Fix mode tests ---

func TestFix_JSONtoJSONB(t *testing.T) {
	src := `CREATE TABLE foos (data JSON);`
	got, changed := Fix(src)
	require.True(t, changed)
	require.Equal(t, `CREATE TABLE foos (data JSONB);`, got)
}

func TestFix_JSONBUnchanged(t *testing.T) {
	src := `CREATE TABLE foos (data JSONB);`
	got, changed := Fix(src)
	require.False(t, changed)
	require.Equal(t, src, got)
}

func TestFix_TIMESTAMPtoTIMESTAMPTZ(t *testing.T) {
	src := `CREATE TABLE foos (ts TIMESTAMP);`
	got, changed := Fix(src)
	require.True(t, changed)
	require.Equal(t, `CREATE TABLE foos (ts TIMESTAMPTZ);`, got)
}

func TestFix_TIMESTAMPWithTZUnchanged(t *testing.T) {
	src := `CREATE TABLE foos (ts TIMESTAMP WITH TIME ZONE);`
	got, changed := Fix(src)
	require.False(t, changed)
	require.Equal(t, src, got)
}

func TestFix_MultipleReplacements(t *testing.T) {
	src := `CREATE TABLE foos (
  data JSON,
  ts TIMESTAMP,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	got, changed := Fix(src)
	require.True(t, changed)
	require.Contains(t, got, "JSONB")
	require.Contains(t, got, "TIMESTAMPTZ")
	require.NotContains(t, got, "\n  data JSON,")
	require.NotContains(t, got, "ts TIMESTAMP,")
}

func TestFix_OverrideRespected(t *testing.T) {
	src := `-- domain-lint-ignore-next: prefer-jsonb
data JSON,`
	got, changed := Fix(src)
	require.False(t, changed)
	require.Equal(t, src, got)
}

func TestFix_CommentIgnored(t *testing.T) {
	src := `-- usa JSON en este comment
CREATE TABLE foos (data JSONB);`
	got, changed := Fix(src)
	require.False(t, changed)
	require.Equal(t, src, got)
}

// Sabotaje: JSON dentro de comment NO debe disparar.
func TestSabotage_JSONInComment_Ignored(t *testing.T) {
	src := validHeader + `-- usar JSON aquí está OK porque es comment
CREATE TABLE foos (
  id UUID PRIMARY KEY,
  data JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	require.NotContains(t, issueRules(Lint("000099_foo.up.sql", src)), "prefer-jsonb")
}

// HU-25.3 Migration Safety rules ----------------------------------------

func TestSafety_CreateIndexConcurrent(t *testing.T) {
	src := validHeader + `CREATE INDEX foos_name_idx ON foos(name);`
	require.Contains(t, issueRules(Lint("000099_x.up.sql", src)), "require-concurrent-index")
}

func TestSafety_CreateIndexConcurrent_OK(t *testing.T) {
	src := validHeader + `CREATE INDEX CONCURRENTLY foos_name_idx ON foos(name);`
	require.NotContains(t, issueRules(Lint("000099_x.up.sql", src)), "require-concurrent-index")
}

func TestSafety_AddColumnNotNullSinDefault(t *testing.T) {
	src := validHeader + `ALTER TABLE users ADD COLUMN status TEXT NOT NULL;`
	require.Contains(t, issueRules(Lint("000099_x.up.sql", src)), "require-default-for-not-null")
}

func TestSafety_AddColumnNotNullConDefault_OK(t *testing.T) {
	src := validHeader + `ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'active';`
	require.NotContains(t, issueRules(Lint("000099_x.up.sql", src)), "require-default-for-not-null")
}

func TestSafety_DropTableSinIfExists(t *testing.T) {
	src := validHeader + `DROP TABLE old_data;`
	require.Contains(t, issueRules(Lint("000099_x.up.sql", src)), "require-if-exists-drop")
}

func TestSafety_DropTableConIfExists_OK(t *testing.T) {
	src := validHeader + `DROP TABLE IF EXISTS old_data;`
	require.NotContains(t, issueRules(Lint("000099_x.up.sql", src)), "require-if-exists-drop")
}

func TestSafety_VacuumFull(t *testing.T) {
	src := validHeader + `VACUUM FULL bloated_table;`
	require.Contains(t, issueRules(Lint("000099_x.up.sql", src)), "no-vacuum-full")
}

func TestSafety_LockTable(t *testing.T) {
	src := validHeader + `LOCK TABLE users IN EXCLUSIVE MODE;`
	require.Contains(t, issueRules(Lint("000099_x.up.sql", src)), "no-explicit-lock-table")
}

func TestSafety_AddFKSinNotValid(t *testing.T) {
	src := validHeader + `ALTER TABLE memberships ADD CONSTRAINT m_org_fk FOREIGN KEY (org_id) REFERENCES organizations(id);`
	require.Contains(t, issueRules(Lint("000099_x.up.sql", src)), "require-not-valid-fk")
}

func TestSafety_AddFKConNotValid_OK(t *testing.T) {
	src := validHeader + `ALTER TABLE memberships ADD CONSTRAINT m_org_fk FOREIGN KEY (org_id) REFERENCES organizations(id) NOT VALID;`
	require.NotContains(t, issueRules(Lint("000099_x.up.sql", src)), "require-not-valid-fk")
}

func TestSafety_Override(t *testing.T) {
	src := validHeader + `-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX foos_name_idx ON foos(name);`
	require.NotContains(t, issueRules(Lint("000099_x.up.sql", src)), "require-concurrent-index")
}

// Sabotaje: si pretendemos pasar regla con comment falso, no bypassea.
func TestSafetySabotage_NonOverrideCommentDoesNotBypass(t *testing.T) {
	src := validHeader + `-- TODO: agregar CONCURRENTLY
CREATE INDEX foos_name_idx ON foos(name);`
	require.Contains(t, issueRules(Lint("000099_x.up.sql", src)), "require-concurrent-index")
}

// Issue.String() es útil para output CI.
func TestIssue_String(t *testing.T) {
	is := Issue{File: "000099_x.up.sql", Line: 42, Rule: "prefer-jsonb", Message: "use JSONB"}
	s := is.String()
	require.True(t, strings.Contains(s, "000099_x.up.sql:42"))
	require.True(t, strings.Contains(s, "[prefer-jsonb]"))
}
