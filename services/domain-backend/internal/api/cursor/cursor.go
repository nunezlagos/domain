// Package cursor — issue-13.6 cursor-based pagination opaque.
//
// El cursor encodea {last_id, last_sort_value, filters_hash, sort_dir} en base64url JSON
// y se valida en cada page request (filters_hash debe matchear).
package cursor

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalid          = errors.New("invalid_cursor")
	ErrFiltersMismatch  = errors.New("cursor_filters_mismatch")
	ErrSortMismatch     = errors.New("cursor_sort_mismatch")
)

// Cursor opaque para paginación. Se serializa como base64url(JSON).
type Cursor struct {
	LastID        string    `json:"id"`
	LastSortValue time.Time `json:"sv"`
	FiltersHash   string    `json:"fh"`
	SortDir       string    `json:"sd"` // "asc" | "desc"
}

// Encode serializa a base64url-safe sin padding.
func (c Cursor) Encode() string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Decode descodifica un cursor opaque. Errores comunes:
// - ErrInvalid si base64 o JSON inválidos
// - ErrFiltersMismatch si filtersHash no coincide
// - ErrSortMismatch si sortDir no coincide
func Decode(raw, expectedFiltersHash, expectedSortDir string) (*Cursor, error) {
	if raw == "" {
		return nil, ErrInvalid
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalid
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, ErrInvalid
	}
	if c.FiltersHash != expectedFiltersHash {
		return nil, ErrFiltersMismatch
	}
	if c.SortDir != expectedSortDir {
		return nil, ErrSortMismatch
	}
	return &c, nil
}

// HashFilters genera un hash determinístico para un set de filtros key→value.
// Se usa para detectar que el cliente reutilizó un cursor con filtros distintos.
func HashFilters(filters map[string]string) string {
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{'='})
		h.Write([]byte(filters[k]))
		h.Write([]byte{';'})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// NormalizeSort acepta "asc" | "desc" | "" → defaultDir ("desc").
func NormalizeSort(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "desc":
		return "desc", nil
	case "asc":
		return "asc", nil
	}
	return "", ErrInvalid
}

// MaxLegacyOffset cap para offset legacy (issue-13.6 escenario 6).
const MaxLegacyOffset = 10000

// PageMeta es la shape estándar de pagination en responses.
type PageMeta struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit"`
}
