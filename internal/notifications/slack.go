package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SlackChannel — HU-20.3 webhook delivery a Slack vía incoming webhook URL.
//
// El recipient en Message debe ser una webhook URL completa (https://hooks.slack.com/...).
// Body se mappea al campo "text" del payload Slack. Subject se prepende como bold.
type SlackChannel struct {
	HTTPClient *http.Client
}

// NewSlackChannel default con timeout 10s.
func NewSlackChannel() *SlackChannel {
	return &SlackChannel{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SlackChannel) Slug() string { return "slack" }

func (s *SlackChannel) Send(ctx context.Context, msg Message) (DeliveryResult, error) {
	if !strings.HasPrefix(msg.Recipient, "https://hooks.slack.com/") {
		return DeliveryResult{Status: "failed", ErrorMessage: "invalid slack webhook URL"},
			fmt.Errorf("recipient must be slack webhook URL")
	}
	text := msg.Body
	if msg.Subject != "" {
		text = "*" + msg.Subject + "*\n" + msg.Body
	}
	payload, _ := json.Marshal(map[string]any{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msg.Recipient, bytes.NewReader(payload))
	if err != nil {
		return DeliveryResult{Status: "failed", ErrorMessage: err.Error()}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return DeliveryResult{Status: "failed", ErrorMessage: err.Error()}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return DeliveryResult{Status: "sent", ResponseCode: resp.StatusCode}, nil
	}
	return DeliveryResult{
		Status:       "failed",
		ResponseCode: resp.StatusCode,
		ErrorMessage: fmt.Sprintf("slack returned %d", resp.StatusCode),
	}, fmt.Errorf("slack http %d", resp.StatusCode)
}

// LogOnlyChannel canal mock para dev/testing.
type LogOnlyChannel struct{}

func (l *LogOnlyChannel) Slug() string { return "log_only" }

func (l *LogOnlyChannel) Send(_ context.Context, _ Message) (DeliveryResult, error) {
	return DeliveryResult{Status: "sent", ResponseCode: 200}, nil
}
