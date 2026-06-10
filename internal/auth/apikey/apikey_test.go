// HU-02.1 api-key-auth unit tests.

package apikey

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerate_PrefixAndHashValid(t *testing.T) {
	pt, prefix, hash, err := Generate("live")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(pt, "domk_live_"))
	require.Equal(t, PrefixLen, len(prefix))
	require.Equal(t, pt[:PrefixLen], prefix)
	require.NotEmpty(t, hash)
}

func TestGenerate_InvalidEnv(t *testing.T) {
	_, _, _, err := Generate("prod")
	require.ErrorIs(t, err, ErrInvalidEnv)
	_, _, _, err = Generate("")
	require.ErrorIs(t, err, ErrInvalidEnv)
}

func TestVerify_HappyPath(t *testing.T) {
	pt, _, hash, _ := Generate("dev")
	require.NoError(t, Verify(pt, hash))
}

func TestVerify_WrongKey(t *testing.T) {
	_, _, hash, _ := Generate("dev")
	otherPT, _, _, _ := Generate("dev")
	err := Verify(otherPT, hash)
	require.Error(t, err)
}

func TestVerify_TamperedKey(t *testing.T) {
	pt, _, hash, _ := Generate("dev")
	tampered := pt[:len(pt)-1] + "X"
	require.Error(t, Verify(tampered, hash))
}

func TestVerify_InvalidFormat(t *testing.T) {
	require.ErrorIs(t, Verify("not_a_key", []byte{}), ErrInvalidFormat)
}

func TestParsePrefix(t *testing.T) {
	pt, expected, _, _ := Generate("test")
	prefix, err := ParsePrefix(pt)
	require.NoError(t, err)
	require.Equal(t, expected, prefix)
}

func TestParsePrefix_TooShort(t *testing.T) {
	_, err := ParsePrefix("domk")
	require.ErrorIs(t, err, ErrInvalidFormat)
}

func TestParsePrefix_BadFormat(t *testing.T) {
	_, err := ParsePrefix("garbage_" + strings.Repeat("x", 20))
	require.ErrorIs(t, err, ErrInvalidFormat)
}

func TestParsePrefix_BadEnv(t *testing.T) {
	_, err := ParsePrefix("domk_zzz_" + strings.Repeat("x", 20))
	require.ErrorIs(t, err, ErrInvalidEnv)
}

func TestIsAPIKeyFormat(t *testing.T) {
	pt, _, _, _ := Generate("live")
	require.True(t, IsAPIKeyFormat(pt))
	require.False(t, IsAPIKeyFormat("not a key"))
	require.False(t, IsAPIKeyFormat(""))
}

// Sabotaje: bcrypt cost mínimo asegura computacionalmente caro verify.
// (No medimos en CI por flakiness; verificamos que el cost esté seteado >=10.)
func TestSabotage_BcryptCostAtLeast10(t *testing.T) {
	require.GreaterOrEqual(t, BcryptCost, 10, "bcrypt cost <10 es inseguro")
}

// Sabotaje: empty key no puede pasar Verify ni GeneratePlaintext.
func TestSabotage_EmptyKeyRejected(t *testing.T) {
	require.Error(t, Verify("", []byte("hash")), "empty plaintext must fail")
	_, _, err := GeneratePlaintext("")
	require.ErrorIs(t, err, ErrInvalidEnv)
}

// Sabotaje: si Verify siempre retorna nil, una key random no debe autenticar.
func TestSabotage_VerifyWithWrongKey(t *testing.T) {
	pt, _, hash, err := Generate("dev")
	require.NoError(t, err)
	require.NoError(t, Verify(pt, hash))
	// misma key con byte corrupto debe fallar
	wrong := pt[:len(pt)-1] + "X"
	require.Error(t, Verify(wrong, hash), "corrupted key must fail verify")

	// hash completamente distinto debe fallar
	require.Error(t, Verify(pt, []byte("$2a$12$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")), "bogus hash must fail")
}

// Sabotaje: keys generados consecutivos NO repiten (random).
// Usa GeneratePlaintext para evitar bcrypt overhead (mide solo randomness).
func TestSabotage_KeysAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		pt, _, err := GeneratePlaintext("live")
		require.NoError(t, err)
		require.False(t, seen[pt], "key colision en iteración %d", i)
		seen[pt] = true
	}
}
