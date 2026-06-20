package mail

import (
	"fmt"
	"log/slog"

	"nunezlagos/domain/internal/mail/resend"
	"nunezlagos/domain/internal/mail/sendgrid"
	"nunezlagos/domain/internal/mail/ses"
	smtpmail "nunezlagos/domain/internal/mail/smtp"
)

type Config struct {
	Provider      string
	Host          string
	Port          int
	Auth          string
	User          string
	Password      string
	TLS           bool
	From          string
	ResendAPIKey  string
	SendGridAPIKey string
	AWSRegion     string
}

func NewFromConfig(cfg Config, env string, logger *slog.Logger) (Mailer, error) {
	provider := cfg.Provider
	if provider == "" {
		if env == "prod" {
			provider = "resend"
			logger.Info("SMTP provider: resend (default for prod)")
		} else {
			provider = "smtp"
		}
	}

	switch provider {
	case "smtp", "":
		return smtpmail.New(smtpmail.Config{
			Host: cfg.Host, Port: cfg.Port, Auth: cfg.Auth,
			User: cfg.User, Password: cfg.Password,
			UseTLS: cfg.TLS, From: cfg.From,
		}), nil

	case "resend":
		if cfg.ResendAPIKey == "" {
			return nil, fmt.Errorf("SMTP provider 'resend' requires DOMAIN_RESEND_API_KEY")
		}
		return resend.New(cfg.ResendAPIKey, cfg.From, logger), nil

	case "ses":
		if cfg.AWSRegion == "" {
			return nil, fmt.Errorf("SMTP provider 'ses' requires AWS_REGION")
		}
		return ses.New(cfg.AWSRegion, cfg.From, logger)

	case "sendgrid":
		if cfg.SendGridAPIKey == "" {
			return nil, fmt.Errorf("SMTP provider 'sendgrid' requires DOMAIN_SENDGRID_API_KEY")
		}
		return sendgrid.New(cfg.SendGridAPIKey, cfg.From, logger), nil

	default:
		return nil, fmt.Errorf("unknown SMTP provider: %s", provider)
	}
}
