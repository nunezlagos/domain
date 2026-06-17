//go:build integration

package outboundwebhook_test

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/crypto"
	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	ow "nunezlagos/domain/internal/service/outboundwebhook"
)

type owFixture struct {
	disp  *ow.Dispatcher
	svc   *ow.Service
	pool  *db.Pools
	orgID uuid.UUID
}

func setupOW(t *testing.T) (*owFixture, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))
	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)

	org, _, err := seedOrgUser(ctx, pools.App, "OWOrg", "oworg", "o@x.com", "O")
	require.NoError(t, err)

	key := make([]byte, crypto.MasterKeySize)
	_, err = rand.Read(key)
	require.NoError(t, err)
	cipherInst, err := crypto.NewCipher(key)
	require.NoError(t, err)

	svc := &ow.Service{Pool: pools.App, Cipher: cipherInst}
	disp := &ow.Dispatcher{
		Pool: pools.App, Svc: svc,
		HTTPClient: &http.Client{Timeout: 3 * time.Second},
	}
	return &owFixture{disp: disp, svc: svc, pool: pools, orgID: org.ID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func (f *owFixture) subscribe(t *testing.T, url, event string, filters string) *ow.Subscription {
	t.Helper()
	ctx := context.Background()
	// El SSRF validator bloquea loopback (correcto en prod, test-005 implícito):
	// se crea con URL pública válida y se apunta al receptor httptest vía SQL.
	in := ow.CreateInput{Name: "sub-" + event, URL: "https://example.com/hook",
		Events: []string{event}, Secret: "whsec_test_secret"}
	if filters != "" {
		in.Filters = json.RawMessage(filters)
	}
	sub, err := f.svc.Create(ctx, f.orgID, in, false)
	require.NoError(t, err)
	_, err = f.pool.App.Exec(ctx,
		`UPDATE outbound_webhook_subscriptions SET url = $1 WHERE id = $2`, url, sub.ID)
	require.NoError(t, err)
	return sub
}

func (f *owFixture) emitAndProcess(t *testing.T, event string, data string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, f.disp.Emit(ctx, f.orgID, ow.Event{
		ID: uuid.New(), Type: event, OccurredAt: time.Now().UTC(),
		Data: json.RawMessage(data),
	}))
	_, err := f.disp.ProcessPending(ctx, 50)
	require.NoError(t, err)
}

// test-001/002: delivery con HMAC verificable por el receptor.
func TestDelivery_HMACVerifiable(t *testing.T) {
	f, cleanup := setupOW(t)
	defer cleanup()
	ctx := context.Background()

	var gotSig, gotTS string
	var gotBody []byte
	received := make(chan struct{}, 1)
	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Domain-Signature")
		gotTS = r.Header.Get("X-Domain-Timestamp")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer recv.Close()

	sub := f.subscribe(t, recv.URL, "agent_run.completed", "")
	f.emitAndProcess(t, "agent_run.completed", `{"run_id":"x","status":"completed"}`)

	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("receptor nunca recibió el webhook")
	}

	// HMAC verificable: sha256=hex(hmac(secret, ts + "." + body))
	secret, err := f.svc.DecryptSecret(ctx, sub.ID)
	require.NoError(t, err)
	require.NotEmpty(t, secret)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(gotTS + "."))
	mac.Write(gotBody)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	require.Equal(t, want, gotSig, "la firma debe ser verificable por el receptor")

	// Delivery marcada succeeded
	var status string
	require.NoError(t, f.pool.App.QueryRow(ctx,
		`SELECT status FROM outbound_webhook_deliveries WHERE subscription_id = $1`,
		sub.ID).Scan(&status))
	require.Equal(t, "succeeded", status)
}

// test-004: filtros con path anidado deciden la entrega.
func TestDelivery_FiltersApplied(t *testing.T) {
	f, cleanup := setupOW(t)
	defer cleanup()
	ctx := context.Background()

	var hits atomic.Int32
	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer recv.Close()

	f.subscribe(t, recv.URL, "flow_run.completed", `{"flow_slug":"deploy"}`)

	// Evento que NO matchea → ninguna delivery encolada
	f.emitAndProcess(t, "flow_run.completed", `{"flow_slug":"otro","status":"completed"}`)
	var count int
	require.NoError(t, f.pool.App.QueryRow(ctx,
		`SELECT COUNT(*) FROM outbound_webhook_deliveries`).Scan(&count))
	require.Zero(t, count, "evento filtrado no debe encolar delivery")

	// Evento que matchea → entrega
	f.emitAndProcess(t, "flow_run.completed", `{"flow_slug":"deploy","status":"completed"}`)
	require.Eventually(t, func() bool { return hits.Load() == 1 },
		5*time.Second, 100*time.Millisecond)
}

// test-003: receptor 503 → reintento programado con backoff; test-007: replay.
func TestDelivery_RetryAndReplay(t *testing.T) {
	f, cleanup := setupOW(t)
	defer cleanup()
	ctx := context.Background()

	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer recv.Close()

	sub := f.subscribe(t, recv.URL, "agent_run.failed", "")
	f.emitAndProcess(t, "agent_run.failed", `{"run_id":"y"}`)

	var status string
	var attempt int
	var deliveryID uuid.UUID
	require.NoError(t, f.pool.App.QueryRow(ctx, `
		SELECT id, status, attempt FROM outbound_webhook_deliveries
		WHERE subscription_id = $1`, sub.ID).Scan(&deliveryID, &status, &attempt))
	require.Equal(t, "pending", status, "503 → sigue pending con backoff")
	require.Equal(t, 2, attempt)

	// Replay manual: resetea a pending NOW con ciclo fresco
	_, err := f.pool.App.Exec(ctx, `
		UPDATE outbound_webhook_deliveries
		SET status='dead_letter', next_retry_at=NULL WHERE id=$1`, deliveryID)
	require.NoError(t, err)
	tag, err := f.pool.App.Exec(ctx, `
		UPDATE outbound_webhook_deliveries
		SET status='pending', next_retry_at=NOW(), attempt=1, error_message=NULL
		WHERE id=$1 AND organization_id=$2`, deliveryID, f.orgID)
	require.NoError(t, err)
	require.EqualValues(t, 1, tag.RowsAffected())

	n, err := f.disp.ProcessPending(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, n, "replay re-procesa el delivery")
}

// test-006: circuit breaker — con failure_count alto y fallo reciente, el
// dispatcher reprograma sin golpear el endpoint.
func TestDelivery_CircuitBreakerSkips(t *testing.T) {
	f, cleanup := setupOW(t)
	defer cleanup()
	ctx := context.Background()

	var hits atomic.Int32
	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer recv.Close()

	sub := f.subscribe(t, recv.URL, "agent_run.completed", "")

	// Forzar estado de breaker abierto
	_, err := f.pool.App.Exec(ctx, `
		UPDATE outbound_webhook_subscriptions
		SET failure_count = $2, last_failure_at = NOW()
		WHERE id = $1`, sub.ID, ow.CBThreshold)
	require.NoError(t, err)

	f.emitAndProcess(t, "agent_run.completed", `{"run_id":"z"}`)

	require.Zero(t, hits.Load(), "breaker abierto → el endpoint NO se golpea")
	var status, errMsg string
	require.NoError(t, f.pool.App.QueryRow(ctx, `
		SELECT status, COALESCE(error_message,'') FROM outbound_webhook_deliveries
		WHERE subscription_id = $1`, sub.ID).Scan(&status, &errMsg))
	require.Equal(t, "pending", status)
	require.Equal(t, "circuit_open", errMsg)
}
