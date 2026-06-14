// Package enrollment — issue-37.1 self-enroll-shared-token.
//
// Genera y valida tokens de enrollment compartidos por org. Reemplaza
// temporalmente al flujo OTP+email mientras el cloud no tenga SMTP.
//
// Formato del token: "et_<base64url 32 bytes>" ≈ 46 chars.
// Storage: bcrypt hash + prefix indexable.
package enrollment

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// TokenPrefix marca el formato del token plaintext.
	TokenPrefix = "et_"
	// PrefixLen chars visibles para indexing en DB (et_ + 16 random).
	PrefixLen = 19
	// SecretBytes random del token (32 → 43 chars base64url).
	SecretBytes = 32
	// BcryptCost cost del hash. 12 = ~250ms en hardware moderno (mismo que api_keys).
	BcryptCost = 12
)

// ErrInvalidFormat se devuelve cuando el plaintext no respeta "et_<chars>".
var ErrInvalidFormat = errors.New("invalid enrollment token format")

// GeneratePlaintext crea token plaintext + prefix indexable + hash bcrypt.
// El plaintext se devuelve UNA sola vez al caller y no se persiste.
func GeneratePlaintext() (plaintext, prefix string, hash []byte, err error) {
	secret := make([]byte, SecretBytes)
	if _, err = rand.Read(secret); err != nil {
		return "", "", nil, fmt.Errorf("rand: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(secret)
	plaintext = TokenPrefix + encoded
	prefix = plaintext[:PrefixLen]
	hash, err = bcrypt.GenerateFromPassword([]byte(plaintext), BcryptCost)
	if err != nil {
		return "", "", nil, fmt.Errorf("bcrypt: %w", err)
	}
	return plaintext, prefix, hash, nil
}

// ParsePrefix extrae el prefix (et_ + 16 chars) del plaintext.
// Si el formato es inválido, devuelve ErrInvalidFormat.
func ParsePrefix(plaintext string) (string, error) {
	if !strings.HasPrefix(plaintext, TokenPrefix) {
		return "", ErrInvalidFormat
	}
	if len(plaintext) < PrefixLen {
		return "", ErrInvalidFormat
	}
	return plaintext[:PrefixLen], nil
}

// VerifyHash compara plaintext contra el hash almacenado.
// Retorna nil si match, error si no.
func VerifyHash(plaintext string, storedHash []byte) error {
	if err := bcrypt.CompareHashAndPassword(storedHash, []byte(plaintext)); err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	return nil
}

// dummyBcryptHash es un hash bcrypt válido pre-computado para correr en
// constant-time cuando NO hay candidatos en DB (anti-enumeration). El
// plaintext correspondiente es irrelevante; lo único que importa es que
// bcrypt.CompareHashAndPassword tome el mismo tiempo que un hash real.
var dummyBcryptHash = []byte("$2a$12$" + "0000000000000000000000" + "0000000000000000000000000000")
