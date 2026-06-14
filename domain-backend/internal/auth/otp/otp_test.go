// issue-02.7 OTP unit tests.

package otp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateCode_LengthSix(t *testing.T) {
	for i := 0; i < 50; i++ {
		code, err := generateCode(6)
		require.NoError(t, err)
		require.Len(t, code, 6)
		// Solo dígitos
		for _, c := range code {
			require.Contains(t, "0123456789", string(c))
		}
	}
}

func TestGenerateCode_VariedLengths(t *testing.T) {
	for _, n := range []int{4, 6, 8} {
		code, err := generateCode(n)
		require.NoError(t, err)
		require.Len(t, code, n)
	}
}

func TestGenerateCode_Random(t *testing.T) {
	seen := map[string]int{}
	for i := 0; i < 200; i++ {
		c, _ := generateCode(6)
		seen[c]++
	}
	// Con 200 generaciones de espacio 1M, esperamos casi todos únicos
	require.Greater(t, len(seen), 190, "duplicados sospechosamente altos: %d unique de 200", len(seen))
}

func TestIsEmail(t *testing.T) {
	require.True(t, IsEmail("alice@example.com"))
	require.True(t, IsEmail("a@b"))
	require.False(t, IsEmail("12345678-5"))
	require.False(t, IsEmail("12.345.678-5"))
	require.False(t, IsEmail(""))
}

// Sabotaje: padding de zeros funciona (code < 10^6).
func TestSabotage_CodePadding(t *testing.T) {
	// Generamos muchos códigos y verificamos que NO contienen length variable
	for i := 0; i < 100; i++ {
		c, _ := generateCode(6)
		require.Len(t, c, 6, "code %q debe ser exactly 6 chars con padding", c)
	}
}

// Sabotaje: códigos no contienen letras
func TestSabotage_OnlyDigits(t *testing.T) {
	for i := 0; i < 50; i++ {
		c, _ := generateCode(6)
		require.False(t, strings.ContainsAny(c, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"),
			"code %q contiene letras", c)
	}
}
