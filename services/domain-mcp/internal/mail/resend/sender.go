package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"time"
)

const DefaultBaseURL = "https://api.resend.com/emails"

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
		"from":    s.From,
		"to":      to,
		"subject": subject,
		"text":    body,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("resend marshal: %w", err)
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		if i > 0 {
			backoff := time.Duration(1<<i) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff + time.Duration(rand.Intn(500))*time.Millisecond):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("resend req: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("resend request: %w", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("resend: %s", resp.Status)
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("resend: %s", resp.Status)
			continue
		}

		s.Logger.Info("email sent",
			slog.String("provider", "resend"),
			slog.String("to", to),
			slog.String("subject", subject),
		)
		return nil
	}

	return fmt.Errorf("resend: max retries exceeded: %w", lastErr)
}
