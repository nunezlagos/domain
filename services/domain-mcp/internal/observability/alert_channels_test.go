package observability

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func noSleep(time.Duration) {}

func TestWebhookNotifier_Send_PostsJSONPayload(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wn := &WebhookNotifier{Client: srv.Client(), Retries: 2, sleep: noSleep}
	cfg := AlertConfig{Channel: "webhook", ChannelConfig: map[string]any{"url": srv.URL}}
	if err := wn.Send(context.Background(), sqlEvent("error"), cfg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(body, "SQL_ERROR") {
		t.Fatalf("payload should carry category, got %q", body)
	}
}

func TestWebhookNotifier_Send_RetriesUntilSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wn := &WebhookNotifier{Client: srv.Client(), Retries: 2, sleep: noSleep}
	cfg := AlertConfig{Channel: "webhook", ChannelConfig: map[string]any{"url": srv.URL}}
	if err := wn.Send(context.Background(), sqlEvent("error"), cfg); err != nil {
		t.Fatalf("Send should succeed on 3rd attempt: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
}

func TestWebhookNotifier_Send_MissingURL_Errors(t *testing.T) {
	wn := &WebhookNotifier{Client: http.DefaultClient, sleep: noSleep}
	err := wn.Send(context.Background(), sqlEvent("error"), AlertConfig{Channel: "webhook"})
	if err == nil {
		t.Fatal("missing url must error")
	}
}

func TestNtfyNotifier_Send_PostsToTopicWithTitle(t *testing.T) {
	var gotPath, gotTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotTitle = r.Header.Get("Title")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	nn := &NtfyNotifier{Client: srv.Client(), BaseURL: srv.URL, Retries: 1, sleep: noSleep}
	cfg := AlertConfig{Channel: "ntfy", ChannelConfig: map[string]any{"topic": "domain-alerts"}}
	if err := nn.Send(context.Background(), sqlEvent("critical"), cfg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotPath != "/domain-alerts" {
		t.Fatalf("path: got %q want /domain-alerts", gotPath)
	}
	if gotTitle == "" {
		t.Fatal("ntfy Title header should be set")
	}
}

func TestNtfyNotifier_Send_MissingTopic_Errors(t *testing.T) {
	nn := &NtfyNotifier{Client: http.DefaultClient, BaseURL: "https://ntfy.sh", sleep: noSleep}
	err := nn.Send(context.Background(), sqlEvent("error"), AlertConfig{Channel: "ntfy"})
	if err == nil {
		t.Fatal("missing topic must error")
	}
}
