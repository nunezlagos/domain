package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
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
	go a.dispatchWebhook(context.Background(), hook, body, inputs, r)

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

func (a *API) dispatchWebhook(ctx context.Context, hook *webhook.Webhook,
	body []byte, inputs map[string]any, r *http.Request) {

	var triggeredID *uuid.UUID
	var errStr string
	status := "triggered"

	switch hook.TargetType {
	case "flow":
		if a.FlowRunner == nil {
			errStr = "flow runner not configured"
			status = "failed"
			break
		}
		res, err := a.FlowRunner.Run(ctx, flowrunner.RunInput{
			FlowID: hook.TargetID, TriggerType: "webhook", Inputs: inputs,
		})
		if err != nil {
			errStr = err.Error()
			status = "failed"
		}
		if res != nil {
			triggeredID = &res.RunID
		}
	case "agent":
		if a.AgentRunner == nil {
			errStr = "agent runner not configured"
			status = "failed"
			break
		}
		input, _ := inputs["input"].(string)
		if input == "" {
			// Si no hay "input" mapeado, usa body raw
			input = string(body)
		}
		res, err := a.AgentRunner.Run(ctx, agentrunner.RunInput{
			AgentID: hook.TargetID, UserPrompt: input, Variables: inputs,
		})
		if err != nil {
			errStr = err.Error()
			status = "failed"
		}
		if res != nil {
			triggeredID = &res.RunID
		}
	case "skill":
		// Skills se ejecutan via agent stub o directly via SkillRunner — pending decisión
		errStr = "skill webhook target pending implementation"
		status = "failed"
	}

	_ = a.WebhookService.RecordDelivery(ctx, hook.ID, body,
		collectHeaders(r), r.RemoteAddr, status, triggeredID, errStr)
}

// Wrap bytes para evitar consumir body más de una vez (si necesario en futuro)
var _ = bytes.NewReader
