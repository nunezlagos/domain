package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/dispatch"
	"nunezlagos/domain/internal/service/webhook"
)

// POST /webhooks/{slug} — endpoint público para webhooks inbound.
// NO requiere Bearer auth; auth es por HMAC con secret del webhook config.
//
// Headers reconocidos:
//   X-Hub-Signature-256: sha256=<hex>  (GitHub)
//   X-Gitlab-Token: <secret>           (GitLab — comparación constante)
//   X-Domain-Signature: sha256=<hex>   (generic)
func (a *API) receiveWebhook(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if a.WebhookService == nil {
		writeError(w, http.StatusServiceUnavailable, "webhooks_disabled", "")
		return
	}

	hook, secret, err := a.WebhookService.ResolveBySlug(r.Context(), slug)
	if errors.Is(err, webhook.ErrNotFound) {
		// Mismo response que 404 para evitar enumeration
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 5<<20)) // cap 5MB
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_body", err.Error())
		return
	}

	// Verificar HMAC según source_type
	if !verifyWebhookSignature(hook.SourceType, secret, body, r) {
		_ = a.WebhookService.RecordDelivery(r.Context(), hook.ID, body,
			collectHeaders(r), r.RemoteAddr, "signature_invalid", nil, "HMAC mismatch")
		writeError(w, http.StatusUnauthorized, "signature_invalid", "")
		return
	}

	// Construir inputs según mapping
	inputs := buildInputs(body, hook.InputsMapping)

	// Dispatch en background (no block client; webhook devuelve 202 Accepted)
	go a.runWebhookTarget(context.Background(), hook, body, inputs, collectHeaders(r), r.RemoteAddr)

	writeData(w, http.StatusAccepted, map[string]any{
		"received": true, "webhook_id": hook.ID, "target_type": hook.TargetType,
	})
}

func verifyWebhookSignature(sourceType string, secret, body []byte, r *http.Request) bool {
	switch sourceType {
	case "github":
		return webhook.VerifyHMAC(secret, body, r.Header.Get("X-Hub-Signature-256"))
	case "gitlab":
		// GitLab usa token plaintext en header (no HMAC)
		token := r.Header.Get("X-Gitlab-Token")
		return token != "" && token == string(secret)
	case "bitbucket":
		// Bitbucket no firma por default; verificación por IP allowlist (no impl)
		return true
	case "generic":
		return webhook.VerifyHMAC(secret, body, r.Header.Get("X-Domain-Signature"))
	}
	return false
}

func buildInputs(payload []byte, mapping map[string]any) map[string]any {
	inputs := map[string]any{"raw": json.RawMessage(payload)}
	// Mapping JSONPath-like simple: {"foo": "$.field.path"}
	// Implementación MVP: copia field-a-field si values son strings con prefix $.
	var parsed map[string]any
	_ = json.Unmarshal(payload, &parsed)
	for k, v := range mapping {
		if path, ok := v.(string); ok && len(path) > 2 && path[:2] == "$." {
			if val := jsonPathLookup(parsed, path[2:]); val != nil {
				inputs[k] = val
			}
		}
	}
	return inputs
}

func jsonPathLookup(m map[string]any, path string) any {
	parts := splitDot(path)
	var cur any = m
	for _, p := range parts {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[p]
	}
	return cur
}

func splitDot(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == '.' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func collectHeaders(r *http.Request) map[string]string {
	out := map[string]string{}
	for k, v := range r.Header {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}

// runWebhookTarget ejecuta el target del webhook delegando al dispatcher
// unificado (issue-35.1 phase 5). El switch local fue eliminado: ahora
// existe 1 sola implementación (dispatch.Dispatcher) compartida por
// cron, webhook y MCP.
func (a *API) runWebhookTarget(ctx context.Context, hook *webhook.Webhook,
	body []byte, inputs map[string]any, headers map[string]string, sourceIP string) {

	var triggeredID *uuid.UUID
	var errStr string
	status := "triggered"

	if a.Dispatcher == nil {
		errStr = "dispatcher not configured"
		status = "failed"
		_ = a.WebhookService.RecordDelivery(ctx, hook.ID, body,
			headers, sourceIP, status, triggeredID, errStr)
		return
	}

	inputsRaw, err := json.Marshal(inputs)
	if err != nil {
		errStr = fmt.Sprintf("marshal inputs: %v", err)
		status = "failed"
		_ = a.WebhookService.RecordDelivery(ctx, hook.ID, body,
			headers, sourceIP, status, triggeredID, errStr)
		return
	}
	res, dispatchErr := a.Dispatcher.Dispatch(ctx, dispatch.Request{
		OrgID:      hook.OrganizationID,
		Source:     dispatch.SourceWebhook,
		TargetType: hook.TargetType,
		TargetID:   hook.TargetID,
		Inputs:     inputsRaw,
	})
	if dispatchErr != nil {
		errStr = dispatchErr.Error()
		status = "failed"
	}
	if res.RunID != uuid.Nil {
		id := res.RunID
		triggeredID = &id
	}
	_ = a.WebhookService.RecordDelivery(ctx, hook.ID, body,
		headers, sourceIP, status, triggeredID, errStr)
}

// Wrap bytes para evitar consumir body más de una vez (si necesario en futuro)
var _ = bytes.NewReader
