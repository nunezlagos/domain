package observability

import (
	"bytes"
	"testing"
)

func TestKnownErrorSeeds_AllHaveFingerprintAndName(t *testing.T) {
	seeds := KnownErrorSeeds()
	if len(seeds) == 0 {
		t.Fatal("expected at least one seed")
	}
	for i, ke := range seeds {
		if len(ke.Fingerprint) == 0 {
			t.Fatalf("seed %d missing fingerprint", i)
		}
		if ke.Name == "" {
			t.Fatalf("seed %d missing name", i)
		}
	}
}

func TestKnownErrorSeeds_ActionConsistentWithRecoverable(t *testing.T) {
	for _, ke := range KnownErrorSeeds() {
		if !ke.Recoverable && ke.AutoHealAction != HealNone {
			t.Fatalf("%s: non-recoverable must use action=none, got %q", ke.Name, ke.AutoHealAction)
		}
	}
}

func TestKnownErrorSeeds_FingerprintsUnique(t *testing.T) {
	seen := [][]byte{}
	for _, ke := range KnownErrorSeeds() {
		for _, p := range seen {
			if bytes.Equal(p, ke.Fingerprint) {
				t.Fatalf("duplicate fingerprint for %s", ke.Name)
			}
		}
		seen = append(seen, ke.Fingerprint)
	}
}
