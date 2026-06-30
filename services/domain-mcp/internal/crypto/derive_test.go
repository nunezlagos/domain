package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeriveKey_ShortInputDerivesTo32(t *testing.T) {
	k16 := bytes.Repeat([]byte{0xAB}, 16)
	out := DeriveKey(k16)
	require.Len(t, out, MasterKeySize)
	require.Equal(t, out, DeriveKey(k16), "derivación determinística")
	require.NotEqual(t, k16, out[:16])
}

func TestDeriveKey_Exact32Unchanged(t *testing.T) {
	k32 := bytes.Repeat([]byte{0xCD}, 32)
	require.Equal(t, k32, DeriveKey(k32), "32 bytes exactos pasan sin cambios")
}

func TestEncrypt_NonceUnique_1000x(t *testing.T) {
	key := make([]byte, MasterKeySize)
	_, _ = rand.Read(key)
	c, err := NewCipher(key)
	require.NoError(t, err)

	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		blob, err := c.Encrypt([]byte("same plaintext"))
		require.NoError(t, err)
		nonce := string(blob[1 : 1+NonceSize])
		require.False(t, seen[nonce], "nonce repetido en iteración %d", i)
		seen[nonce] = true
	}
}

func TestLoadKeyring_MultiVersionRoundTrip(t *testing.T) {
	k1 := make([]byte, MasterKeySize)
	k2 := make([]byte, MasterKeySize)
	_, _ = rand.Read(k1)
	_, _ = rand.Read(k2)


	c1, err := LoadKeyring("1:" + base64.StdEncoding.EncodeToString(k1))
	require.NoError(t, err)
	require.Equal(t, byte(1), c1.CurrentVersion())
	blobV1, err := c1.Encrypt([]byte("dato viejo"))
	require.NoError(t, err)


	c2, err := LoadKeyring("1:" + base64.StdEncoding.EncodeToString(k1) +
		",2:" + base64.StdEncoding.EncodeToString(k2))
	require.NoError(t, err)
	require.Equal(t, byte(2), c2.CurrentVersion())

	plain, err := c2.Decrypt(blobV1)
	require.NoError(t, err)
	require.Equal(t, "dato viejo", string(plain))

	blobV2, err := c2.Encrypt([]byte("dato nuevo"))
	require.NoError(t, err)
	require.Equal(t, byte(2), blobV2[0], "nuevos blobs llevan version 2")


	c3, err := LoadKeyring("2:" + base64.StdEncoding.EncodeToString(k2))
	require.NoError(t, err)
	_, err = c3.Decrypt(blobV1)
	require.ErrorIs(t, err, ErrUnknownKeyVersion)
}

func TestLoadKeyring_InvalidSpecs(t *testing.T) {
	cases := []string{
		"",
		"sin-dos-puntos",
		"0:" + base64.StdEncoding.EncodeToString(make([]byte, 32)),
		"1:no-base64!!!",
		"1:" + base64.StdEncoding.EncodeToString(make([]byte, 16)), // key corta
	}
	for _, spec := range cases {
		_, err := LoadKeyring(spec)
		require.Error(t, err, "spec %q debería fallar", spec)
	}
}
