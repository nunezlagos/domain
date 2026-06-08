package userstory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHUSlugValidation(t *testing.T) {
	valid := []string{
		"HU-01.1",
		"HU-01.1-db-schema",
		"HU-99.99-auth",
		"HU-04.2-user-stories-gherkin",
	}
	invalid := []string{
		"",
		"HU-1",
		"HU-01",
		"hu-01.1",
		"HU_01.1",
		"REQ-01",
		"HU-01.1.",
	}

	for _, s := range valid {
		require.True(t, reHUSlug.MatchString(s), "slug %q debe ser válido", s)
	}
	for _, s := range invalid {
		require.False(t, reHUSlug.MatchString(s), "slug %q debe ser inválido", s)
	}
}

func TestScenarioValidation(t *testing.T) {
	valid := Scenario{
		Feature:  "Auth",
		Scenario: "Login exitoso",
		Given:    []string{"usuario existe"},
		When:     "login",
		Then:     []string{"token devuelto"},
	}
	require.NoError(t, validateScenario(valid))

	require.Error(t, validateScenario(Scenario{Feature: "", Scenario: "x", Given: []string{"a"}, When: "x", Then: []string{"b"}}))
	require.Error(t, validateScenario(Scenario{Feature: "F", Scenario: "", Given: []string{"a"}, When: "x", Then: []string{"b"}}))
	require.Error(t, validateScenario(Scenario{Feature: "F", Scenario: "x", Given: nil, When: "x", Then: []string{"b"}}))
	require.Error(t, validateScenario(Scenario{Feature: "F", Scenario: "x", Given: []string{"a"}, When: "", Then: []string{"b"}}))
	require.Error(t, validateScenario(Scenario{Feature: "F", Scenario: "x", Given: []string{"a"}, When: "x", Then: nil}))
}

func TestValidStatusesAndPriorities(t *testing.T) {
	for _, s := range []string{StatusProposed, StatusActive, StatusImplemented, StatusArchived} {
		require.True(t, validStatuses[s])
	}
	for _, p := range []string{PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical} {
		require.True(t, validPriorities[p])
	}
}

func TestCreateValidation(t *testing.T) {
	s := &Service{}
	_, err := s.Create(nil, "", "title", "", "", "", "REQ-01", nil)
	require.ErrorIs(t, err, ErrSlugInvalid)

	_, err = s.Create(nil, "HU-01.1", "", "", "", "", "REQ-01", nil)
	require.Error(t, err)

	_, err = s.Create(nil, "HU-01.1", "title", "", "bogus", "", "REQ-01", nil)
	require.ErrorIs(t, err, ErrInvalidStatus)
}
