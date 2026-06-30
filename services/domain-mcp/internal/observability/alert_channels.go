// Package observability: este archivo cubre los canales de alerta HTTP
// (webhook y ntfy). Ambos comparten un POST con reintentos + backoff.
//
// issue-53.9 early-error-reporting.
package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// backoffSteps son los delays entre reintentos (1s, 5s, 30s).
var backoffSteps = []time.Duration{time.Second, 5 * time.Second, 30 * time.Second}

func backoffFor(attempt int) time.Duration {
	if attempt-1 < len(backoffSteps) {
		return backoffSteps[attempt-1]
	}
	return backoffSteps[len(backoffSteps)-1]
}

// alertPayload serializa el evento para el body del webhook.
func alertPayload(e ErrorEvent) []byte {
	b, _ := json.Marshal(map[string]string{
		"category":    string(e.Category),
		"severity":    e.Severity,
		"message":     e.Message,
		"source":      e.Source,
		"workflow_id": e.WorkflowID,
		"fingerprint": fmt.Sprintf("%x", e.Fingerprint),
	})
	return b
}

// postWithRetry hace POST con reintentos; 2xx = exito. sleep es inyectable.
func postWithRetry(ctx context.Context, client *http.Client, sleep func(time.Duration), retries int,
	url, contentType string, body []byte, headers map[string]string) error {
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			sleep(backoffFor(attempt))
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", contentType)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode/100 == 2 {
			return nil
		}
		lastErr = fmt.Errorf("status %d", resp.StatusCode)
	}
	return lastErr
}

// WebhookNotifier postea el evento como JSON a la url configurada.
type WebhookNotifier struct {
	Client  *http.Client
	Retries int
	sleep   func(time.Duration)
}

// NewWebhookNotifier construye un notifier con defaults razonables.
func NewWebhookNotifier() *WebhookNotifier {
	return &WebhookNotifier{Client: &http.Client{Timeout: 10 * time.Second}, Retries: 2, sleep: time.Sleep}
}

// Send postea el payload JSON. Requiere ChannelConfig["url"].
func (w *WebhookNotifier) Send(ctx context.Context, e ErrorEvent, cfg AlertConfig) error {
	url := cfgStr(cfg.ChannelConfig, "url")
	if url == "" {
		return fmt.Errorf("webhook: missing url in channel_config")
	}
	return postWithRetry(ctx, w.Client, w.sleep, w.Retries, url, "application/json", alertPayload(e), nil)
}

// NtfyNotifier publica en https://ntfy.sh/<topic> (BaseURL configurable).
type NtfyNotifier struct {
	Client  *http.Client
	BaseURL string
	Retries int
	sleep   func(time.Duration)
}

// NewNtfyNotifier construye un notifier ntfy con BaseURL por defecto.
func NewNtfyNotifier() *NtfyNotifier {
	return &NtfyNotifier{Client: &http.Client{Timeout: 10 * time.Second}, BaseURL: "https://ntfy.sh", Retries: 2, sleep: time.Sleep}
}

// Send publica el mensaje al topic. Topic de ChannelConfig["topic"] o DOMAIN_NTFY_TOPIC.
func (n *NtfyNotifier) Send(ctx context.Context, e ErrorEvent, cfg AlertConfig) error {
	topic := cfgStr(cfg.ChannelConfig, "topic")
	if topic == "" {
		topic = os.Getenv("DOMAIN_NTFY_TOPIC")
	}
	if topic == "" {
		return fmt.Errorf("ntfy: missing topic")
	}
	headers := map[string]string{
		"Title":    fmt.Sprintf("[%s] %s", e.Category, e.Severity),
		"Priority": ntfyPriority(e.Severity),
		"Tags":     "rotating_light",
	}
	return postWithRetry(ctx, n.Client, n.sleep, n.Retries, n.BaseURL+"/"+topic, "text/plain", []byte(e.Message), headers)
}

// ntfyPriority mapea severidad a la escala de prioridad de ntfy (1..5).
func ntfyPriority(sev string) string {
	switch sev {
	case "critical":
		return "5"
	case "error":
		return "4"
	default:
		return "3"
	}
}
