// Package apikey — HU-02.1 api-key-auth.
//
// Formato: domk_{env}_{rand32_b64url}.
// Storage: bcrypt(key) + key_prefix visible (primeros N chars).
// Lookup: O(N) en candidatos por prefix (índice partial revoked_at IS NULL).
package apikey

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// PrefixLen chars visibles del API key para indexing y display.
	PrefixLen = 16
	// SecretBytes bytes random del secret part (32 → 43 chars base64url).
	SecretBytes = 32
	// BcryptCost cost del hash. 12 = ~250ms en hardware moderno.
	BcryptCost = 12
)

var (
	// ErrInvalidFormat key string no respeta `domk_{env}_{secret}`.
	ErrInvalidFormat = errors.New("invalid api key format")
	// ErrInvalidEnv prefix env desconocido.
	ErrInvalidEnv = errors.New("invalid api key env (expected live/test/dev)")
)

// validEnvs aceptados en el prefix.
var validEnvs = map[string]bool{"live": true, "test": true, "dev": true}

// GeneratePlaintext crea plaintext + prefix SIN hash (rápido). Útil para tests
// de uniqueness y para flows donde el hash se computa async.
func GeneratePlaintext(env string) (plaintext, prefix string, err error) {
	if !validEnvs[env] {
		return "", "", ErrInvalidEnv
	}
	secret := make([]byte, SecretBytes)
	if _, err = rand.Read(secret); err != nil {
		return "", "", fmt.Errorf("rand: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(secret)
	plaintext = "domk_" + env + "_" + encoded
	prefix = plaintext[:PrefixLen]
	return plaintext, prefix, nil
}

// Generate crea nueva API key con bcrypt hash (cost configurado).
// O(250ms+) por bcrypt. El plaintext SE DEVUELVE UNA SOLA VEZ al caller.
func Generate(env string) (plaintext, prefix string, hash []byte, err error) {
	plaintext, prefix, err = GeneratePlaintext(env)
	if err != nil {
		return "", "", nil, err
	}
	hash, err = bcrypt.GenerateFromPassword([]byte(plaintext), BcryptCost)
	if err != nil {
		return "", "", nil, fmt.Errorf("bcrypt: %w", err)
	}
	return plaintext, prefix, hash, nil
}

// Verify checks plaintext key against stored bcrypt hash.
// Returns nil si match, error si no.
func Verify(plaintext string, storedHash []byte) error {
	if !strings.HasPrefix(plaintext, "domk_") {
		return ErrInvalidFormat
	}
	if err := bcrypt.CompareHashAndPassword(storedHash, []byte(plaintext)); err != nil {
		return fmt.Errorf("api key verify: %w", err)
	}
	return nil
}

// ParsePrefix extrae prefix (primeros 16 chars) de plaintext.
// Útil para lookup en DB sin tocar bcrypt (índice por prefix).
func ParsePrefix(plaintext string) (string, error) {
	if len(plaintext) < PrefixLen {
		return "", ErrInvalidFormat
	}
	if !strings.HasPrefix(plaintext, "domk_") {
		return "", ErrInvalidFormat
	}
	parts := strings.SplitN(plaintext, "_", 3)
	if len(parts) != 3 || parts[0] != "domk" {
		return "", ErrInvalidFormat
	}
	if !validEnvs[parts[1]] {
		return "", ErrInvalidEnv
	}
	return plaintext[:PrefixLen], nil
}

// IsAPIKeyFormat checks superficial format (sin bcrypt) — para fail-fast en middleware.
func IsAPIKeyFormat(s string) bool {
	_, err := ParsePrefix(s)
	return err == nil
}
