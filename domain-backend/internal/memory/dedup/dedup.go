// Package dedup — issue-03.6 hash SHA-256 normalizado para dedup de observations.
//
// Hash input es la concatenación de campos identificantes con separador NUL.
// Pre-normalización del content: lowercasing + whitespace collapse + trim.
// Esto convierte "Fix Login" y "fix  login" en el mismo fingerprint.
package dedup

import (
	"crypto/sha256"
	"strings"

	"github.com/google/uuid"
)

// FingerprintInput campos a hashear para detectar duplicados.
type FingerprintInput struct {
	ProjectID       uuid.UUID
	ObservationType string
	Title           string
	Content         string
}

// Hash devuelve el SHA-256 (32 bytes) del fingerprint normalizado.
// Normalización aplicada a Title + Content:
//   1. lowercasing
//   2. colapso de whitespace runs en un espacio único
//   3. trim
func Hash(in FingerprintInput) []byte {
	h := sha256.New()
	h.Write([]byte(in.ProjectID.String()))
	h.Write([]byte{0})
	h.Write([]byte(strings.ToLower(in.ObservationType)))
	h.Write([]byte{0})
	h.Write([]byte(normalize(in.Title)))
	h.Write([]byte{0})
	h.Write([]byte(normalize(in.Content)))
	return h.Sum(nil)
}

func normalize(s string) string {
	s = strings.ToLower(s)
	return strings.Join(strings.Fields(s), " ")
}
