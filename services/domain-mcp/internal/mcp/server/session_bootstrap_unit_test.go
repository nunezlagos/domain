package mcpserver

import "testing"

func TestSanitizeSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"acme-web", "acme-web"},
		{"Acme Web", "acme-web"},
		{"Mi Proyecto!", "mi-proyecto"},
		{"  Foo__Bar  ", "foo-bar"},
		{"123proj", "p-123proj"},
		{"", "proyecto"},
		{"...", "proyecto"},
		{"a", "a"},
	}
	for _, c := range cases {
		if got := sanitizeSlug(c.in); got != c.want {
			t.Errorf("sanitizeSlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCodeGraphIsStale(t *testing.T) {
	cases := []struct {
		name                 string
		indexedHead, curHead string
		want                 bool
	}{
		{"distinto -> stale", "abc123", "def456", true},
		{"igual -> fresco", "abc123", "abc123", false},
		{"sin indexed_head -> no afirmar stale", "", "def456", false},
		{"sin current_head -> no afirmar stale", "abc123", "", false},
		{"ambos vacios -> no stale", "", "", false},
	}
	for _, c := range cases {
		if got := codeGraphIsStale(c.indexedHead, c.curHead); got != c.want {
			t.Errorf("%s: codeGraphIsStale(%q,%q) = %v, want %v",
				c.name, c.indexedHead, c.curHead, got, c.want)
		}
	}
}
