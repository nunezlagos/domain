// REQ-58 — webhook receiver stub para Jira issue updates.
//
// Cuando se conecte Jira (futuro), configurar webhook en Jira hacia
// POST /api/v1/webhooks/jira/issue-updated con shared secret en header
// X-Jira-Webhook-Secret. El handler:
//   1. Verifica el shared secret contra config (X-Jira-Webhook-Secret).
//   2. Parsea el payload de Jira (issue.key, issue.fields.summary,
//      issue.fields.status.name, etc).
//   3. Busca el ticket local por (provider=jira, external_id=issue.key).
//   4. Si encuentra → actualiza title/description/status segun mapping.
//      Si NO encuentra → opcion A: ignora; opcion B: crea ticket nuevo
//      (decision por project_policy 'jira-sync-mode').
//   5. Marca external_synced_at = NOW().
//
// HOY este handler es un STUB que persiste el payload + responde 202
// Accepted. No mapea status ni updates aun. Cuando se conecte Jira
// se agrega el parser real. Mantenemos el endpoint para que la
// integracion sea zero-code-change desde el lado del usuario.
package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

type jiraWebhookPayload struct {
	WebhookEvent string `json:"webhookEvent"`         // ej: "jira:issue_updated"
	Issue        struct {
		Key    string `json:"key"`                      // MPS-12
		Fields struct {
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Status      struct {
				Name string `json:"name"`              // "In Progress", "Done"
			} `json:"status"`
		} `json:"fields"`
	} `json:"issue"`
}

// POST /api/v1/webhooks/jira/issue-updated
// Auth: header X-Jira-Webhook-Secret == env DOMAIN_JIRA_WEBHOOK_SECRET
func (a *API) jiraWebhookIssueUpdated(w http.ResponseWriter, r *http.Request) {

	expected := strings.TrimSpace(os.Getenv("DOMAIN_JIRA_WEBHOOK_SECRET"))
	if expected == "" {

		writeError(w, http.StatusServiceUnavailable, "jira_webhook_not_configured",
			"set DOMAIN_JIRA_WEBHOOK_SECRET to enable")
		return
	}
	got := strings.TrimSpace(r.Header.Get("X-Jira-Webhook-Secret"))
	if got != expected {
		writeError(w, http.StatusUnauthorized, "invalid_webhook_secret", "")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // max 1 MB
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_body", err.Error())
		return
	}
	defer r.Body.Close()

	var payload jiraWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}







	slog.Info("jira webhook received (stub)",
		"event", payload.WebhookEvent,
		"issue_key", payload.Issue.Key,
		"status", payload.Issue.Fields.Status.Name,
		"received_at", time.Now().UTC().Format(time.RFC3339),
	)

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"received":   true,
		"issue_key":  payload.Issue.Key,
		"event":      payload.WebhookEvent,
		"note":       "stub: persiste el payload pero no mapea updates aun (REQ-58).",
	})
}
