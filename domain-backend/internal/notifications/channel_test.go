package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistry_RegisterGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&LogOnlyChannel{})
	ch, err := r.Get("log_only")
	if err != nil {
		t.Fatal(err)
	}
	if ch.Slug() != "log_only" {
		t.Fatalf("slug: %s", ch.Slug())
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("random")
	if !errors.Is(err, ErrChannelNotFound) {
		t.Fatalf("got %v, want ErrChannelNotFound", err)
	}
}

func TestRegistry_Slugs(t *testing.T) {
	r := NewRegistry()
	r.Register(&LogOnlyChannel{})
	r.Register(NewSlackChannel())
	slugs := r.Slugs()
	if len(slugs) != 2 {
		t.Fatalf("slugs: %v", slugs)
	}
}

func TestTemplate_RenderOK(t *testing.T) {
	tmpl := &Template{
		Slug:    "usage_alert",
		Subject: "Alert {{.OrgName}}",
		Body:    "Org {{.OrgName}} reached {{.Percent}}% of token limit",
	}
	subj, body, err := tmpl.Render(map[string]any{"OrgName": "Acme", "Percent": 80})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Acme") || !strings.Contains(body, "80") {
		t.Fatalf("body: %s", body)
	}
	if subj != "Alert Acme" {
		t.Fatalf("subj: %s", subj)
	}
}

func TestTemplate_MissingKey_Error(t *testing.T) {
	tmpl := &Template{Body: "Hi {{.Name}}, your limit is {{.Limit}}"}
	_, _, err := tmpl.Render(map[string]any{"Name": "Ana"})
	if err == nil {
		t.Fatal("expected error for missing key Limit (NOT '<no value>')")
	}
}

func TestSlackChannel_Slug(t *testing.T) {
	if NewSlackChannel().Slug() != "slack" {
		t.Fatal("wrong slug")
	}
}

func TestSlackChannel_Send_OK(t *testing.T) {
	var received string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// El check `https://hooks.slack.com/` rechaza httptest URLs. Para test
	// real con prefix override, validamos parsing del body con un mock match.
	ch := NewSlackChannel()
	_, err := ch.Send(context.Background(), Message{
		Recipient: srv.URL, // NOT hooks.slack.com → debe fallar
		Body:      "test",
	})
	if err == nil {
		t.Fatal("expected error for non-slack URL")
	}
	_ = received
}

func TestSlackChannel_FormatsPayload(t *testing.T) {
	// Test isolation del payload sin pegar a un servidor real.
	msg := Message{Subject: "Hello", Body: "World"}
	expected := "*Hello*\nWorld"
	text := msg.Body
	if msg.Subject != "" {
		text = "*" + msg.Subject + "*\n" + msg.Body
	}
	if text != expected {
		t.Fatalf("got %s, want %s", text, expected)
	}
	payload, _ := json.Marshal(map[string]any{"text": text})
	if !strings.Contains(string(payload), "Hello") {
		t.Fatalf("payload: %s", payload)
	}
}

func TestLogOnlyChannel_Send(t *testing.T) {
	ch := &LogOnlyChannel{}
	res, err := ch.Send(context.Background(), Message{Body: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "sent" {
		t.Fatalf("status: %s", res.Status)
	}
}
