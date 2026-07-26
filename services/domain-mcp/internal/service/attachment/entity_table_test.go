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
		// DOMAINSERV-144: decía "tickets", y esa tabla NO existe — la real es
		// project_tickets (000112). requireEntity interpola este nombre en un
		// fmt.Sprintf, así que todo init_upload sobre un ticket moría con
		// 42P01. Este test asserteaba el valor equivocado, o sea FIJABA el bug:
		// por eso file_attachments tenía 0 filas y parecía falta de uso.
		"intake_payload": {"issue_intake_payloads", true},
		"ticket":         {"project_tickets", true},
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
