package observability

import (
	"context"
	"net/smtp"
	"strings"
	"testing"
)

func TestEmailNotifier_Send_CallsSendMailWithSubject(t *testing.T) {
	var gotMsg string
	var gotTo []string
	em := &EmailNotifier{
		Host: "smtp.local:25",
		From: "domain@local",
		send: func(_ string, _ smtp.Auth, _ string, to []string, msg []byte) error {
			gotTo, gotMsg = to, string(msg)
			return nil
		},
	}
	cfg := AlertConfig{Channel: "email", ChannelConfig: map[string]string{"to": "ops@local"}}
	if err := em.Send(context.Background(), sqlEvent("error"), cfg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(gotTo) != 1 || gotTo[0] != "ops@local" {
		t.Fatalf("to: got %v", gotTo)
	}
	if !strings.Contains(gotMsg, "Subject:") || !strings.Contains(gotMsg, "SQL_ERROR") {
		t.Fatalf("message should carry subject+category, got %q", gotMsg)
	}
}

func TestEmailNotifier_Send_MissingTo_Errors(t *testing.T) {
	em := &EmailNotifier{Host: "smtp.local:25", send: func(string, smtp.Auth, string, []string, []byte) error { return nil }}
	if err := em.Send(context.Background(), sqlEvent("error"), AlertConfig{Channel: "email"}); err == nil {
		t.Fatal("missing to must error")
	}
}

func TestEmailNotifier_Send_MissingHost_Errors(t *testing.T) {
	em := &EmailNotifier{send: func(string, smtp.Auth, string, []string, []byte) error { return nil }}
	cfg := AlertConfig{Channel: "email", ChannelConfig: map[string]string{"to": "ops@local"}}
	if err := em.Send(context.Background(), sqlEvent("error"), cfg); err == nil {
		t.Fatal("missing host must error")
	}
}
