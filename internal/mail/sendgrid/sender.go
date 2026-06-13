package sendgrid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const DefaultBaseURL = "https://api.sendgrid.com/v3/mail/send"

type Sender struct {
	APIKey     string
	From       string
	HTTPClient *http.Client
	Logger     *slog.Logger
	BaseURL    string
}

func New(apiKey, from string, logger *slog.Logger) *Sender {
	return &Sender{
		APIKey: apiKey,
		From:   from,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Logger:  logger,
		BaseURL: DefaultBaseURL,
	}
}

func (s *Sender) Send(ctx context.Context, to, subject, body string) error {
	payload := map[string]any{
		"personalizations": []any{
			map[string]any{
				"to": []any{
					map[string]string{"email": to},
				},
			},
		},
		"from": map[string]string{"email": s.From},
		"subject": map[string]string{"value": subject},
		"content": []any{
			map[string]string{
				"type":  "text/plain",
				"value": body,
			},
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("sendgrid marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("sendgrid req: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("sendgrid request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("sendgrid: %s", resp.Status)
	}

	s.Logger.Info("email sent",
		slog.String("provider", "sendgrid"),
		slog.String("to", to),
		slog.String("subject", subject),
	)
	return nil
}

func (s *Sender) SendOTP(ctx context.Context, to, code string, expiresIn time.Duration) error {
	subject := "Tu código de acceso a Domain"
	body := fmt.Sprintf("Tu código: %s\nVence en: %s\n\nSi no lo solicitaste, ignorá este correo.",
		code, expiresIn.String())
	return s.Send(ctx, to, subject, body)
}
