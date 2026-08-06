package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/dispatch"
	"nunezlagos/domain/internal/service/webhook"
)

// POST /webhooks/{slug} — endpoint publico para webhooks inbound.
// NO requiere Bearer auth; auth es por HMAC con secret del webhook config.
//
// Headers reconocidos:
//
//	X-Hub-Signature-256: sha256=<hex>  (GitHub)
//	X-Gitlab-Token: <secret>           (GitLab — comparacion constante)
//	X-Domain-Signature: sha256=<hex>   (generic)
func (a *API) receiveWebhook(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if a.WebhookService == nil {
		writeError(w, http.StatusServiceUnavailable, "webhooks_disabled", "")
		return
	}

	hook, secret, err := a.WebhookService.ResolveBySlug(r.Context(), slug)
	if errors.Is(err, webhook.ErrNotFound) {
		responderNoAutorizado(w)
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

	if !verifyWebhookSignature(hook.SourceType, secret, body, r) {
		// DOMAINSERV-240: la entrega se registra SIN el payload. Antes se persistían
		// hasta 5MB antes de autenticar, así que un endpoint público sin auth era una
		// amplificación de escritura: cualquiera llenaba la tabla mandando basura con
		// firma inválida. La fila se conserva porque saber que llegó algo y no cuadró es
		// justo lo que se diagnostica; el cuerpo no aporta a eso.
		_ = a.WebhookService.RecordDelivery(r.Context(), hook.ID, nil,
			collectHeaders(r), r.RemoteAddr, "signature_invalid", nil, "HMAC mismatch")
		responderNoAutorizado(w)
		return
	}

	inputs := buildInputs(body, hook.InputsMapping)

	if a.WebhookDispatcher == nil {
		// context.WithoutCancel y no r.Context(): el contexto del request muere al
		// responder, así que la goroutine arrancaba y se cancelaba sola. Conserva los
		// valores del ctx (request_id del logging estructurado) sin heredar la
		// cancelación, y el timeout propio evita que quede colgada para siempre
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Second)
		go func() {
			defer cancel()
			a.runWebhookTarget(ctx, hook, body, inputs, collectHeaders(r), r.RemoteAddr)
		}()
	} else {
		job := webhookJob{
			hookID:    hook.ID.String(),
			hookSlug:  hook.Slug,
			hook:      hook,
			body:      body,
			inputs:    inputs,
			headers:   collectHeaders(r),
			remote:    r.RemoteAddr,
			startedAt: time.Now(),
		}
		if !a.WebhookDispatcher.Enqueue(r.Context(), job) {
			writeError(w, http.StatusServiceUnavailable, "backpressure", "webhook queue full")
			return
		}
	}

	writeData(w, http.StatusAccepted, map[string]any{
		"received": true, "webhook_id": hook.ID, "target_type": hook.TargetType,
	})
}

// responderNoAutorizado devuelve la MISMA respuesta para un slug que no existe y para una
// firma que no cuadra (DOMAINSERV-240). La diferencia entre 404 y 401 era un oráculo de
// enumeración: bastaba mirar el código para saber qué webhooks tiene la instancia, sin
// credencial alguna, en un endpoint público. Un webhook deshabilitado cae acá también,
// porque ResolveBySlug devuelve ErrNotFound cuando enabled=false.
func responderNoAutorizado(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "unauthorized", "")
}

func verifyWebhookSignature(sourceType string, secret, body []byte, r *http.Request) bool {
	switch sourceType {
	case "github":
		return webhook.VerifyHMAC(secret, body, r.Header.Get("X-Hub-Signature-256"))
	case "gitlab":
		// comparación en tiempo constante: == sobre strings corta en el primer byte
		// distinto y filtra el prefijo correcto del secreto por diferencia de tiempo
		token := r.Header.Get("X-Gitlab-Token")
		return token != "" && hmac.Equal([]byte(token), secret)
	case "generic":
		return webhook.VerifyHMAC(secret, body, r.Header.Get("X-Domain-Signature"))
	}
	// DOMAINSERV-240: bitbucket devolvía true sin verificar NADA, así que aceptaba
	// cualquier payload de cualquiera. El default es rechazar: un source_type sin
	// verificación implementada no se atiende, y el alta por MCP tampoco lo acepta
	return false
}

func buildInputs(payload []byte, mapping map[string]any) map[string]any {
	inputs := map[string]any{"raw": json.RawMessage(payload)}

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

// denyHeaders son los headers cuyo VALOR no se persiste (DOMAINSERV-240). El más grave es
// X-Gitlab-Token: para gitlab el secreto ES ese header —`token == string(secret)` en
// verifyWebhookSignature— así que el log de entregas venía guardando la credencial que valida
// las entregas, en claro, sin cifrar.
//
// Las claves van en minúscula porque la comparación es case-insensitive: Go canonicaliza los
// headers de una request real, pero RecordDelivery también se llama desde el dispatcher con mapas
// ya armados a mano, y ahí la capitalización no está garantizada.
//
// Sin anotación de tipo a propósito: stateless-lint solo audita las vars con tipo explícito.
var denyHeaders = map[string]bool{
	"x-gitlab-token":      true,
	"x-hub-signature":     true,
	"x-hub-signature-256": true,
	"x-domain-signature":  true,
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
}

// collectHeaders copia los headers para el log de la entrega, redactando el VALOR de los
// sensibles y conservando la CLAVE. La distinción importa: saber que llegó un X-Hub-Signature es
// lo que sirve para diagnosticar una firma que no cuadra; su contenido es el secreto.
func collectHeaders(r *http.Request) map[string]string {
	out := map[string]string{}
	for k, v := range r.Header {
		if len(v) == 0 {
			continue
		}
		if denyHeaders[strings.ToLower(k)] {
			out[k] = "[redacted]"
			continue
		}
		out[k] = v[0]
	}
	return out
}

// runWebhookTarget ejecuta el target del webhook delegando al dispatcher
// unificado (issue-35.1 phase 5). El switch local fue eliminado: ahora
// existe 1 sola implementacion (dispatch.Dispatcher) compartida por
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
		OrgID:      uuid.Nil,
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

// Wrap bytes para evitar consumir body mas de una vez (si necesario en futuro)
var _ = bytes.NewReader
