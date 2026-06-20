package systemcron

import "testing"

func TestSagaEventFor(t *testing.T) {
	cases := []struct {
		policy string
		want   string
	}{
		{"require-cleanup", "cleanup_required"},
		{"re-emit", "reemit_eligible"},
		{"idempotent", "auto_retry_eligible"},
		{"", "auto_retry_eligible"},
		{"unknown_policy", "auto_retry_eligible"},
	}
	for _, c := range cases {
		t.Run(c.policy, func(t *testing.T) {
			got := sagaEventFor(c.policy)
			if got != c.want {
				t.Errorf("sagaEventFor(%q) = %q, want %q", c.policy, got, c.want)
			}
		})
	}
}
