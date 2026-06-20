package dedup

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestHash_Deterministic(t *testing.T) {
	pid := uuid.New()
	in := FingerprintInput{ProjectID: pid, ObservationType: "note", Title: "T", Content: "C"}
	require.Equal(t, Hash(in), Hash(in))
}

func TestHash_IgnoresWhitespaceAndCase(t *testing.T) {
	pid := uuid.New()
	a := Hash(FingerprintInput{ProjectID: pid, ObservationType: "note", Title: "Fix Login", Content: "Bug fixed"})
	b := Hash(FingerprintInput{ProjectID: pid, ObservationType: "note", Title: "fix  login", Content: "bug   fixed"})
	require.Equal(t, a, b)
}

func TestHash_DifferentContentDifferentHash(t *testing.T) {
	pid := uuid.New()
	a := Hash(FingerprintInput{ProjectID: pid, ObservationType: "note", Title: "A", Content: "x"})
	b := Hash(FingerprintInput{ProjectID: pid, ObservationType: "note", Title: "A", Content: "y"})
	require.NotEqual(t, a, b)
}

func TestHash_DifferentProjectDifferentHash(t *testing.T) {
	a := Hash(FingerprintInput{ProjectID: uuid.New(), ObservationType: "note", Title: "x", Content: "x"})
	b := Hash(FingerprintInput{ProjectID: uuid.New(), ObservationType: "note", Title: "x", Content: "x"})
	require.NotEqual(t, a, b, "mismo contenido en distintos projects → distinto hash (no leak cross-project)")
}

func TestHash_DifferentTypeDifferentHash(t *testing.T) {
	pid := uuid.New()
	a := Hash(FingerprintInput{ProjectID: pid, ObservationType: "note", Title: "x", Content: "x"})
	b := Hash(FingerprintInput{ProjectID: pid, ObservationType: "decision", Title: "x", Content: "x"})
	require.NotEqual(t, a, b)
}

func TestHash_FixedLength(t *testing.T) {
	h := Hash(FingerprintInput{ProjectID: uuid.New(), ObservationType: "x", Title: "y", Content: "z"})
	require.Equal(t, 32, len(h), "SHA-256 = 32 bytes")
}
