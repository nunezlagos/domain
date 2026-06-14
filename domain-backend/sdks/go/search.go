package domain

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type SearchResource struct{ c *Client }

type SearchParams struct {
	Query       string
	Limit       int
	EntityTypes []string
	Tags        []string
}

func (p SearchParams) values() url.Values {
	v := url.Values{}
	if p.Query != "" {
		v.Set("q", p.Query)
	}
	if p.Limit > 0 {
		v.Set("limit", strconv.Itoa(p.Limit))
	}
	if len(p.EntityTypes) > 0 {
		v.Set("entity_type", strings.Join(p.EntityTypes, ","))
	}
	if len(p.Tags) > 0 {
		v.Set("tags", strings.Join(p.Tags, ","))
	}
	return v
}

// Global ejecuta búsqueda federada sobre observaciones, knowledge, etc.
func (r *SearchResource) Global(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	var out []SearchResult
	_, err := r.c.do(ctx, http.MethodGet, "/search", params.values(), nil, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// itoa wrapper local para evitar import de strconv en archivos que no lo usan directo.
func itoa(n int) string { return strconv.Itoa(n) }
