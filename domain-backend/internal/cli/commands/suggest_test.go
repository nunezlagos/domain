package commands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"projcts", "projects", 1},
		{"flws", "flows", 1},
		{"kitten", "sitting", 3},
		{"", "abc", 3},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, levenshtein(tc.a, tc.b), "%s vs %s", tc.a, tc.b)
	}
}

func TestSuggest_CommandTypos(t *testing.T) {
	require.Equal(t, "projects", suggest("projcts", knownCommands))
	require.Equal(t, "flows", suggest("flws", knownCommands))
	require.Equal(t, "observations", suggest("observatons", knownCommands))
	require.Equal(t, "config", suggest("confgi", knownCommands))
	require.Empty(t, suggest("xyzzy", knownCommands), "nada cercano → sin sugerencia")
}

func TestSuggest_FlagTypos(t *testing.T) {
	shells := []string{"bash", "zsh", "fish", "powershell"}
	require.Equal(t, "bash", suggest("bsh", shells))
	require.Equal(t, "powershell", suggest("powershel", shells))
}

func TestKeyPrefix_NeverFullKey(t *testing.T) {
	full := "dk_live_abcdefghijklmnopqrstuvwxyz"
	p := keyPrefix(full)
	require.NotEqual(t, full, p)
	require.Contains(t, p, "...")
	require.LessOrEqual(t, len(p), 19, "máximo 16 chars + ellipsis")
	require.Equal(t, "(no configurado)", keyPrefix(""))
}

func TestManPage_TroffStructure(t *testing.T) {
	require.Contains(t, manPage, ".TH DOMAIN 1")
	require.Contains(t, manPage, ".SH NAME")
	require.Contains(t, manPage, ".SH COMMANDS")
	require.Contains(t, manPage, ".SH EXAMPLES")
}

func TestPowershellCompletion_HasCommands(t *testing.T) {
	require.Contains(t, powershellCompletion, "Register-ArgumentCompleter")
	require.Contains(t, powershellCompletion, "'projects'")
	require.Contains(t, powershellCompletion, "'config'")
}

func TestParseGlobalFlags_Verbose(t *testing.T) {
	gf, rest := parseGlobalFlags([]string{"--verbose", "ls"})
	require.True(t, gf.Verbose)
	require.Equal(t, []string{"ls"}, rest)

	gf, _ = parseGlobalFlags([]string{"-v"})
	require.True(t, gf.Verbose)
}
