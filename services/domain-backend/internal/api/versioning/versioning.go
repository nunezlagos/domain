// Package versioning — issue-13.8 API versioning policy.
//
// Mantiene un catálogo de versiones (v1, v2, ...) con su estado:
//   - active: en producción, soportada sin warnings
//   - deprecated: anuncio público, headers Deprecation+Sunset+Link
//   - sunset: pasó la fecha de sunset → 410 Gone
//
// El middleware enriquece cada response con los headers correspondientes
// según el prefijo /api/vN/ de la URL.
package versioning

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// State del versionado de una versión específica.
type State string

const (
	StateActive     State = "active"
	StateDeprecated State = "deprecated"
	StateSunset     State = "sunset"
)

// Version describe una versión del API.
type Version struct {
	Slug              string    // "v1", "v2"
	State             State
	DeprecatedAt      time.Time // cuándo se anunció deprecation (cero si no aplica)
	SunsetAt          time.Time // cuándo se corta el soporte (cero si no aplica)
	MigrationDocsURL  string    // URL a docs de migración (rel="deprecation")
}

// Catalog catálogo de versions del proyecto. Default tiene v1 active.
type Catalog struct {
	versions map[string]Version
	current  string
}

// NewCatalog crea un catálogo. La current version se reporta en /api/version.
func NewCatalog(current string, versions ...Version) *Catalog {
	c := &Catalog{
		versions: make(map[string]Version, len(versions)),
		current:  current,
	}
	for _, v := range versions {
		c.versions[v.Slug] = v
	}
	if _, ok := c.versions[current]; !ok && current != "" {
		c.versions[current] = Version{Slug: current, State: StateActive}
	}
	return c
}

// Get retorna la versión por slug (o nil si no existe).
func (c *Catalog) Get(slug string) *Version {
	v, ok := c.versions[slug]
	if !ok {
		return nil
	}
	return &v
}

// Current retorna el slug de la versión current.
func (c *Catalog) Current() string { return c.current }

// All retorna todas las versiones registradas.
func (c *Catalog) All() []Version {
	out := make([]Version, 0, len(c.versions))
	for _, v := range c.versions {
		out = append(out, v)
	}
	return out
}

// extractVersionSlug toma "/api/v1/observations/x" → "v1".
// Retorna "" si el path no matchea el patrón.
func extractVersionSlug(path string) string {
	if !strings.HasPrefix(path, "/api/v") {
		return ""
	}
	rest := path[len("/api/"):]
	end := strings.IndexByte(rest, '/')
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// Middleware enriquece responses con headers de versioning y bloquea sunset.
//
// Para /api/v1/* deprecated:
//   - Deprecation: @<unix> (RFC 8594)
//   - Sunset: <RFC1123> (RFC 8594)
//   - Link: <docs_url>; rel="deprecation"
//
// Para versiones en sunset (fecha pasada) → 410 Gone.
func (c *Catalog) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug := extractVersionSlug(r.URL.Path)
		if slug == "" {
			next.ServeHTTP(w, r)
			return
		}
		v := c.Get(slug)
		if v == nil {
			next.ServeHTTP(w, r)
			return
		}
		// Sunset enforcement
		if v.State == StateSunset || (!v.SunsetAt.IsZero() && time.Now().After(v.SunsetAt)) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if v.MigrationDocsURL != "" {
				w.Header().Set("Link", "<"+v.MigrationDocsURL+">; rel=\"deprecation\"")
			}
			w.WriteHeader(http.StatusGone)
			_, _ = w.Write([]byte(`{"error":{"code":"version_sunset","message":"API version no longer supported"}}`))
			return
		}
		// Deprecated headers
		if v.State == StateDeprecated {
			if !v.DeprecatedAt.IsZero() {
				w.Header().Set("Deprecation", "@"+strconv.FormatInt(v.DeprecatedAt.Unix(), 10))
			}
			if !v.SunsetAt.IsZero() {
				w.Header().Set("Sunset", v.SunsetAt.UTC().Format(http.TimeFormat))
			}
			if v.MigrationDocsURL != "" {
				w.Header().Set("Link", "<"+v.MigrationDocsURL+">; rel=\"deprecation\"")
			}
		}
		next.ServeHTTP(w, r)
	})
}

// VersionInfoHandler devuelve GET /api/version con el catálogo serializable.
func (c *Catalog) VersionInfoHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	type entry struct {
		Slug         string `json:"slug"`
		State        string `json:"state"`
		DeprecatedAt string `json:"deprecated_at,omitempty"`
		SunsetAt     string `json:"sunset_at,omitempty"`
	}
	type body struct {
		Current string  `json:"current"`
		All     []entry `json:"versions"`
	}
	out := body{Current: c.current}
	for _, v := range c.All() {
		e := entry{Slug: v.Slug, State: string(v.State)}
		if !v.DeprecatedAt.IsZero() {
			e.DeprecatedAt = v.DeprecatedAt.UTC().Format(time.RFC3339)
		}
		if !v.SunsetAt.IsZero() {
			e.SunsetAt = v.SunsetAt.UTC().Format(time.RFC3339)
		}
		out.All = append(out.All, e)
	}
	_ = json.NewEncoder(w).Encode(out)
}
