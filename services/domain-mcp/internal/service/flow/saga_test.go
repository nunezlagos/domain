package flow

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSagaStore_RegisterCompensation(t *testing.T) {
	store := NewSagaStore(nil)
	runID := uuid.New()

	comp := SagaCompensation{
		StepKey:       "create_user",
		CompensateKey: "delete_user",
		MaxRetries:    3,
	}

	err := store.RegisterCompensation(context.Background(), runID, comp)
	if err != nil {
		t.Fatalf("RegisterCompensation: %v", err)
	}

	plans := store.RegisteredCompensations(runID)
	if len(plans) != 1 {
		t.Fatalf("expected 1 compensation, got %d", len(plans))
	}
	if plans[0].StepKey != "create_user" {
		t.Fatalf("expected step create_user, got %s", plans[0].StepKey)
	}
}

func TestSagaStore_RegisterCompensation_ReplacesExisting(t *testing.T) {
	store := NewSagaStore(nil)
	runID := uuid.New()

	_ = store.RegisterCompensation(context.Background(), runID, SagaCompensation{
		StepKey:       "create_user",
		CompensateKey: "delete_user",
		MaxRetries:    3,
	})
	_ = store.RegisterCompensation(context.Background(), runID, SagaCompensation{
		StepKey:       "create_user",
		CompensateKey: "purge_user",
		MaxRetries:    5,
	})

	plans := store.RegisteredCompensations(runID)
	if len(plans) != 1 {
		t.Fatalf("expected 1 compensation after replace, got %d", len(plans))
	}
	if plans[0].CompensateKey != "purge_user" {
		t.Fatalf("expected compensate_key purge_user, got %s", plans[0].CompensateKey)
	}
	if plans[0].MaxRetries != 5 {
		t.Fatalf("expected max_retries 5, got %d", plans[0].MaxRetries)
	}
}

func TestSagaStore_MultipleSteps(t *testing.T) {
	store := NewSagaStore(nil)
	runID := uuid.New()

	comps := []SagaCompensation{
		{StepKey: "step_a", CompensateKey: "undo_a"},
		{StepKey: "step_b", CompensateKey: "undo_b"},
		{StepKey: "step_c", CompensateKey: "undo_c"},
	}
	for _, c := range comps {
		if err := store.RegisterCompensation(context.Background(), runID, c); err != nil {
			t.Fatalf("register: %v", err)
		}
	}

	plans := store.RegisteredCompensations(runID)
	if len(plans) != 3 {
		t.Fatalf("expected 3 compensations, got %d", len(plans))
	}
}

func TestSagaStore_ClearRun(t *testing.T) {
	store := NewSagaStore(nil)
	runID := uuid.New()

	_ = store.RegisterCompensation(context.Background(), runID, SagaCompensation{
		StepKey: "test", CompensateKey: "undo_test",
	})
	store.ClearRun(runID)

	plans := store.RegisteredCompensations(runID)
	if plans != nil {
		t.Fatal("expected nil after ClearRun")
	}
}

func TestSagaStore_RegisteredCompensations_NoRun(t *testing.T) {
	store := NewSagaStore(nil)
	plans := store.RegisteredCompensations(uuid.New())
	if plans != nil {
		t.Fatal("expected nil for unknown run")
	}
}

func TestSagaExecutor_ExecuteCompensations_ReverseOrder(t *testing.T) {
	executed := []string{}
	runID := uuid.New()

	exec := &SagaExecutor{
		Logger: slog.Default(),
		RunCompensate: func(_ context.Context, _ uuid.UUID, stepKey string, _ json.RawMessage) error {
			executed = append(executed, stepKey)
			return nil
		},
	}

	plan := []SagaCompensation{
		{StepKey: "step_a", CompensateKey: "undo_a"},
		{StepKey: "step_b", CompensateKey: "undo_b"},
		{StepKey: "step_c", CompensateKey: ""},
	}
	completed := []string{"step_a", "step_b", "step_c"}

	err := exec.ExecuteCompensations(context.Background(), runID, completed, plan)
	if err != nil {
		t.Fatalf("ExecuteCompensations: %v", err)
	}

	if len(executed) != 2 {
		t.Fatalf("expected 2 compensations executed, got %d: %v", len(executed), executed)
	}
	if executed[0] != "undo_b" {
		t.Fatalf("expected first compensation undo_b, got %s", executed[0])
	}
	if executed[1] != "undo_a" {
		t.Fatalf("expected second compensation undo_a, got %s", executed[1])
	}
}

func TestSagaExecutor_ExecuteCompensations_ParallelExplicit(t *testing.T) {
	mu := make(chan string, 10)
	exec := &SagaExecutor{
		Logger:               slog.Default(),
		CompensateInParallel: true,
		RunCompensate: func(_ context.Context, _ uuid.UUID, stepKey string, _ json.RawMessage) error {
			time.Sleep(50 * time.Millisecond)
			mu <- stepKey
			return nil
		},
	}

	plan := []SagaCompensation{
		{StepKey: "step_a", CompensateKey: "undo_a"},
		{StepKey: "step_b", CompensateKey: "undo_b"},
		{StepKey: "step_c", CompensateKey: "undo_c"},
	}
	completed := []string{"step_a", "step_b", "step_c"}

	start := time.Now()
	err := exec.ExecuteCompensations(context.Background(), uuid.New(), completed, plan)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ExecuteCompensations: %v", err)
	}
	if duration > 150*time.Millisecond {
		t.Fatalf("parallel execution took %v, expected ~50ms", duration)
	}
	close(mu)
	count := 0
	for range mu {
		count++
	}
	if count != 3 {
		t.Fatalf("expected 3 parallel compensations, got %d", count)
	}
}

func TestSagaExecutor_ExecuteCompensations_ParallelWithCleanupIsSequential(t *testing.T) {
	order := []string{}
	exec := &SagaExecutor{
		Logger:               slog.Default(),
		CompensateInParallel: true,
		RunCompensate: func(_ context.Context, _ uuid.UUID, stepKey string, _ json.RawMessage) error {
			order = append(order, stepKey)
			return nil
		},
	}

	plan := []SagaCompensation{
		{StepKey: "step_a", CompensateKey: "undo_a", RetryPolicy: RetryRequireCleanup},
		{StepKey: "step_b", CompensateKey: "undo_b"},
	}
	completed := []string{"step_a", "step_b"}

	_ = exec.ExecuteCompensations(context.Background(), uuid.New(), completed, plan)
	if len(order) != 2 || order[0] != "undo_b" || order[1] != "undo_a" {
		t.Fatalf("expected sequential reverse order undo_b, undo_a, got %v", order)
	}
}

func TestSagaExecutor_ExecuteCompensations_NoCompensateSteps(t *testing.T) {
	executed := 0
	exec := &SagaExecutor{
		Logger: slog.Default(),
		RunCompensate: func(_ context.Context, _ uuid.UUID, _ string, _ json.RawMessage) error {
			executed++
			return nil
		},
	}

	plan := []SagaCompensation{
		{StepKey: "step_a", CompensateKey: ""},
		{StepKey: "step_b", CompensateKey: ""},
	}
	completed := []string{"step_a", "step_b"}

	err := exec.ExecuteCompensations(context.Background(), uuid.New(), completed, plan)
	if err != nil {
		t.Fatalf("ExecuteCompensations: %v", err)
	}
	if executed != 0 {
		t.Fatalf("expected 0 executed, got %d", executed)
	}
}

func TestSagaExecutor_ExecuteCompensations_EmptyPlan(t *testing.T) {
	exec := &SagaExecutor{Logger: slog.Default()}
	err := exec.ExecuteCompensations(context.Background(), uuid.New(), []string{"a", "b"}, nil)
	if err != nil {
		t.Fatalf("expected no error for empty plan, got %v", err)
	}
}

func TestSagaExecutor_CompensationFailedContinues(t *testing.T) {
	executed := []string{}
	exec := &SagaExecutor{
		Logger: slog.Default(),
		RunCompensate: func(_ context.Context, _ uuid.UUID, stepKey string, _ json.RawMessage) error {
			executed = append(executed, stepKey)
			if stepKey == "undo_a" {
				return errors.New("compensation failed")
			}
			return nil
		},
	}

	plan := []SagaCompensation{
		{StepKey: "step_a", CompensateKey: "undo_a", MaxRetries: 1},
		{StepKey: "step_b", CompensateKey: "undo_b", MaxRetries: 1},
	}
	completed := []string{"step_a", "step_b"}

	err := exec.ExecuteCompensations(context.Background(), uuid.New(), completed, plan)
	if err != nil {
		t.Fatalf("expected no error (best effort), got %v", err)
	}
	if len(executed) != 2 {
		t.Fatalf("expected both compensations, got %v", executed)
	}
}

func TestSagaExecutor_RetryIdempotent_SuccessOnSecondAttempt(t *testing.T) {
	attempts := 0
	exec := &SagaExecutor{
		Logger: slog.Default(),
		RunCompensate: func(_ context.Context, _ uuid.UUID, _ string, _ json.RawMessage) error {
			attempts++
			if attempts < 2 {
				return errors.New("transient error")
			}
			return nil
		},
	}

	comp := SagaCompensation{
		StepKey:       "test",
		CompensateKey: "undo_test",
		MaxRetries:    3,
		RetryPolicy:   RetryIdempotent,
	}

	err := exec.runCompensateWithRetry(context.Background(), uuid.New(), comp)
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestSagaExecutor_RetryRequireCleanup_NoRetry(t *testing.T) {
	attempts := 0
	exec := &SagaExecutor{
		Logger: slog.Default(),
		RunCompensate: func(_ context.Context, _ uuid.UUID, _ string, _ json.RawMessage) error {
			attempts++
			return errors.New("fatal error")
		},
	}

	comp := SagaCompensation{
		StepKey:       "test",
		CompensateKey: "undo_test",
		MaxRetries:    5,
		RetryPolicy:   RetryRequireCleanup,
	}

	err := exec.runCompensateWithRetry(context.Background(), uuid.New(), comp)
	if err == nil {
		t.Fatal("expected error for require-cleanup")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (no retry), got %d", attempts)
	}
}

func TestSagaExecutor_RunCompensateNil(t *testing.T) {
	exec := &SagaExecutor{Logger: slog.Default()}
	err := exec.runCompensateWithRetry(context.Background(), uuid.New(), SagaCompensation{
		StepKey: "test", CompensateKey: "undo_test",
	})
	if err == nil {
		t.Fatal("expected error when RunCompensate is nil")
	}
}



func TestSagaExecutor_DefaultMaxRetries(t *testing.T) {
	attempts := 0
	exec := &SagaExecutor{
		Logger: slog.Default(),
		RunCompensate: func(_ context.Context, _ uuid.UUID, _ string, _ json.RawMessage) error {
			attempts++
			return errors.New("fail")
		},
	}

	comp := SagaCompensation{
		StepKey:       "test",
		CompensateKey: "undo_test",
		MaxRetries:    0,
	}

	err := exec.runCompensateWithRetry(context.Background(), uuid.New(), comp)
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts with default MaxRetries, got %d", attempts)
	}
}

func TestSagaStore_ToCompensationLog(t *testing.T) {
	payload := json.RawMessage(`{"user_id": 42}`)
	log := CompensationLog{
		ID:            1,
		RunID:         uuid.New(),
		OriginalStep:  "create_user",
		CompensateRan: "delete_user",
		Success:       true,
		Payload:       payload,
		ExecutedAt:    time.Now(),
	}
	if log.Success != true {
		t.Fatal("expected success")
	}
	if string(log.Payload) != `{"user_id": 42}` {
		t.Fatal("payload mismatch")
	}
}

func TestSagaCompensation_JSONRoundTrip(t *testing.T) {
	comp := SagaCompensation{
		StepKey:       "charge_card",
		CompensateKey: "refund_card",
		MaxRetries:    5,
		RetryPolicy:   RetryIdempotent,
	}
	data, err := json.Marshal(comp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded SagaCompensation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.StepKey != "charge_card" || decoded.CompensateKey != "refund_card" {
		t.Fatal("roundtrip mismatch")
	}
	if decoded.RetryPolicy != RetryIdempotent {
		t.Fatalf("expected RetryIdempotent, got %s", decoded.RetryPolicy)
	}
}
