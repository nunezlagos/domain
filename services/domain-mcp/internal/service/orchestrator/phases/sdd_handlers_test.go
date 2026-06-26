package phases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllHandlers_SlugIdentity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		want    PhaseSlug
		factory func() Handler
	}{
		{"sdd-explore", NewSDDExploreHandler},
		{"sdd-spec", NewSDDSpecHandler},
		{"sdd-propose", NewSDDProposeHandler},
		{"sdd-design", NewSDDDesignHandler},
		{"sdd-tasks", NewSDDTasksHandler},
		{"sdd-apply", NewSDDApplyHandler},
		{"sdd-verify", NewSDDVerifyHandler},
		{"sdd-judge", NewSDDJudgeHandler},
		{"sdd-review", NewSDDReviewHandler},
		{"sdd-archive", NewSDDArchiveHandler},
		{"sdd-onboard", NewSDDOnboardHandler},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.want), func(t *testing.T) {
			t.Parallel()
			h := tc.factory()
			require.Equal(t, tc.want, h.Slug())
		})
	}
}

func TestAllHandlers_BuildRejectsEmptyRawText(t *testing.T) {
	t.Parallel()
	handlers := []Handler{
		NewSDDExploreHandler(), NewSDDSpecHandler(), NewSDDProposeHandler(),
		NewSDDDesignHandler(), NewSDDTasksHandler(),
		NewSDDJudgeHandler(), NewSDDReviewHandler(), NewSDDArchiveHandler(),
		NewSDDOnboardHandler(),
	}
	for _, h := range handlers {
		h := h
		t.Run(string(h.Slug()), func(t *testing.T) {
			t.Parallel()
			_, err := h.Build(context.Background(), Input{})
			require.Error(t, err)
		})
	}
}

func TestAllHandlers_BuildReturnsEmptySystemPrompt_BDSourceOfTruth(t *testing.T) {
	t.Parallel()
	handlers := []Handler{
		NewSDDExploreHandler(), NewSDDSpecHandler(), NewSDDProposeHandler(),
		NewSDDDesignHandler(), NewSDDTasksHandler(), NewSDDApplyHandler(),
		NewSDDVerifyHandler(), NewSDDJudgeHandler(), NewSDDReviewHandler(),
		NewSDDArchiveHandler(), NewSDDOnboardHandler(),
	}
	for _, h := range handlers {
		h := h
		t.Run(string(h.Slug()), func(t *testing.T) {
			t.Parallel()
			out, err := h.Build(context.Background(), Input{RawText: "any"})
			require.NoError(t, err)
			require.Empty(t, out.SystemPrompt,
				"handler.Build NO debe hardcodear system_prompt — BD es source-of-truth (Service.hydrateSystemPrompts)")
			require.Equal(t, string(h.Slug()), out.AgentTemplateSlug,
				"AgentTemplateSlug debe coincidir con el slug del handler")
		})
	}
}

// D5 contract: propose + design + tasks + apply + judge tienen un
// SuggestedSave Required=true (propose/design/tasks por Feature B: el
// documento de la fase queda registrado en BD); el resto no.
func TestHandlers_D5RequiredSavesByPhase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		factory      func() Handler
		requiredType string // "" si no hay required
	}{
		{NewSDDExploreHandler, ""},
		{NewSDDSpecHandler, ""},
		{NewSDDProposeHandler, "knowledge_doc"},
		{NewSDDDesignHandler, "adr"},
		{NewSDDTasksHandler, "knowledge_doc"},
		{NewSDDApplyHandler, "code_reference"},
		{NewSDDVerifyHandler, ""},
		{NewSDDJudgeHandler, "sabotage_record"},
		{NewSDDReviewHandler, ""},
		{NewSDDArchiveHandler, ""},
		{NewSDDOnboardHandler, ""},
	}
	for _, tc := range cases {
		tc := tc
		h := tc.factory()
		t.Run(string(h.Slug()), func(t *testing.T) {
			t.Parallel()
			out, err := h.Build(context.Background(), Input{RawText: "x"})
			require.NoError(t, err)
			var requiredFound string
			for _, s := range out.SuggestedSaves {
				if s.Required {
					requiredFound = s.Type
					break
				}
			}
			require.Equal(t, tc.requiredType, requiredFound,
				"D5 contract rompió para fase %s", h.Slug())
		})
	}
}

// Validators happy path por fase
func TestSDDExploreHandler_Validate_HappyPath(t *testing.T) {
	t.Parallel()
	h := NewSDDExploreHandler()
	require.NoError(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"intent": "feature", "scope": "single-file"},
	}))
}

func TestSDDExploreHandler_Validate_MissingIntent(t *testing.T) {
	t.Parallel()
	h := NewSDDExploreHandler()
	require.Error(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"scope": "single-file"},
	}))
}

func TestSDDSpecHandler_Validate_RequiresSlugAndMD(t *testing.T) {
	t.Parallel()
	h := NewSDDSpecHandler()
	require.Error(t, h.Validate(context.Background(), nil, ClientResult{Output: map[string]any{}}))
	require.Error(t, h.Validate(context.Background(), nil, ClientResult{Output: map[string]any{"issue_slug": "issue-9.1-x"}}))
	require.NoError(t, h.Validate(context.Background(), nil, ClientResult{Output: map[string]any{
		"issue_slug": "issue-9.1-x", "issue_md": "# spec",
	}}))
}

func TestSDDProposeHandler_Validate_RequiresDraftStatus(t *testing.T) {
	t.Parallel()
	h := NewSDDProposeHandler()
	require.NoError(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"proposal_md": "...", "status": "draft"},
	}))

	require.Error(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"proposal_md": "...", "status": "approved"},
	}))
}

func TestSDDDesignHandler_Validate_RequiresAtLeastOneADR(t *testing.T) {
	t.Parallel()
	h := NewSDDDesignHandler()
	require.NoError(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{
			"design_md": "...",
			"adrs":      []any{map[string]any{"id": "ADR-1"}},
		},
	}))
	require.Error(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"design_md": "...", "adrs": []any{}},
	}))
}

func TestSDDTasksHandler_Validate_RequiresShapedTasks(t *testing.T) {
	t.Parallel()
	h := NewSDDTasksHandler()
	require.NoError(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{
			"tasks": []any{
				map[string]any{"id": "T01", "description": "x"},
			},
		},
	}))

	require.Error(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{
			"tasks": []any{map[string]any{"description": "x"}},
		},
	}))
}

func TestSDDJudgeHandler_Validate_RequiresSabotageRecords(t *testing.T) {
	t.Parallel()
	h := NewSDDJudgeHandler()
	require.NoError(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{
			"sabotage_records": []any{map[string]any{"invariant": "x"}},
		},
	}))
	require.Error(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"sabotage_records": []any{}},
	}))
}

func TestSDDReviewHandler_Validate_GatesOnVerdict(t *testing.T) {
	t.Parallel()
	h := NewSDDReviewHandler()
	// compliant → pasa
	require.NoError(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"verdict": "compliant"},
	}))
	// violations_found → bloquea el flow (error sentinela)
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"verdict": "violations_found"},
	})
	require.ErrorIs(t, err, ErrPolicyReviewFailed)
	// verdict ausente o desconocido → error
	require.Error(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{},
	}))
	require.Error(t, h.Validate(context.Background(), nil, ClientResult{Output: nil}))
}

func TestSDDArchiveHandler_Validate_RequiresArchivedFlag(t *testing.T) {
	t.Parallel()
	h := NewSDDArchiveHandler()
	require.NoError(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"archived": true},
	}))
	require.Error(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"archived": false},
	}))
}

func TestSDDOnboardHandler_Validate_SkippedOrDocCreated(t *testing.T) {
	t.Parallel()
	h := NewSDDOnboardHandler()

	require.NoError(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"skipped": true},
	}))

	require.NoError(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"doc_created": true},
	}))

	require.Error(t, h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{},
	}))
}
