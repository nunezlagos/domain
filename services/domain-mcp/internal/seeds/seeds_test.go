

package seeds

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// fakeSeeder testing helper.
type fakeSeeder struct {
	name      string
	version   int
	order     int
	devOnly   bool
	runFn     func(ctx context.Context, tx pgx.Tx, env Env) (Report, error)
	callCount int
}

func (f *fakeSeeder) Name() string  { return f.name }
func (f *fakeSeeder) Version() int  { return f.version }
func (f *fakeSeeder) Order() int    { return f.order }
func (f *fakeSeeder) IsDevOnly() bool { return f.devOnly }
func (f *fakeSeeder) Run(ctx context.Context, tx pgx.Tx, env Env) (Report, error) {
	f.callCount++
	if f.runFn != nil {
		return f.runFn(ctx, tx, env)
	}
	return Report{Created: 1}, nil
}

func TestRegistry_Sorted_ByOrderAndName(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeSeeder{name: "zeta", order: 1})
	r.Register(&fakeSeeder{name: "alpha", order: 1})
	r.Register(&fakeSeeder{name: "first", order: 0})
	got := r.Sorted()
	require.Len(t, got, 3)
	require.Equal(t, "first", got[0].Name())
	require.Equal(t, "alpha", got[1].Name()) // secondary sort by name
	require.Equal(t, "zeta", got[2].Name())
}

func TestRegistry_Find(t *testing.T) {
	r := NewRegistry()
	a := &fakeSeeder{name: "a"}
	r.Register(a)
	require.Equal(t, a, r.Find("a"))
	require.Nil(t, r.Find("missing"))
}

func TestRegistry_Names(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeSeeder{name: "b"})
	r.Register(&fakeSeeder{name: "a"})
	require.Equal(t, []string{"a", "b"}, r.Names())
}

// Sabotaje: fake seeder que devuelve error → propaga correctamente.
func TestSabotage_SeederError_Propagates(t *testing.T) {
	s := &fakeSeeder{
		name: "bad",
		runFn: func(_ context.Context, _ pgx.Tx, _ Env) (Report, error) {
			return Report{}, errors.New("explosion")
		},
	}

	rep, err := s.Run(context.Background(), nil, EnvDev)
	require.Error(t, err)
	require.Contains(t, err.Error(), "explosion")
	require.Zero(t, rep.Created)
}

func TestEnv_Constants(t *testing.T) {
	require.Equal(t, Env("dev"), EnvDev)
	require.Equal(t, Env("staging"), EnvStaging)
	require.Equal(t, Env("prod"), EnvProd)
}

func TestFakeSeeder_DevOnlyFlag(t *testing.T) {
	s := &fakeSeeder{name: "demo", devOnly: true}
	require.True(t, s.IsDevOnly())
	s2 := &fakeSeeder{name: "core"}
	require.False(t, s2.IsDevOnly())
}
