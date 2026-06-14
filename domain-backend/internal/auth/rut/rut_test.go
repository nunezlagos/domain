// issue-02.7 RUT chileno unit tests.

package rut

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Tabla de RUTs reales válidos (DVs calculados con algoritmo módulo 11).
// Para verificar: dv = (11 - (sum%11)); si 11→"0", si 10→"K", else digit.
var validRUTs = []struct {
	input     string
	canonical string
}{
	{"12.345.678-5", "12345678-5"},
	{"12345678-5", "12345678-5"},
	{"123456785", "12345678-5"},
	{"11.111.111-1", "11111111-1"},
	{"22.222.222-2", "22222222-2"},
	{"33333333-3", "33333333-3"},
	{"44444444-4", "44444444-4"},
	{"1-9", "1-9"},
	{"19", "1-9"},     // sin guión
	{"5-1", "5-1"},
}

// Tabla de inválidos (formato ok, DV malo).
var invalidDVRUTs = []string{
	"12.345.678-9", // DV correcto: 5
	"11.111.111-2", // DV correcto: 1
	"1-0",          // DV correcto: 9
	"5-K",          // DV correcto: 1
	"44.444.444-K", // DV correcto: 4
}

// Tabla malformados (rechazados por Normalize antes incluso de check DV).
var malformedRUTs = []string{
	"",
	"abc",         // sin dígitos
	"!@#$",        // sin dígitos
	"-9",          // sin body
	"100000000-K", // body > 99M
}

func TestNormalize_ValidFormats(t *testing.T) {
	for _, tc := range validRUTs {
		got, err := Normalize(tc.input)
		require.NoErrorf(t, err, "Normalize(%q)", tc.input)
		require.Equalf(t, tc.canonical, got, "Normalize(%q)", tc.input)
	}
}

func TestNormalize_Empty(t *testing.T) {
	_, err := Normalize("")
	require.ErrorIs(t, err, ErrEmpty)
}

func TestNormalize_Malformed(t *testing.T) {
	for _, raw := range malformedRUTs {
		if raw == "" {
			continue // empty es ErrEmpty, no malformed
		}
		_, err := Normalize(raw)
		require.Errorf(t, err, "should reject %q", raw)
	}
}

func TestValidate_ValidRUTs(t *testing.T) {
	for _, tc := range validRUTs {
		canonical, err := Validate(tc.input)
		require.NoErrorf(t, err, "Validate(%q)", tc.input)
		require.Equal(t, tc.canonical, canonical)
	}
}

func TestValidate_InvalidDV(t *testing.T) {
	for _, raw := range invalidDVRUTs {
		_, err := Validate(raw)
		require.ErrorIsf(t, err, ErrInvalidCheckDigit, "should reject DV %q", raw)
	}
}

func TestValidate_MalformedRejected(t *testing.T) {
	for _, raw := range malformedRUTs {
		_, err := Validate(raw)
		require.Errorf(t, err, "should reject malformed %q", raw)
	}
}

func TestCheckDigit_KnownValues(t *testing.T) {
	// Algoritmo módulo 11 chileno: pesos 2..7 cíclicos desde menos significativo.
	cases := map[int]string{
		1:        "9",
		5:        "1",
		12345678: "5",
		11111111: "1",
		22222222: "2",
		33333333: "3",
		44444444: "4",
	}
	for body, expected := range cases {
		require.Equalf(t, expected, CheckDigit(body), "CheckDigit(%d)", body)
	}
}

func TestIsValid(t *testing.T) {
	require.True(t, IsValid("12.345.678-5"))
	require.True(t, IsValid("1-9"))
	require.False(t, IsValid("12.345.678-9"))
	require.False(t, IsValid("garbage"))
}

// Sabotaje: idempotencia — Normalize(canonical) == canonical.
func TestSabotage_NormalizeIdempotent(t *testing.T) {
	for _, tc := range validRUTs {
		once, err := Normalize(tc.input)
		require.NoError(t, err)
		twice, err := Normalize(once)
		require.NoError(t, err)
		require.Equal(t, once, twice, "Normalize debe ser idempotente")
	}
}

// Sabotaje: roundtrip body→CheckDigit→Validate (todos los body 1..1000 son válidos).
func TestSabotage_RoundTrip(t *testing.T) {
	for body := 1; body < 1000; body++ {
		dv := CheckDigit(body)
		s := joinBodyDV(body, dv)
		canon, err := Validate(s)
		require.NoErrorf(t, err, "body=%d dv=%s rut=%s", body, dv, s)
		require.Containsf(t, canon, dv, "canonical %s should contain dv %s", canon, dv)
	}
}

func joinBodyDV(body int, dv string) string {
	if body == 0 {
		return "0-" + dv
	}
	digits := []byte{}
	for body > 0 {
		digits = append([]byte{byte('0' + body%10)}, digits...)
		body /= 10
	}
	return string(digits) + "-" + dv
}
