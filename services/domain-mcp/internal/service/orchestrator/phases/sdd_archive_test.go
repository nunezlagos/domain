package phases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSddArchiveValidate_Archived_OK(t *testing.T) {
	h := &sddArchiveHandler{}
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"archived": true},
	})
	require.NoError(t, err)
}

// DOMAINSERV-89: en Lite sin issue/change, nothing_to_archive cierra la fase.
func TestSddArchiveValidate_NothingToArchive_OK(t *testing.T) {
	h := &sddArchiveHandler{}
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"nothing_to_archive": true},
	})
	require.NoError(t, err)
}

func TestSddArchiveValidate_NingunFlag_Error(t *testing.T) {
	h := &sddArchiveHandler{}
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"archived": false},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing_to_archive")
}

// Sin sdd-spec previo (modo Lite), el prompt guía al nothing_to_archive.
func TestSddArchiveBuild_SinIssue_EmiteHintNothingToArchive(t *testing.T) {
	h := &sddArchiveHandler{}
	out, err := h.Build(context.Background(), Input{RawText: "fix trivial"})
	require.NoError(t, err)
	assert.Contains(t, out.UserPrompt, "nothing_to_archive")
}

// Con issue_slug de sdd-spec (modo Full), archiva ese issue y no emite el hint.
func TestSddArchiveBuild_ConIssue_ArchivaEseIssue(t *testing.T) {
	h := &sddArchiveHandler{}
	out, err := h.Build(context.Background(), Input{
		RawText: "feature X",
		PriorOutputs: map[PhaseSlug]map[string]any{
			PhaseSlug("sdd-spec"): {"issue_slug": "issue-42"},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, out.UserPrompt, "issue-42")
	assert.NotContains(t, out.UserPrompt, "nothing_to_archive")
}
