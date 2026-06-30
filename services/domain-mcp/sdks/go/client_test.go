package domain

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testKey = "domk_test_abc123"

// newTestClient construye un Client apuntando al httptest server con la apikey
// fija. Devuelve el client y un cleanup.
func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New(WithBaseURL(srv.URL), WithAPIKey(testKey), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNew_RequiresAPIKey(t *testing.T) {
	t.Setenv("DOMAIN_API_KEY", "")
	_, err := New(WithBaseURL("http://x"))
	if err == nil {
		t.Fatal("expected error when no api key configured")
	}
}

func TestNew_PicksUpEnvAPIKey(t *testing.T) {
	t.Setenv("DOMAIN_API_KEY", "domk_env_xyz")
	c, err := New(WithBaseURL("http://x"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.apiKey != "domk_env_xyz" {
		t.Fatalf("expected env key, got %q", c.apiKey)
	}
}

func TestAuthHeaderAndContentType(t *testing.T) {
	var gotAuth, gotCT, gotUA, gotAccept string
	var gotMethod, gotPath string
	var gotBody map[string]any

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"11111111-1111-1111-1111-111111111111","name":"Demo","slug":"demo","description":"","created_at":"2026-01-01T00:00:00Z"}}`))
	})

	c := newTestClient(t, h)


	proj, err := c.Projects.Create(context.Background(), ProjectCreateInput{Name: "Demo", Slug: "demo"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if want := "Bearer " + testKey; gotAuth != want {
		t.Errorf("auth header = %q, want %q", gotAuth, want)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q", gotCT)
	}
	if !strings.HasPrefix(gotUA, "domain-sdk-go/") {
		t.Errorf("user-agent = %q", gotUA)
	}
	if gotAccept != "application/json" {
		t.Errorf("accept = %q", gotAccept)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
	if gotPath != "/api/v1/projects" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["name"] != "Demo" || gotBody["slug"] != "demo" {
		t.Errorf("body = %v", gotBody)
	}
	if proj.Slug != "demo" {
		t.Errorf("response slug = %q", proj.Slug)
	}
}

func TestCursorPaginationRoundtrip(t *testing.T) {
	var calls int
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		cursor := r.URL.Query().Get("cursor")
		switch cursor {
		case "":
			_, _ = w.Write([]byte(`{
				"data":[
					{"id":"id-1","project_id":"p","content":"a","created_at":"2026-01-01T00:00:00Z"},
					{"id":"id-2","project_id":"p","content":"b","created_at":"2026-01-01T00:00:00Z"}
				],
				"pagination":{"next_cursor":"cur-2","has_more":true,"limit":2}
			}`))
		case "cur-2":
			_, _ = w.Write([]byte(`{
				"data":[
					{"id":"id-3","project_id":"p","content":"c","created_at":"2026-01-01T00:00:00Z"}
				],
				"pagination":{"next_cursor":"","has_more":false,"limit":2}
			}`))
		default:
			t.Errorf("unexpected cursor %q", cursor)
			http.Error(w, "bad cursor", 500)
		}
	})

	c := newTestClient(t, h)


	items, pg, err := c.Observations.List(context.Background(), ListObservationsParams{Limit: 2})
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("page 1 items = %d", len(items))
	}
	if pg == nil || pg.NextCursor != "cur-2" || !pg.HasMore {
		t.Fatalf("page 1 pagination = %+v", pg)
	}

	items2, pg2, err := c.Observations.List(context.Background(), ListObservationsParams{Limit: 2, Cursor: pg.NextCursor})
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(items2) != 1 || items2[0].ID != "id-3" {
		t.Fatalf("page 2 items = %+v", items2)
	}
	if pg2 == nil || pg2.HasMore {
		t.Fatalf("page 2 should be terminal, got %+v", pg2)
	}


	calls = 0
	it := c.Observations.Iter(context.Background(), ListObservationsParams{Limit: 2})
	var collected []string
	for {
		obs, ok, err := it.Next(context.Background())
		if err != nil {
			t.Fatalf("iter: %v", err)
		}
		if !ok {
			break
		}
		collected = append(collected, obs.ID)
	}
	wantIDs := []string{"id-1", "id-2", "id-3"}
	if len(collected) != len(wantIDs) {
		t.Fatalf("iter collected %v", collected)
	}
	for i, id := range wantIDs {
		if collected[i] != id {
			t.Errorf("iter[%d] = %q want %q", i, collected[i], id)
		}
	}
	if calls != 2 {
		t.Errorf("expected 2 server calls during iter, got %d", calls)
	}
}

func TestAPIError_Unauthorized(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req-401")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"bad key","request_id":"req-401"}}`))
	})
	c := newTestClient(t, h)
	_, err := c.Projects.Get(context.Background(), "demo")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsUnauthorized(err) {
		t.Errorf("expected IsUnauthorized, got %v", err)
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected errors.Is ErrUnauthorized")
	}
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 || apiErr.Code != "unauthorized" || apiErr.RequestID != "req-401" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestAPIError_NotFound(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"missing"}}`))
	})
	c := newTestClient(t, h)
	_, err := c.Observations.Get(context.Background(), "x")
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func TestAPIError_Validation(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":{"code":"validation_failed","message":"bad","details":[{"field":"slug","code":"required","message":"required"}]}}`))
	})
	c := newTestClient(t, h)
	_, err := c.Projects.Create(context.Background(), ProjectCreateInput{Name: "x"})
	if !IsValidation(err) {
		t.Fatalf("expected IsValidation, got %v", err)
	}
	apiErr, _ := AsAPIError(err)
	if len(apiErr.Details) != 1 || apiErr.Details[0].Field != "slug" {
		t.Errorf("details = %+v", apiErr.Details)
	}
}

func TestAPIError_RateLimited_WithRetryAfter(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"slow down"}}`))
	})
	c := newTestClient(t, h)
	_, err := c.Search.Global(context.Background(), SearchParams{Query: "x"})
	if !IsRateLimited(err) {
		t.Fatalf("expected IsRateLimited, got %v", err)
	}
	apiErr, _ := AsAPIError(err)
	if apiErr.RetryAfter != 7 {
		t.Errorf("retry_after = %d, want 7", apiErr.RetryAfter)
	}
}

func TestAPIError_Conflict(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"slug_taken","message":"already"}}`))
	})
	c := newTestClient(t, h)
	_, err := c.Projects.Create(context.Background(), ProjectCreateInput{Slug: "x", Name: "x"})
	if !IsConflict(err) {
		t.Errorf("expected IsConflict, got %v", err)
	}
}

func TestContextCancel(t *testing.T) {

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	c := newTestClient(t, h)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Projects.Get(ctx, "demo")
	if err == nil {
		t.Fatal("expected error from cancelled ctx")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestDelete_204NoContent(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %q", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	c := newTestClient(t, h)
	if err := c.Observations.Delete(context.Background(), "id-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestQueryParamsSerialized(t *testing.T) {
	var gotQuery string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	c := newTestClient(t, h)
	_, err := c.Search.Global(context.Background(), SearchParams{
		Query:       "hello",
		Limit:       10,
		EntityTypes: []string{"observation", "knowledge"},
		Tags:        []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(gotQuery, "q=hello") ||
		!strings.Contains(gotQuery, "limit=10") ||
		!strings.Contains(gotQuery, "entity_type=observation%2Cknowledge") ||
		!strings.Contains(gotQuery, "tags=a%2Cb") {
		t.Errorf("query = %q", gotQuery)
	}
}
