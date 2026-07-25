package apikey

import (
	"encoding/hex"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSigningSecret_Derive_SamePlaintext_IsDeterministic(t *testing.T) {
	a := SigningSecret("domk_live_abcdefghijklmnop")
	b := SigningSecret("domk_live_abcdefghijklmnop")
	require.Equal(t, a, b)
	require.Len(t, a, 32)
}

func TestSigningSecret_Derive_DifferentPlaintext_DiffersAndHidesKey(t *testing.T) {
	plaintext := "domk_live_abcdefghijklmnop"
	secret := SigningSecret(plaintext)
	require.NotEqual(t, secret, SigningSecret(plaintext+"x"))
	// el secreto derivado no debe contener la key en claro: si se filtra, no
	// entrega la credencial original
	require.NotContains(t, string(secret), plaintext)
}

func TestParseSignatureHeader_Parse_ValidHeader_ExtractsAllParams(t *testing.T) {
	sig := strings.Repeat("ab", 32)
	h := HMACScheme + " key=domk_live_aaaaaa,ts=1750000000,nonce=n-1,sig=" + sig
	p, err := ParseSignatureHeader(h)
	require.NoError(t, err)
	require.Equal(t, "domk_live_aaaaaa", p.KeyPrefix)
	require.Equal(t, int64(1750000000), p.Timestamp)
	require.Equal(t, "n-1", p.Nonce)
	require.Equal(t, sig, p.Signature)
}

func TestParseSignatureHeader_Parse_FieldOrderShuffled_StillParses(t *testing.T) {
	sig := strings.Repeat("cd", 32)
	h := HMACScheme + " sig=" + sig + ",nonce=n-2,key=domk_test_aaaaaa,ts=42"
	p, err := ParseSignatureHeader(h)
	require.NoError(t, err)
	require.Equal(t, "domk_test_aaaaaa", p.KeyPrefix)
	require.Equal(t, int64(42), p.Timestamp)
}

func TestParseSignatureHeader_Parse_MalformedHeader_ReturnsError(t *testing.T) {
	good := strings.Repeat("ab", 32)
	cases := map[string]string{
		"scheme ausente":  "Bearer key=domk_live_aaaaaa,ts=1,nonce=n,sig=" + good,
		"sin key":         HMACScheme + " ts=1,nonce=n,sig=" + good,
		"sin ts":          HMACScheme + " key=domk_live_aaaaaa,nonce=n,sig=" + good,
		"sin nonce":       HMACScheme + " key=domk_live_aaaaaa,ts=1,sig=" + good,
		"sin sig":         HMACScheme + " key=domk_live_aaaaaa,ts=1,nonce=n",
		"ts no numerico":  HMACScheme + " key=domk_live_aaaaaa,ts=ayer,nonce=n,sig=" + good,
		"sig no hex":      HMACScheme + " key=domk_live_aaaaaa,ts=1,nonce=n,sig=" + strings.Repeat("z", 64),
		"sig corta":       HMACScheme + " key=domk_live_aaaaaa,ts=1,nonce=n,sig=abcd",
		"prefix invalido": HMACScheme + " key=nope_live_aaaaaa,ts=1,nonce=n,sig=" + good,
		"nonce vacio":     HMACScheme + " key=domk_live_aaaaaa,ts=1,nonce=,sig=" + good,
		"duplicado":       HMACScheme + " key=domk_live_aaaaaa,key=domk_live_bbbbbb,ts=1,nonce=n,sig=" + good,
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseSignatureHeader(h)
			require.Error(t, err)
		})
	}
}

func TestParseSignatureHeader_Parse_OversizedNonce_ReturnsError(t *testing.T) {
	h := HMACScheme + " key=domk_live_aaaaaa,ts=1,nonce=" + strings.Repeat("n", MaxNonceLen+1) +
		",sig=" + strings.Repeat("ab", 32)
	_, err := ParseSignatureHeader(h)
	require.ErrorIs(t, err, ErrInvalidSignatureHeader)
}

func TestCanonicalRequest_Build_KnownInput_MatchesGoldenVector(t *testing.T) {
	got := CanonicalRequest("get", "/api/v1/projects?limit=2", BodySHA256(nil), 1750000000, "n-1")
	want := "GET\n/api/v1/projects?limit=2\n" +
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\n" +
		"1750000000\nn-1"
	require.Equal(t, want, got)
}

// Golden vector del contrato de firma, calculado fuera de Go a partir de la
// especificación (python hmac/hashlib): el SDK (otro módulo) tiene que llegar
// a este mismo hex. Si cambia, toda firma ya emitida deja de validar.
func TestSign_Compute_GoldenVector_IsStable(t *testing.T) {
	require.Equal(t,
		"ab2596ca50c30bfe2d947ea62a0c9e353a43df13c3bfe1175a392902c9744289",
		hex.EncodeToString(SigningSecret("domk_live_TESTKEYTESTKEY")))

	canonical := CanonicalRequest("POST", "/api/v1/projects", BodySHA256([]byte(`{"slug":"demo"}`)), 1750000000, "n-1")
	got := Sign(SigningSecret("domk_live_TESTKEYTESTKEY"), canonical)
	require.Equal(t, "d2ca225062b3783c9829c05bdf9f90847412cfcc27a1f4d8e7f8f90991dc5e91", got)
}

func TestSignatureMatches_Compare_TamperedBody_ReturnsFalse(t *testing.T) {
	secret := SigningSecret("domk_live_abcdefghijklmnop")
	original := CanonicalRequest("POST", "/api/v1/x", BodySHA256([]byte("a")), 10, "n")
	sig := Sign(secret, original)
	tampered := CanonicalRequest("POST", "/api/v1/x", BodySHA256([]byte("b")), 10, "n")
	require.True(t, SignatureMatches(secret, original, sig))
	require.False(t, SignatureMatches(secret, tampered, sig))
}

func TestSignatureMatches_Compare_NonHexSignature_ReturnsFalse(t *testing.T) {
	secret := SigningSecret("domk_live_abcdefghijklmnop")
	require.False(t, SignatureMatches(secret, "canon", "no-es-hex"))
}

func TestCanonicalTarget_Build_QueryPresent_KeepsRawQuery(t *testing.T) {
	u, err := url.Parse("/api/v1/search?q=a%20b&limit=1")
	require.NoError(t, err)
	require.Equal(t, "/api/v1/search?q=a%20b&limit=1", CanonicalTarget(u))
}

func TestCanonicalTarget_Build_NoQuery_OmitsQuestionMark(t *testing.T) {
	u, err := url.Parse("/api/v1/projects")
	require.NoError(t, err)
	require.Equal(t, "/api/v1/projects", CanonicalTarget(u))
}

func TestNewNonce_Generate_TwoCalls_DiffersAndFitsHeader(t *testing.T) {
	a, err := NewNonce()
	require.NoError(t, err)
	b, err := NewNonce()
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.LessOrEqual(t, len(a), MaxNonceLen)
	require.NotContains(t, a, ",")
	require.NotContains(t, a, "=")
}

func TestBuildSignatureHeader_Build_RoundTrips_ThroughParse(t *testing.T) {
	secret := SigningSecret("domk_live_abcdefghijklmnop")
	canonical := CanonicalRequest("PUT", "/api/v1/x", BodySHA256(nil), 99, "nn")
	h := BuildSignatureHeader("domk_live_abcdef", 99, "nn", Sign(secret, canonical))
	p, err := ParseSignatureHeader(h)
	require.NoError(t, err)
	require.Equal(t, "domk_live_abcdef", p.KeyPrefix)
	require.True(t, SignatureMatches(secret, canonical, p.Signature))
}
