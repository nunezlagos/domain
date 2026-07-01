package agentprotocol

import "testing"

// El protocolo debe declarar el sync SDD OBLIGATORIO (REQ-55): sin esta directiva
// el auto-sync de openspec deja de ser determinístico. Guarda contra regresiones.
func TestFull_ContainsMandatorySDDSyncDirective(t *testing.T) {
	must := []string{
		"SYNC AUTOMÁTICO DEL SDD",
		"domain_openspec_export",
		"domain_issue_create_commit",
		"domain_orchestrate_phase_result",
		"domain_verify_complete",
		"domain_issue_set_status(archived)",
		"changes/archive/",
	}
	for _, s := range must {
		if !contains(Full, s) {
			t.Errorf("agent-protocol Full no contiene la directiva de sync SDD: falta %q", s)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
