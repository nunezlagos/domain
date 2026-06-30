package activity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type captureRecorder struct {
	mu     sync.Mutex
	events []Event
}

func (c *captureRecorder) Record(_ context.Context, e Event) (uuid.UUID, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	return uuid.New(), nil
}

func okPrincipal(orgID uuid.UUID, actorID uuid.UUID) PrincipalFn {
	return func(*http.Request) (uuid.UUID, *uuid.UUID, bool) {
		return orgID, &actorID, true
	}
}

func TestSummarize_Matrix(t *testing.T) {
	id := uuid.New()
	cases := []struct {
		method, path string
		wantAction   string
		wantEntity   string
		wantID       bool
	}{
		{"POST", "/api/v1/flows", "flow.created", "flow", false},
		{"PATCH", "/api/v1/flows/" + id.String(), "flow.updated", "flow", true},
		{"DELETE", "/api/v1/api-keys/" + id.String(), "api_key.deleted", "api_key", true},
		{"POST", "/api/v1/flows/" + id.String() + "/run", "flow.run", "flow", true},
		{"POST", "/api/v1/flow-runs/" + id.String() + "/pause", "flow_run.pause", "flow_run", true},
		{"POST", "/api/v1/flows/import", "flow.import", "flow", false},
		{"PUT", "/api/v1/policies/" + id.String(), "policy.updated", "policy", true},
	}
	for _, tc := range cases {
		action, entity, entityID, summary := Summarize(tc.method, tc.path)
		require.Equal(t, tc.wantAction, action, "%s %s", tc.method, tc.path)
		require.Equal(t, tc.wantEntity, entity)
		require.Equal(t, tc.wantID, entityID != nil)
		require.NotEmpty(t, summary)
	}


	action, _, _, _ := Summarize("POST", "/health")
	require.Empty(t, action)
}

func TestMiddleware_RecordsMutations(t *testing.T) {
	rec := &captureRecorder{}
	orgID, actorID := uuid.New(), uuid.New()
	mw := &HTTPMiddleware{Recorder: rec, Principal: okPrincipal(orgID, actorID)}

	h := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("POST", "/api/v1/flows", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	require.Len(t, rec.events, 1)
	require.Equal(t, "flow.created", rec.events[0].Action)
	require.Equal(t, orgID, rec.events[0].OrganizationID)
	require.Equal(t, actorID, *rec.events[0].ActorID)
	require.NotEmpty(t, rec.events[0].Summary)
}

func TestMiddleware_SkipsReadsErrorsAndAuth(t *testing.T) {
	rec := &captureRecorder{}
	mw := &HTTPMiddleware{Recorder: rec, Principal: okPrincipal(uuid.New(), uuid.New())}


	h := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/flows", nil))


	hFail := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	hFail.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v1/flows", nil))


	hAuth := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	hAuth.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v1/auth/request-otp", nil))

	require.Empty(t, rec.events)
}

func TestMiddleware_NoPrincipal_Skips(t *testing.T) {
	rec := &captureRecorder{}
	mw := &HTTPMiddleware{Recorder: rec,
		Principal: func(*http.Request) (uuid.UUID, *uuid.UUID, bool) { return uuid.Nil, nil, false }}
	h := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v1/flows", nil))
	require.Empty(t, rec.events)
}

// El wrapper preserva http.Flusher (SSE no debe romperse).
func TestStatusWriter_PreservesFlusher(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}
	var w http.ResponseWriter = sw
	_, ok := w.(http.Flusher)
	require.True(t, ok)
}

func TestSingularize(t *testing.T) {
	cases := map[string]string{
		"flows": "flow", "api-keys": "api_key", "flow-runs": "flow_run",
		"policies": "policy", "organizations": "organization", "dlq": "dlq",
	}
	for in, want := range cases {
		require.Equal(t, want, singularize(in), in)
	}
}
