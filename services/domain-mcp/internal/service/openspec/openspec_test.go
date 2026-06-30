package openspec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleChange() *Change {
	return &Change{
		IssueID:   "11111111-1111-1111-1111-111111111111",
		IssueSlug: "login-email-password",
		Title:     "Login con email y contraseña",
		ReqSlug:   "REQ-01-auth",
		Status:    "active",
		Priority:  "high",
		Proposal: &ProposalDoc{
			Intention:    "Permitir login nativo",
			Scope:        "Solo email + password",
			Approach:     "PBKDF2 + cookie",
			Risks:        "timing attack",
			TestingNotes: "tests de credenciales inválidas",
		},
		Design: &DesignDoc{
			ArchDecisions:   "Web Crypto API",
			Alternatives:    "bcrypt nativo",
			DataFlow:        "form -> endpoint -> KV",
			TDDPlan:         "red green refactor",
			RisksMitigation: "rate limit",
		},
		Tasks: []TaskDoc{
			{ID: "aaaa", Section: "Implementación", Text: "crear endpoint", Completed: true},
			{ID: "bbbb", Section: "Tests", Text: "test happy path", Completed: false},
		},
		Scenarios: []ScenarioDoc{
			{Feature: "Auth", Scenario: "login ok", Given: []string{"usuario existe"}, When: "envío credenciales", Then: []string{"recibo 200", "se setea cookie"}},
		},
		ProposalVersion: 2,
		DesignVersion:   1,
	}
}

func TestProposalRoundTrip(t *testing.T) {
	c := sampleChange()
	r := Render(c, false, "")
	got := ParseProposal(r.Files[fileProposal])
	assert.Equal(t, *c.Proposal, got)
}

func TestDesignRoundTrip(t *testing.T) {
	c := sampleChange()
	r := Render(c, false, "")
	got := ParseDesign(r.Files[fileDesign])
	assert.Equal(t, *c.Design, got)
}

func TestTasksRoundTrip(t *testing.T) {
	c := sampleChange()
	r := Render(c, false, "")
	got := ParseTasks(r.Files[fileTasks])
	require.Len(t, got, 2)
	assert.Equal(t, c.Tasks, got)
}

func TestScenariosRoundTrip(t *testing.T) {
	c := sampleChange()
	r := Render(c, false, "")
	got := ParseScenarios(r.Files[specFile(c.IssueSlug)])
	require.Len(t, got, 1)
	assert.Equal(t, "login ok", got[0].Scenario)
	assert.Equal(t, []string{"usuario existe"}, got[0].Given)
	assert.Equal(t, "envío credenciales", got[0].When)
	assert.Equal(t, []string{"recibo 200", "se setea cookie"}, got[0].Then)
}

func TestEmptySectionRoundTrips(t *testing.T) {
	c := sampleChange()
	c.Proposal.Risks = ""
	r := Render(c, false, "")
	assert.Contains(t, r.Files[fileProposal], emptySentinel)
	got := ParseProposal(r.Files[fileProposal])
	assert.Equal(t, "", got.Risks)
}

func TestMetaRoundTrip(t *testing.T) {
	c := sampleChange()
	r := Render(c, false, "")
	m := ParseMeta(r.Files[fileMeta])
	assert.Equal(t, c.IssueID, m.IssueID)
	assert.Equal(t, c.IssueSlug, m.IssueSlug)
	assert.Equal(t, c.Status, m.Status)
	assert.Equal(t, r.Hashes[fileProposal], m.Hashes[fileProposal])
}

func TestArchivePath(t *testing.T) {
	c := sampleChange()
	r := Render(c, true, "2026-06-26")
	assert.Equal(t, "openspec/changes/archive/2026-06-26-login-email-password/", r.Dir)
}

func TestRenderHashesMatchContent(t *testing.T) {
	c := sampleChange()
	r := Render(c, false, "")
	assert.Equal(t, ContentHash(r.Files[fileProposal]), r.Hashes[fileProposal])
}
