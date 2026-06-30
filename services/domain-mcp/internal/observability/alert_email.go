// Package observability: este archivo cubre el canal de alerta email (SMTP).
// El envio (smtp.SendMail) es inyectable para test.
//
// issue-53.9 early-error-reporting.
package observability

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
)

// EmailNotifier envia la alerta por SMTP via DOMAIN_SMTP_HOST.
type EmailNotifier struct {
	Host string
	From string
	send func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// NewEmailNotifier lee Host/From de env (DOMAIN_SMTP_HOST/DOMAIN_SMTP_FROM).
func NewEmailNotifier() *EmailNotifier {
	from := os.Getenv("DOMAIN_SMTP_FROM")
	if from == "" {
		from = "domain@localhost"
	}
	return &EmailNotifier{
		Host: os.Getenv("DOMAIN_SMTP_HOST"),
		From: from,
		send: smtp.SendMail,
	}
}

// Send arma el mensaje y lo envia. Requiere Host y ChannelConfig["to"].
func (em *EmailNotifier) Send(_ context.Context, e ErrorEvent, cfg AlertConfig) error {
	to := cfg.ChannelConfig["to"]
	if to == "" {
		return fmt.Errorf("email: missing 'to' in channel_config")
	}
	if em.Host == "" {
		return fmt.Errorf("email: missing DOMAIN_SMTP_HOST")
	}
	return em.send(em.Host, nil, em.From, []string{to}, buildEmailMessage(e, em.From, to))
}

// buildEmailMessage arma el RFC822 minimo: subject "[category] fp-prefix" + body.
func buildEmailMessage(e ErrorEvent, from, to string) []byte {
	subject := fmt.Sprintf("[%s] %s", e.Category, fingerprintPrefix(e))
	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		e.Message + "\r\n"
	return []byte(msg)
}

// fingerprintPrefix devuelve los primeros 8 chars hex del fingerprint.
func fingerprintPrefix(e ErrorEvent) string {
	s := fmt.Sprintf("%x", e.Fingerprint)
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
