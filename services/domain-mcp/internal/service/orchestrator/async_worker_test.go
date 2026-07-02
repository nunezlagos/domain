package orchestrator

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/anthropic"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// asyncFixtureRepo extiende el fake base con lo que ProcessAsyncFlowRun
// necesita: template con modelo MiniMax y sincronización para lecturas
// seguras desde el test (el worker procesa en goroutine).
type asyncFixtureRepo struct {
	multiConcernRepo
	mu   sync.Mutex
	done chan struct{} // se cierra al UpdateFlowRunStatus (fin del flow)
}

func (r *asyncFixtureRepo) GetAgentTemplate(_ context.Context, _ uuid.UUID, slug string) (*AgentTemplate, error) {
	return &AgentTemplate{Slug: slug, Model: anthropic.MiniMaxModel, SystemPrompt: "sys"}, nil
}

func (r *asyncFixtureRepo) MarkStepCompleted(_ context.Context, stepID uuid.UUID, _ map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completedID = stepID
	return nil
}

func (r *asyncFixtureRepo) UpdateFlowRunStatus(_ context.Context, _ uuid.UUID, status string) error {
	r.mu.Lock()
	r.updatedStatusTo = status
	r.mu.Unlock()
	if r.done != nil {
		select {
		case <-r.done: // ya cerrado
		default:
			close(r.done)
		}
	}
	return nil
}

func (r *asyncFixtureRepo) snapshot() (uuid.UUID, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.completedID, r.updatedStatusTo
}

// newAsyncFixture arma un Service + repo con UN flow async pendiente de una
// fase sdd-verify con prompt cacheado, y un provider fake bajo "minimax".
func newAsyncFixture(t *testing.T, providerResp string) (*Service, *asyncFixtureRepo, uuid.UUID) {
	t.Helper()
	flowRunID := uuid.New()
	stepID := uuid.New()
	step := &FlowRunStepRow{
		ID:        stepID,
		FlowRunID: flowRunID,
		StepKey:   "sdd-verify",
		Status:    "pending",
		Inputs:    map[string]any{"user_prompt": "valida los escenarios"},
	}
	repo := &asyncFixtureRepo{
		multiConcernRepo: multiConcernRepo{
			flowRun: &FlowRunRow{
				ID:             flowRunID,
				OrganizationID: uuid.New(),
				ProjectID:      uuid.New(),
				Status:         "running",
				Cursor:         map[string]any{"mode": "async"},
			},
			step:     step,
			allSteps: []FlowRunStepRow{*step},
		},
		done: make(chan struct{}),
	}
	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDVerifyHandler())
	svc := New(nil, nil, reg, "test")
	svc.Repo = repo
	f := llm.NewFactory()
	f.Register(anthropic.MiniMaxProviderName, &fakeProvider{resp: providerResp})
	svc.LLM = f
	return svc, repo, flowRunID
}

func TestProcessAsyncFlowRun_HappyPath_CompletesFlow(t *testing.T) {
	t.Parallel()
	svc, repo, flowRunID := newAsyncFixture(t, `{"scenarios_failed": []}`)

	err := svc.ProcessAsyncFlowRun(context.Background(), flowRunID)
	require.NoError(t, err)

	completedID, status := repo.snapshot()
	require.Equal(t, repo.step.ID, completedID, "el step debe completarse")
	require.Equal(t, "completed", status, "el flow debe terminar completed")
}

func TestProcessAsyncFlowRun_NotAsync_Rejected(t *testing.T) {
	t.Parallel()
	svc, repo, flowRunID := newAsyncFixture(t, `{}`)
	repo.flowRun.Cursor = map[string]any{"mode": "full"} // NO async

	err := svc.ProcessAsyncFlowRun(context.Background(), flowRunID)
	require.ErrorIs(t, err, ErrAsyncFlowNotAsync,
		"el ejecutor async jamás debe procesar flows no-async")
}

func TestRunAsyncWorker_NoLLM_DisabledImmediately(t *testing.T) {
	t.Parallel()
	svc := New(nil, nil, phases.NewRegistry(), "test")
	svc.Repo = &asyncFixtureRepo{}
	// Sin LLM: debe retornar de inmediato (no colgarse en el ticker).
	doneCh := make(chan struct{})
	go func() {
		svc.RunAsyncWorker(context.Background(), AsyncWorkerConfig{Logger: slog.Default()})
		close(doneCh)
	}()
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("RunAsyncWorker sin LLM debería deshabilitarse y retornar")
	}
}

func TestRunAsyncWorker_RepoWithoutClaimer_DisabledImmediately(t *testing.T) {
	t.Parallel()
	svc := New(nil, nil, phases.NewRegistry(), "test")
	svc.LLM = llm.NewFactory()
	svc.Repo = &multiConcernRepo{} // NO implementa AsyncFlowClaimer
	doneCh := make(chan struct{})
	go func() {
		svc.RunAsyncWorker(context.Background(), AsyncWorkerConfig{Logger: slog.Default()})
		close(doneCh)
	}()
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("RunAsyncWorker sin claimer debería deshabilitarse y retornar")
	}
}

// asyncClaimOnce es un claimer que entrega un único flow y después nada.
type asyncClaimOnce struct {
	*asyncFixtureRepo
	mu       sync.Mutex
	claimed  bool
	workerID string
}

func (c *asyncClaimOnce) ClaimNextPendingAsyncFlow(_ context.Context, workerID string) (uuid.UUID, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.claimed {
		return uuid.Nil, false, nil
	}
	c.claimed = true
	c.workerID = workerID
	return c.flowRun.ID, true, nil
}

func TestDrainAsyncFlows_ClaimsProcessesAndStops(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newAsyncFixture(t, `{"scenarios_failed": []}`)
	claimer := &asyncClaimOnce{asyncFixtureRepo: repo}
	svc.Repo = claimer // el worker usa el mismo repo, ahora con claim

	sem := make(chan struct{}, 1)
	svc.drainAsyncFlows(context.Background(), claimer, sem, "test-worker", slog.Default())

	// Esperar a que la goroutine del flow termine (cierra done al actualizar status).
	select {
	case <-repo.done:
	case <-time.After(3 * time.Second):
		t.Fatal("el flow claimed nunca se procesó")
	}
	_, status := repo.snapshot()
	require.Equal(t, "completed", status)

	claimer.mu.Lock()
	require.True(t, claimer.claimed)
	require.Equal(t, "test-worker", claimer.workerID)
	claimer.mu.Unlock()

	// Segundo drain: no hay pendientes → no debe lanzar nada ni colgarse.
	svc.drainAsyncFlows(context.Background(), claimer, sem, "test-worker", slog.Default())
}
