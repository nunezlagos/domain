package acp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRoller_Next_RoundRobin_CiclaModelos(t *testing.T) {
	r := newRoller(t.TempDir(), time.Minute, nil)
	r.setRoster([]string{"a", "b", "c"})

	var got []string
	for i := 0; i < 4; i++ {
		m, _, ok := r.next()
		require.True(t, ok)
		got = append(got, m)
	}
	require.Equal(t, []string{"a", "b", "c", "a"}, got)
}

func TestRoller_Next_ModeloEnCooldown_LoSaltea(t *testing.T) {
	base := time.Unix(1000, 0)
	r := newRoller(t.TempDir(), time.Minute, nil)
	r.now = func() time.Time { return base }
	r.setRoster([]string{"a", "b"})

	r.cooldownModel("a")

	m, _, ok := r.next()
	require.True(t, ok)
	require.Equal(t, "b", m, "el modelo en cooldown se saltea")
}

func TestRoller_SetRoster_Vacio_ConservaUltimoRoster(t *testing.T) {
	r := newRoller(t.TempDir(), time.Minute, nil)
	r.setRoster([]string{"a", "b"})

	r.setRoster(nil) // discover fallido/vacio

	require.Equal(t, 2, r.size(), "conserva el ultimo roster conocido")
}

func TestParseFreeModels_FiltraCostCero(t *testing.T) {
	out := `opencode/free-one
{
  "id": "free-one",
  "providerID": "opencode",
  "cost": {
    "input": 0,
    "output": 0
  }
}
opencode/paid-one
{
  "id": "paid-one",
  "providerID": "opencode",
  "cost": {
    "input": 3,
    "output": 15
  }
}`
	got := parseFreeModels([]byte(out))
	require.Equal(t, []string{"opencode/free-one"}, got)
}
