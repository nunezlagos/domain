// Package etag — helpers para HU-13.7 HTTP caching con ETag + Last-Modified.
//
// El ETag se computa como sha256_16(updated_at_unix_nano + ":" + id) — barato y
// no requiere serializar el body. Last-Modified es updated_at en RFC1123 GMT.
package etag

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Compute returns the quoted ETag for the given (id, updatedAt) pair.
// Output already includes surrounding quotes per RFC 7232 (ej. `"abc123def456"`).
func Compute(id string, updatedAt time.Time) string {
	h := sha256.New()
	h.Write([]byte(strconv.FormatInt(updatedAt.UnixNano(), 10)))
	h.Write([]byte{':'})
	h.Write([]byte(id))
	sum := hex.EncodeToString(h.Sum(nil))
	return `"` + sum[:16] + `"`
}

// LastModified returns the RFC1123 GMT string for the given time.
func LastModified(updatedAt time.Time) string {
	return updatedAt.UTC().Format(http.TimeFormat)
}

// SetHeaders escribe ETag, Last-Modified y Cache-Control.
// cacheControl: ej. "private, max-age=60" o "private, no-store"
func SetHeaders(w http.ResponseWriter, id string, updatedAt time.Time, cacheControl string) string {
	tag := Compute(id, updatedAt)
	w.Header().Set("ETag", tag)
	w.Header().Set("Last-Modified", LastModified(updatedAt))
	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
	return tag
}

// IsNotModified evalúa If-None-Match / If-Modified-Since.
// Retorna true si el cliente ya tiene la versión actual y se debe responder 304.
func IsNotModified(r *http.Request, etag string, updatedAt time.Time) bool {
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		for _, t := range parseList(inm) {
			if t == etag || t == "*" {
				return true
			}
		}
		return false
	}
	if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		t, err := http.ParseTime(ims)
		if err == nil && !updatedAt.After(t) {
			return true
		}
	}
	return false
}

// MatchesIfMatch evalúa If-Match para optimistic concurrency (PATCH/PUT/DELETE).
// Retorna:
// - hasHeader=false si no hay If-Match (no aplica precondition)
// - hasHeader=true, matched=true si el ETag actual coincide
// - hasHeader=true, matched=false → responder 412 Precondition Failed
func MatchesIfMatch(r *http.Request, currentETag string) (hasHeader, matched bool) {
	im := r.Header.Get("If-Match")
	if im == "" {
		return false, false
	}
	for _, t := range parseList(im) {
		if t == currentETag || t == "*" {
			return true, true
		}
	}
	return true, false
}

func parseList(h string) []string {
	parts := strings.Split(h, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
