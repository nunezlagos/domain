package attachment

import "testing"

// TestEntityTable: mapeo entity_type→tabla, incluido ticket (DOMAINSERV-79 H2).
// Función pura, sin DB.
func TestEntityTable(t *testing.T) {
	cases := map[string]struct {
		table string
		ok    bool
	}{
		"user_story":     {"issues", true},
		"requirement":    {"sdd_requirements", true},
		"hu_draft":       {"issue_drafts", true},
		"intake_payload": {"issue_intake_payloads", true},
		"ticket":         {"tickets", true},
		"bogus":          {"", false},
		"":               {"", false},
	}
	for et, want := range cases {
		gotTable, gotOK := entityTable(et)
		if gotTable != want.table || gotOK != want.ok {
			t.Errorf("entityTable(%q) = (%q, %v), want (%q, %v)", et, gotTable, gotOK, want.table, want.ok)
		}
	}
}
