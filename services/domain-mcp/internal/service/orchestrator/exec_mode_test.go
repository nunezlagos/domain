package orchestrator

import "testing"

func TestRequiresPhaseGate(t *testing.T) {
	cases := []struct {
		mode, phase string
		want        bool
	}{
		{"auto", "sdd-spec", false},
		{"auto", "sdd-apply", false},
		{"", "sdd-design", false}, // vacío == auto
		{"manual", "sdd-explore", true},
		{"manual", "sdd-apply", true},
		{"hybrid", "sdd-spec", true},
		{"hybrid", "sdd-design", true},
		{"hybrid", "sdd-apply", true},
		{"hybrid", "sdd-judge", true},
		{"hybrid", "sdd-explore", false}, // no es fase clave
		{"hybrid", "sdd-research", false},
	}
	for _, c := range cases {
		if got := requiresPhaseGate(c.mode, c.phase); got != c.want {
			t.Errorf("requiresPhaseGate(%q,%q)=%v want %v", c.mode, c.phase, got, c.want)
		}
	}
}
