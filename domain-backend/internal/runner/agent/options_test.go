package agentrunner

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestResolveRunOpts_DefaultIsStandalone(t *testing.T) {
	t.Parallel()
	o := resolveRunOpts(nil)
	require.True(t, o.standalone, "legacy callers (no options) deben quedar standalone")
	require.Nil(t, o.flowRunID)
	require.Nil(t, o.flowRunStepID)
}

func TestResolveRunOpts_WithFlowRun_TogglesStandaloneOff(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	o := resolveRunOpts([]RunOption{WithFlowRun(id)})
	require.False(t, o.standalone, "WithFlowRun debe marcar el run como gobernado")
	require.NotNil(t, o.flowRunID)
	require.Equal(t, id, *o.flowRunID)
}

func TestResolveRunOpts_WithStandaloneExplicit(t *testing.T) {
	t.Parallel()
	o := resolveRunOpts([]RunOption{WithStandalone(false)})
	require.False(t, o.standalone)
}

func TestBuildRunMetadata_Standalone(t *testing.T) {
	t.Parallel()
	o := resolveRunOpts(nil)
	got := string(buildRunMetadata(o, ""))
	require.Equal(t, `{"standalone":true,"reason":"direct_invocation"}`, got)
	got2 := string(buildRunMetadata(o, "direct_invocation_failed"))
	require.Equal(t, `{"standalone":true,"reason":"direct_invocation_failed"}`, got2)
}

func TestBuildRunMetadata_OrchestratedRunIsEmpty(t *testing.T) {
	t.Parallel()
	o := resolveRunOpts([]RunOption{WithFlowRun(uuid.New())})
	require.Equal(t, `{}`, string(buildRunMetadata(o, "irrelevant")))
}

func TestCheckOrphanPolicy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		env        string
		opts       []RunOption
		wantErr    error
	}{
		{
			name:    "dev sin opts → permitido",
			env:     "dev",
			opts:    nil,
			wantErr: nil,
		},
		{
			name:    "staging sin opts → permitido",
			env:     "staging",
			opts:    nil,
			wantErr: nil,
		},
		{
			name:    "prod sin opts (default standalone=true) → permitido",
			env:     "prod",
			opts:    nil,
			wantErr: nil,
		},
		{
			name:    "prod standalone=false sin flow_run → bloqueado",
			env:     "prod",
			opts:    []RunOption{WithStandalone(false)},
			wantErr: ErrOrphanRunNotAllowed,
		},
		{
			name:    "prod con WithFlowRun → permitido",
			env:     "prod",
			opts:    []RunOption{WithFlowRun(uuid.New())},
			wantErr: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := &Runner{Env: tc.env}
			err := r.checkOrphanPolicy(resolveRunOpts(tc.opts))
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}
