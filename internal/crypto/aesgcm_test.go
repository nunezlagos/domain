// HU-02.3 secrets-encryption unit tests.

package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// Helper: master key determinista para tests.
func testKey() []byte {
	k := make([]byte, MasterKeySize)
	for i := range k {
		k[i] = byte(i + 1)
	}
	return k
}

func testKeyAlt() []byte {
	k := make([]byte, MasterKeySize)
	for i := range k {
		k[i] = byte(255 - i)
	}
	return k
}

// Escenario 1: round-trip encrypt → decrypt.
func TestCipher_RoundTrip(t *testing.T) {
	c, err := NewCipher(testKey())
	require.NoError(t, err)

	plaintext := []byte("super secret value 🔒")
	ciphertext, err := c.Encrypt(plaintext)
	require.NoError(t, err)
	require.NotEqual(t, plaintext, ciphertext)

	got, err := c.Decrypt(ciphertext)
	require.NoError(t, err)
	require.Equal(t, plaintext, got)
}

// Escenario 2: empty plaintext OK.
func TestCipher_EmptyPlaintext(t *testing.T) {
	c, _ := NewCipher(testKey())
	ct, err := c.Encrypt([]byte{})
	require.NoError(t, err)
	pt, err := c.Decrypt(ct)
	require.NoError(t, err)
	require.Empty(t, pt)
}

// Escenario 3: nonce único por encrypt (no determinístico).
func TestCipher_Encrypt_NonceUnique(t *testing.T) {
	c, _ := NewCipher(testKey())
	pt := []byte("same input")
	a, _ := c.Encrypt(pt)
	b, _ := c.Encrypt(pt)
	require.False(t, bytes.Equal(a, b), "ciphertexts must differ (random nonce)")
}

// Escenario 4: master key wrong size rechazada.
func TestCipher_InvalidKeySize(t *testing.T) {
	_, err := NewCipher(make([]byte, 16))
	require.ErrorIs(t, err, ErrInvalidKeySize)
	_, err = NewCipher(make([]byte, 64))
	require.ErrorIs(t, err, ErrInvalidKeySize)
}

// Escenario 5: ciphertext muy corto rechazado.
func TestCipher_CiphertextTooShort(t *testing.T) {
	c, _ := NewCipher(testKey())
	_, err := c.Decrypt([]byte{0x01, 0x02})
	require.ErrorIs(t, err, ErrCiphertextTooShort)
}

// Escenario 6: rotation — encrypt con v1, agregar v2 como current, decrypt v1 sigue OK.
func TestCipher_KeyRotation(t *testing.T) {
	c, _ := NewCipher(testKey())
	require.Equal(t, byte(1), c.CurrentVersion())

	v1Ciphertext, _ := c.Encrypt([]byte("old secret"))

	require.NoError(t, c.AddKey(2, testKeyAlt()))
	require.Equal(t, byte(2), c.CurrentVersion())

	// v1 sigue decifrando
	pt, err := c.Decrypt(v1Ciphertext)
	require.NoError(t, err)
	require.Equal(t, []byte("old secret"), pt)

	// Nuevos encrypts usan v2
	v2Ciphertext, _ := c.Encrypt([]byte("new secret"))
	require.Equal(t, byte(2), v2Ciphertext[0], "new ciphertext uses v2")

	pt, err = c.Decrypt(v2Ciphertext)
	require.NoError(t, err)
	require.Equal(t, []byte("new secret"), pt)
}

// Escenario 7: version desconocida en ciphertext.
func TestCipher_UnknownVersion(t *testing.T) {
	c, _ := NewCipher(testKey())
	ct, _ := c.Encrypt([]byte("x"))
	ct[0] = 99 // version inválida
	_, err := c.Decrypt(ct)
	require.ErrorIs(t, err, ErrUnknownKeyVersion)
}

// Escenario 8: rotation NO permite mismo version dos veces.
func TestCipher_AddKey_DuplicateVersion(t *testing.T) {
	c, _ := NewCipher(testKey())
	err := c.AddKey(1, testKeyAlt())
	require.Error(t, err, "v1 ya existe")
}

// Escenario 9: rotation NO permite version <= current.
func TestCipher_AddKey_LowerVersion(t *testing.T) {
	c, _ := NewCipher(testKey())
	require.NoError(t, c.AddKey(2, testKeyAlt()))
	err := c.AddKey(2, testKeyAlt())
	require.Error(t, err)
}

// Escenario 10: LoadFromBase64.
func TestCipher_LoadFromBase64(t *testing.T) {
	key := testKey()
	b64 := MasterKeyBase64(key)
	c, err := LoadFromBase64(b64)
	require.NoError(t, err)
	pt, err := c.Decrypt(mustEncrypt(t, key, []byte("hello")))
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), pt)
	_ = c
}

func mustEncrypt(t *testing.T, key []byte, pt []byte) []byte {
	t.Helper()
	c, _ := NewCipher(key)
	out, err := c.Encrypt(pt)
	require.NoError(t, err)
	return out
}

// Escenario 11: GenerateMasterKey produce keys aleatorios distintos.
func TestGenerateMasterKey_Random(t *testing.T) {
	k1, err := GenerateMasterKey()
	require.NoError(t, err)
	require.Len(t, k1, MasterKeySize)
	k2, err := GenerateMasterKey()
	require.NoError(t, err)
	require.False(t, bytes.Equal(k1, k2), "two generated keys must differ")
}

// Sabotaje: tampered ciphertext (flip bit) → Decrypt falla.
func TestSabotage_TamperedCiphertext_Rejected(t *testing.T) {
	c, _ := NewCipher(testKey())
	ct, _ := c.Encrypt([]byte("untouched"))

	// Flip 1 bit en el ciphertext body (no en version/nonce header)
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[len(tampered)-1] ^= 0x01

	_, err := c.Decrypt(tampered)
	require.Error(t, err, "GCM tag mismatch debe rechazar")
}

// Sabotaje: tampered nonce → Decrypt falla.
func TestSabotage_TamperedNonce_Rejected(t *testing.T) {
	c, _ := NewCipher(testKey())
	ct, _ := c.Encrypt([]byte("nonce-test"))
	ct[1] ^= 0xFF // flip primer byte del nonce

	_, err := c.Decrypt(ct)
	require.Error(t, err, "wrong nonce debe rechazar GCM auth")
}

// Sabotaje: decrypt con key distinta.
func TestSabotage_WrongKey_Rejected(t *testing.T) {
	a, _ := NewCipher(testKey())
	b, _ := NewCipher(testKeyAlt())

	ct, _ := a.Encrypt([]byte("only-A-can-decrypt"))
	_, err := b.Decrypt(ct)
	require.Error(t, err, "B con otra key NO debe poder descifrar A")
}
