package outboundwebhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RetryBackoff define el delay para cada intento. Tras agotar → dead_letter.
var RetryBackoff = []time.Duration{
	10 * time.Second,
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	6 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
}

// MaxAttempts = primer intento + len(RetryBackoff) retries.
var MaxAttempts = len(RetryBackoff) + 1

// Event representa un dominio event que dispara entrega.
type Event struct {
	ID         uuid.UUID       `json:"event_id"`
	Type       string          `json:"event_type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

// Dispatcher emite events y procesa la cola de deliveries pendientes.
type Dispatcher struct {
	Pool       *pgxpool.Pool
	Svc        *Service
	HTTPClient *http.Client
	Now        func() time.Time
	Logger     *slog.Logger
}

func (d *Dispatcher) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now().UTC()
}

func (d *Dispatcher) httpClient() *http.Client {
	if d.HTTPClient != nil {
		return d.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// Emit toma un event y encola deliveries para cada subscription matcheada de la org.
// Es síncrono pero rápido: solo INSERT, el dispatcher worker hace el POST HTTP.
func (d *Dispatcher) Emit(ctx context.Context, orgID uuid.UUID, ev Event) error {
	subs, err := d.Svc.ListByEvent(ctx, orgID, ev.Type)
	if err != nil {
		return fmt.Errorf("list subs: %w", err)
	}
	if len(subs) == 0 {
		return nil
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	for _, sub := range subs {
		if !matchesFilters(sub.Filters, ev.Data) {
			continue
		}
		_, err := d.Pool.Exec(ctx,
			`INSERT INTO outbound_webhook_deliveries
				(subscription_id, event_id, event_type, payload,
				 status, next_retry_at)
			 VALUES ($1,$2,$3,$4,'pending', NOW())`,
			sub.ID, ev.ID, ev.Type, payload)
		if err != nil && d.Logger != nil {
			d.Logger.ErrorContext(ctx, "enqueue webhook delivery failed",
				slog.String("subscription_id", sub.ID.String()),
				slog.String("error", err.Error()))
		}
	}
	return nil
}

// ProcessPending procesa hasta `limit` deliveries con next_retry_at <= NOW.
// Usa FOR UPDATE SKIP LOCKED para soportar múltiples workers concurrentes.
// Retorna count procesado.
func (d *Dispatcher) ProcessPending(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx,
		`SELECT id, subscription_id, event_id, event_type, payload, attempt
		 FROM outbound_webhook_deliveries
		 WHERE status = 'pending' AND next_retry_at <= NOW()
		 ORDER BY next_retry_at
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return 0, err
	}
	type job struct {
		ID, SubID, EventID uuid.UUID
		EventType          string
		Payload            []byte
		Attempt            int
	}
	var jobs []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.ID, &j.SubID, &j.EventID, &j.EventType, &j.Payload, &j.Attempt); err != nil {
			rows.Close()
			return 0, err
		}
		jobs = append(jobs, j)
	}
	rows.Close()

	// Marca como "in flight" extendiendo el next_retry_at para que otro worker no la tome.
	for _, j := range jobs {
		_, _ = tx.Exec(ctx,
			`UPDATE outbound_webhook_deliveries
			 SET next_retry_at = NOW() + INTERVAL '5 minutes'
			 WHERE id = $1`, j.ID)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	processed := 0
	for _, j := range jobs {
		d.deliver(ctx, j.ID, j.SubID, j.EventType, j.Payload, j.Attempt)
		processed++
	}
	return processed, nil
}

// deliver envía un delivery individual al endpoint con HMAC + actualiza estado.
func (d *Dispatcher) deliver(ctx context.Context, deliveryID, subID uuid.UUID, eventType string, payload []byte, attempt int) {
	sub, err := d.Svc.GetByIDInternal(ctx, subID)
	if err != nil {
		// La subscription fue borrada; cancelar delivery.
		_, _ = d.Pool.Exec(ctx,
			`UPDATE outbound_webhook_deliveries SET status='failed', error_message=$1
			 WHERE id=$2`, "subscription_not_found", deliveryID)
		return
	}
	// ow-006: circuit breaker — endpoint en cooldown, reprogramar sin
	// gastar intento ni golpear el receptor.
	if d.circuitOpen(sub) {
		retryAt := sub.LastFailureAt.Add(CBCooldown)
		_, _ = d.Pool.Exec(ctx,
			`UPDATE outbound_webhook_deliveries
			 SET status='pending', next_retry_at=$1, error_message='circuit_open'
			 WHERE id=$2`, retryAt, deliveryID)
		return
	}
	secret, _ := d.Svc.DecryptSecret(ctx, subID)

	deliveryUUID := uuid.New()
	ts := strconv.FormatInt(d.now().Unix(), 10)
	signature := ""
	if len(secret) > 0 {
		mac := hmac.New(sha256.New, secret)
		mac.Write([]byte(ts + "."))
		mac.Write(payload)
		signature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(payload))
	if err != nil {
		d.recordFailure(ctx, deliveryID, subID, attempt, 0, "", 0, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "domain-webhooks/1.0")
	req.Header.Set("X-Domain-Event", eventType)
	req.Header.Set("X-Domain-Delivery-Id", deliveryUUID.String())
	req.Header.Set("X-Domain-Timestamp", ts)
	if signature != "" {
		req.Header.Set("X-Domain-Signature", signature)
	}

	start := time.Now()
	resp, err := d.httpClient().Do(req)
	duration := int(time.Since(start) / time.Millisecond)
	if err != nil {
		d.recordFailure(ctx, deliveryID, subID, attempt, 0, "", duration, err.Error())
		return
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		d.recordSuccess(ctx, deliveryID, subID, resp.StatusCode, string(bodyBytes), duration)
		return
	}
	d.recordFailure(ctx, deliveryID, subID, attempt, resp.StatusCode, string(bodyBytes), duration,
		fmt.Sprintf("http_%d", resp.StatusCode))
}

func (d *Dispatcher) recordSuccess(ctx context.Context, deliveryID, subID uuid.UUID, code int, body string, duration int) {
	_, _ = d.Pool.Exec(ctx,
		`UPDATE outbound_webhook_deliveries
		 SET status='succeeded', response_code=$1, response_body=$2,
			 duration_ms=$3, delivered_at=NOW(), next_retry_at=NULL
		 WHERE id=$4`, code, truncate(body, 2000), duration, deliveryID)
	_, _ = d.Pool.Exec(ctx,
		`UPDATE outbound_webhook_subscriptions
		 SET last_success_at=NOW(), failure_count=0
		 WHERE id=$1`, subID)
}

func (d *Dispatcher) recordFailure(ctx context.Context, deliveryID, subID uuid.UUID, attempt, code int, body string, duration int, errMsg string) {
	nextAttempt := attempt + 1
	if nextAttempt > MaxAttempts {
		_, _ = d.Pool.Exec(ctx,
			`UPDATE outbound_webhook_deliveries
			 SET status='dead_letter', response_code=$1, response_body=$2,
				 duration_ms=$3, error_message=$4, attempt=$5, next_retry_at=NULL
			 WHERE id=$6`,
			nilIfZero(code), truncate(body, 2000), duration, errMsg, nextAttempt, deliveryID)
	} else {
		backoff := RetryBackoff[attempt-1]
		if attempt-1 >= len(RetryBackoff) {
			backoff = RetryBackoff[len(RetryBackoff)-1]
		}
		nextRetry := d.now().Add(backoff)
		_, _ = d.Pool.Exec(ctx,
			`UPDATE outbound_webhook_deliveries
			 SET status='pending', response_code=$1, response_body=$2,
				 duration_ms=$3, error_message=$4, attempt=$5, next_retry_at=$6
			 WHERE id=$7`,
			nilIfZero(code), truncate(body, 2000), duration, errMsg, nextAttempt, nextRetry, deliveryID)
	}
	_, _ = d.Pool.Exec(ctx,
		`UPDATE outbound_webhook_subscriptions
		 SET last_failure_at=NOW(), failure_count=failure_count+1
		 WHERE id=$1`, subID)
}

func nilIfZero(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// matchesFilters evalúa filtros sobre el payload data (ow-008).
// Keys soportan paths anidados con punto ("flow.slug": "deploy") evaluados
// de forma segura (sin eval); el match es por equality de representación.
func matchesFilters(filters, data json.RawMessage) bool {
	if len(filters) == 0 || bytes.Equal(filters, []byte("{}")) {
		return true
	}
	var f map[string]any
	if err := json.Unmarshal(filters, &f); err != nil || len(f) == 0 {
		return true
	}
	var d map[string]any
	if err := json.Unmarshal(data, &d); err != nil {
		return false
	}
	for path, want := range f {
		got, ok := lookupPath(d, path)
		if !ok {
			return false
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			return false
		}
	}
	return true
}

// lookupPath navega un path con puntos sobre maps anidados (y arrays por
// índice numérico). Traversal puro de datos — nunca evalúa expresiones.
func lookupPath(doc map[string]any, path string) (any, bool) {
	var cur any = doc
	for _, seg := range strings.Split(path, ".") {
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[seg]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			cur = v[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// Circuit breaker (ow-006): tras CBThreshold fallos consecutivos, la
// subscription queda en cooldown CBCooldown desde el último fallo — los
// deliveries se reprograman sin gastar intentos ni golpear el endpoint.
const (
	CBThreshold = 10
	CBCooldown  = 1 * time.Hour
)

// circuitOpen indica si la subscription está en cooldown.
func (d *Dispatcher) circuitOpen(sub *Subscription) bool {
	if sub.FailureCount < CBThreshold || sub.LastFailureAt == nil {
		return false
	}
	return d.now().Sub(*sub.LastFailureAt) < CBCooldown
}
