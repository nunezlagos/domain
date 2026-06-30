// Package crypto — issue-02.3 secrets-encryption AES-256-GCM.
//
// Envelope encryption: master key (32 bytes) cifra valores. Cada valor cifrado
// lleva su nonce único (12 bytes) + key_version inline:
//   layout: [version:1byte][nonce:12bytes][ciphertext+gcm_tag:N bytes]
//
// Master key se carga desde env DOMAIN_MASTER_KEY (base64 32 bytes) o KMS/Vault
// en producción. Rotation: bump version, re-encrypt en background job.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

const (

	MasterKeySize = 32

	NonceSize = 12

	headerSize = 1 + NonceSize
)

var (

	ErrInvalidKeySize = errors.New("master key must be exactly 32 bytes")

	ErrCiphertextTooShort = errors.New("ciphertext too short")

	ErrUnknownKeyVersion = errors.New("unknown key version")
)

// Cipher AES-256-GCM con keyring multi-version para rotation.
type Cipher struct {

	keyring map[byte]cipher.AEAD

	current byte
}

// NewCipher crea Cipher con master key version 1.
func NewCipher(masterKey []byte) (*Cipher, error) {
	if len(masterKey) != MasterKeySize {
		return nil, ErrInvalidKeySize
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("aes new: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm new: %w", err)
	}
	c := &Cipher{keyring: map[byte]cipher.AEAD{1: aead}, current: 1}
	return c, nil
}

// LoadFromBase64 helper para boot desde env DOMAIN_MASTER_KEY.
func LoadFromBase64(b64 string) (*Cipher, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode master key: %w", err)
	}
	return NewCipher(raw)
}

// AddKey agrega nueva version al keyring (para rotation).
// La nueva pasa a ser current; la anterior queda solo para Decrypt.
func (c *Cipher) AddKey(version byte, masterKey []byte) error {
	if len(masterKey) != MasterKeySize {
		return ErrInvalidKeySize
	}
	if _, exists := c.keyring[version]; exists {
		return fmt.Errorf("version %d already in keyring", version)
	}
	if version <= c.current {
		return fmt.Errorf("new version must be > current (%d)", c.current)
	}
	block, _ := aes.NewCipher(masterKey)
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("gcm new: %w", err)
	}
	c.keyring[version] = aead
	c.current = version
	return nil
}

// CurrentVersion retorna version actual (usada para Encrypt).
func (c *Cipher) CurrentVersion() byte { return c.current }

// Encrypt cifra plaintext con current version. Output: [version|nonce|ciphertext+tag].
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	aead := c.keyring[c.current]
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce read: %w", err)
	}
	out := make([]byte, 0, headerSize+len(plaintext)+aead.Overhead())
	out = append(out, c.current)
	out = append(out, nonce...)
	out = aead.Seal(out, nonce, plaintext, nil)
	return out, nil
}

// Decrypt descifra blob respetando version inline.
func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < headerSize {
		return nil, ErrCiphertextTooShort
	}
	version := ciphertext[0]
	nonce := ciphertext[1:headerSize]
	body := ciphertext[headerSize:]

	aead, ok := c.keyring[version]
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrUnknownKeyVersion, version)
	}
	plaintext, err := aead.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open (tampered or wrong key): %w", err)
	}
	return plaintext, nil
}

// GenerateMasterKey crea master key cripto-segura (para setup inicial).
func GenerateMasterKey() ([]byte, error) {
	k := make([]byte, MasterKeySize)
	if _, err := rand.Read(k); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}
	return k, nil
}

// MasterKeyBase64 helper para print master key generada al setup.
func MasterKeyBase64(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}
