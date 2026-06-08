package requirement

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlugValidation(t *testing.T) {
	valid := []string{
		"REQ-01",
		"REQ-01-core-platform",
		"REQ-99-auth-security",
		"REQ-100",
	}
	invalid := []string{
		"",
		"REQ-",
		"req-01",
		"REQ_01",
		"REQ-01.1", // dot no permitido
		"HU-01.1",
		"random-slug",
	}

	for _, s := range valid {
		require.True(t, reReqSlug.MatchString(s), "slug %q debe ser válido", s)
	}
	for _, s := range invalid {
		require.False(t, reReqSlug.MatchString(s), "slug %q debe ser inválido", s)
	}
}

func TestValidStatuses(t *testing.T) {
	require.True(t, validStatuses[StatusActive])
	require.True(t, validStatuses[StatusArchived])
	require.False(t, validStatuses["deleted"])
	require.False(t, validStatuses[""])
}

func TestValidPriorities(t *testing.T) {
	require.True(t, validPriorities[PriorityLow])
	require.True(t, validPriorities[PriorityMedium])
	require.True(t, validPriorities[PriorityHigh])
	require.True(t, validPriorities[PriorityCritical])
	require.False(t, validPriorities["urgent"])
	require.False(t, validPriorities[""])
}

func TestCreateValidation(t *testing.T) {
	s := &Service{}

	_, err := s.Create(nil, "", "title", "", "", "", "")
	require.ErrorIs(t, err, ErrSlugInvalid)

	_, err = s.Create(nil, "REQ-01", "", "", "", "", "")
	require.Error(t, err)

	_, err = s.Create(nil, "REQ-01", "title", "", "invalid", "", "")
	require.ErrorIs(t, err, ErrInvalidStatus)

	_, err = s.Create(nil, "REQ-01", "title", "", "", "urgent", "")
	require.ErrorIs(t, err, ErrInvalidPriority)
}

// Sabotaje: unique violation debe detectarse por código postgres 23505
func TestSabotage_UniqueViolationCheck(t *testing.T) {
	require.False(t, isUniqueViolation(nil))
	require.False(t, isUniqueViolation(ErrNotFound))
	// PostgreSQL unique violation error message
	require.True(t, isUniqueViolation(fmt.Errorf("ERROR: duplicate key value violates unique constraint \"requirements_slug_idx\" (SQLSTATE 23505)")))
}
