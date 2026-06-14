package mail_test

import (
	"log/slog"
	"testing"

	"nunezlagos/domain/internal/mail"
	smtpmail "nunezlagos/domain/internal/mail/smtp"

	"github.com/stretchr/testify/require"
)

func TestSelector_DefaultsToSMTPInDev(t *testing.T) {
	m, err := mail.NewFromConfig(mail.Config{
		Provider: "", Host: "localhost", Port: 1025, From: "noreply@test.com",
	}, "dev", slog.Default())
	require.NoError(t, err)
	_, ok := m.(*smtpmail.Mailer)
	require.True(t, ok, "en dev debe devolver smtp.Mailer")
}

func TestSelector_DefaultsToResendInProd(t *testing.T) {
	m, err := mail.NewFromConfig(mail.Config{
		Provider: "", ResendAPIKey: "re_xxx", From: "noreply@test.com",
	}, "prod", slog.Default())
	require.NoError(t, err)
	require.NotNil(t, m)
}

func TestSelector_RequiresResendKey(t *testing.T) {
	_, err := mail.NewFromConfig(mail.Config{
		Provider: "resend", From: "noreply@test.com",
	}, "prod", slog.Default())
	require.Error(t, err)
	require.Contains(t, err.Error(), "'resend' requires")
}

func TestSelector_RequiresSESRegion(t *testing.T) {
	_, err := mail.NewFromConfig(mail.Config{
		Provider: "ses", From: "noreply@test.com",
	}, "prod", slog.Default())
	require.Error(t, err)
	require.Contains(t, err.Error(), "'ses' requires")
}

func TestSelector_RequiresSendGridKey(t *testing.T) {
	_, err := mail.NewFromConfig(mail.Config{
		Provider: "sendgrid", From: "noreply@test.com",
	}, "prod", slog.Default())
	require.Error(t, err)
	require.Contains(t, err.Error(), "'sendgrid' requires")
}

func TestSelector_UnknownProvider(t *testing.T) {
	_, err := mail.NewFromConfig(mail.Config{
		Provider: "unknown", From: "noreply@test.com",
	}, "dev", slog.Default())
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown SMTP provider")
}
