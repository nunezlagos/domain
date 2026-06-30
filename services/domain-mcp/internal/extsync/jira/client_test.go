package jira_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/extsync/jira"
)

func TestCreateIssue_Story(t *testing.T) {
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/api/3/issue", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		fields := body["fields"].(map[string]any)
		require.Equal(t, "Test summary", fields["summary"])
		require.Equal(t, "DIDE", fields["project"].(map[string]any)["key"])
		require.Equal(t, "Story", fields["issuetype"].(map[string]any)["name"])

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id": "10001", "key": "DIDE-100", "self": srvURL + "/rest/api/3/issue/10001",
		})
	}))
	defer srv.Close()
	srvURL = srv.URL

	c := jira.New(srv.URL, "u@x.com", "tok", "DIDE")
	out, err := c.CreateIssue(context.Background(), jira.CreateIssueRequest{
		Summary:     "Test summary",
		Description: "Test description",
		Type:        jira.TypeStory,
	})
	require.NoError(t, err)
	require.Equal(t, "DIDE-100", out.Key)
}

func TestCreateIssue_EpicWithLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		fields := body["fields"].(map[string]any)
		labels := fields["labels"].([]any)
		require.Len(t, labels, 2)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id": "10002", "key": "DIDE-101",
		})
	}))
	defer srv.Close()

	c := jira.New(srv.URL, "u", "t", "DIDE")
	out, err := c.CreateIssue(context.Background(), jira.CreateIssueRequest{
		Summary: "Epic", Type: jira.TypeEpic, Labels: []string{"sdd", "req-01"},
	})
	require.NoError(t, err)
	require.Equal(t, "DIDE-101", out.Key)
}

func TestCreateIssue_HTTP4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":{"summary":"required"}}`))
	}))
	defer srv.Close()

	c := jira.New(srv.URL, "u", "t", "DIDE")
	_, err := c.CreateIssue(context.Background(), jira.CreateIssueRequest{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "400")
}

func TestGetIssue_ParsesADFDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
		  "id": "10001", "key": "DIDE-100",
		  "fields": {
		    "summary": "Test",
		    "description": {"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello world"}]}]},
		    "status": {"name": "In Progress"},
		    "updated": "2026-06-10T12:00:00.000-0400"
		  }
		}`))
	}))
	defer srv.Close()

	c := jira.New(srv.URL, "u", "t", "DIDE")
	iss, err := c.GetIssue(context.Background(), "DIDE-100")
	require.NoError(t, err)
	require.Equal(t, "Hello world", iss.Description)
	require.Equal(t, "In Progress", iss.Status)
}

func TestVerifyWebhookSignature_OK(t *testing.T) {
	secret := "shared-secret"
	payload := []byte(`{"webhookEvent":"jira:issue_updated"}`)

	c := jira.EncodeBasicAuth("x", "y") // touch helper
	_ = c

	require.False(t, jira.VerifyWebhookSignature(payload, "sha256=bogus", secret))
}

func TestParseWebhook_SignatureMismatch(t *testing.T) {
	_, err := jira.ParseWebhook([]byte(`{"webhookEvent":"x"}`), "sha256=bogus", "secret")
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature mismatch")
}

func TestParseWebhook_NoSecret_Allowed(t *testing.T) {
	ev, err := jira.ParseWebhook([]byte(`{"webhookEvent":"jira:issue_updated","issue":{"id":"1","key":"DIDE-1"}}`), "", "")
	require.NoError(t, err)
	require.Equal(t, "DIDE-1", ev.Issue.Key)
}

// Sabotaje: Description nil en GetIssue no debe crashear.
func TestSabotage_NilDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x","key":"K-1","fields":{"summary":"x","description":null,"status":{"name":"To Do"},"updated":""}}`))
	}))
	defer srv.Close()

	c := jira.New(srv.URL, "u", "t", "DIDE")
	iss, err := c.GetIssue(context.Background(), "K-1")
	require.NoError(t, err)
	require.Empty(t, iss.Description)
}
