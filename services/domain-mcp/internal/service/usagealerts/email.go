package usagealerts

import (
	"context"
	"fmt"
	"strings"
)

// SMTPMailer interface mínima para enviar emails de alerta.
// Implementada por smtpmail.Mailer en main.go.
type SMTPMailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// smtpSender adapta SMTPMailer a EmailSender (multi-to).
type smtpSender struct {
	mailer SMTPMailer
}

func NewSMTPEmailSender(mailer SMTPMailer) EmailSender {
	return &smtpSender{mailer: mailer}
}

func (s *smtpSender) SendAlertEmail(ctx context.Context, to []string, subject, body string) error {
	if len(to) == 0 {
		return nil
	}

	first := to[0]
	cc := ""
	if len(to) > 1 {
		cc = "\r\nCc: " + strings.Join(to[1:], ",")
	}
	fullBody := body + cc
	err := s.mailer.Send(ctx, first, subject, fullBody)
	if err != nil {
		return fmt.Errorf("alert email: %w", err)
	}
	return nil
}
