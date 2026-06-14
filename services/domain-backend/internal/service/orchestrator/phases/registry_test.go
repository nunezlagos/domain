package phases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeHandler struct{ slug PhaseSlug }

func (f *fakeHandler) Slug() PhaseSlug { return f.slug }
func (f *fakeHandler) Build(_ context.Context, _ Input) (*Output, error) {
	return &Output{AgentTemplateSlug: string(f.slug)}, nil
}
func (f *fakeHandler) Validate(_ context.Context, _ *Output, _ ClientResult) error { return nil }

func TestRegistry_RegisterAndLookup(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	require.NoError(t, r.Register(&fakeHandler{slug: "sdd-explore"}))
	h, err := r.Lookup("sdd-explore")
	require.NoError(t, err)
	out, err := h.Build(context.Background(), Input{})
	require.NoError(t, err)
	require.Equal(t, "sdd-explore", out.AgentTemplateSlug)
}

func TestRegistry_DuplicateRegisterRejected(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	require.NoError(t, r.Register(&fakeHandler{slug: "sdd-design"}))
	err := r.Register(&fakeHandler{slug: "sdd-design"})
	require.ErrorIs(t, err, ErrPhaseAlreadyExists)
}

func TestRegistry_LookupMissingReturnsError(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	_, err := r.Lookup("sdd-nope")
	require.ErrorIs(t, err, ErrPhaseNotRegistered)
}

func TestRegistry_MustRegisterPanicsOnDup(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.MustRegister(&fakeHandler{slug: "sdd-tasks"})
	require.Panics(t, func() { r.MustRegister(&fakeHandler{slug: "sdd-tasks"}) })
}

func TestRegistry_NilHandlerRejected(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	require.Error(t, r.Register(nil))
}
