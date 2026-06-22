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
