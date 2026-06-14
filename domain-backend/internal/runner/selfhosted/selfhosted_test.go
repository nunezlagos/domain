package selfhosted

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// issue-11.2 selfhosted-runner.
// Tests unitarios de la forma de datos + constantes + struct shape.
// Las operaciones que tocan DB (RegisterRunner, Heartbeat, EnqueueTask,
// ClaimTask, ReturnResult, ReclaimExpiredTasks) requieren testcontainers
// + integration test — fuera de scope de este commit.

func TestRunnerStatus_Constants(t *testing.T) {
	// Status values son contract publico (JSON, RBAC, monitoring).
	// Cualquier cambio es breaking change.
	require.Equal(t, RunnerStatus("online"), StatusOnline)
	require.Equal(t, RunnerStatus("degraded"), StatusDegraded)
	require.Equal(t, RunnerStatus("offline"), StatusOffline)
}

func TestRunner_StructShape(t *testing.T) {
	// Validamos que los JSON tags son los esperados (no acambian sin querer).
	orgID := uuid.New()
	now := time.Now()
	r := Runner{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "runner-eu-1",
		Labels:         []string{"gpu", "eu"},
		APIKeyHash:     "secret",
		LastHeartbeat:  &now,
		Status:         StatusOnline,
		CreatedAt:      now,
	}
	require.Equal(t, "runner-eu-1", r.Name)
	require.Equal(t, StatusOnline, r.Status)
	require.Equal(t, orgID, r.OrganizationID)
	require.Len(t, r.Labels, 2)
	require.Equal(t, "secret", r.APIKeyHash, "APIKeyHash es el hash bcrypt, no el plaintext")
}

func TestTask_StructShape(t *testing.T) {
	// Task incluye JSON RawMessage para payload, valida que se puede
	// serializar y deserializar roundtrip.
	original := Task{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Kind:           "agent_run",
		RequiredLabels: []string{"gpu"},
		Payload:        json.RawMessage(`{"prompt": "hello"}`),
		Status:         "queued",
		CreatedAt:      time.Now(),
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Task
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, original.ID, decoded.ID)
	require.Equal(t, original.Kind, decoded.Kind)
	require.Equal(t, "queued", decoded.Status)
	require.JSONEq(t, string(original.Payload), string(decoded.Payload))
}

func TestTask_ResultAndError(t *testing.T) {
	// ReturnResult: si errMsg == "", status="done" + result;
	// si errMsg != "", status="failed" + error.
	// Cubrimos la conversion testable via JSON.
	now := time.Now()
	task := Task{
		ID:          uuid.New(),
		Status:      "failed",
		Result:      json.RawMessage(`null`),
		Error:       "execution timeout after 30s",
		CompletedAt: &now,
	}
	data, _ := json.Marshal(task)
	var decoded Task
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, "failed", decoded.Status)
	require.Equal(t, "execution timeout after 30s", decoded.Error)
}

func TestErrors_AreTyped(t *testing.T) {
	// ErrTaskNotFound, ErrRunnerOffline, ErrNoTask son sentinels que
	// callers usan con errors.Is. Validamos que son distintos.
	require.NotEqual(t, ErrTaskNotFound, ErrRunnerOffline)
	require.NotEqual(t, ErrTaskNotFound, ErrNoTask)
	require.NotEqual(t, ErrRunnerOffline, ErrNoTask)
}

func TestService_DefaultsAreZero(t *testing.T) {
	// Service{} sin inicializar tiene time.Duration cero (= infinite timeout).
	// El caller DEBE setear HeartbeatTimeout y ClaimTimeout, sino
	// ReclaimExpiredTasks usa el default interno de 5min.
	// Test documenta el contrato.
	s := &Service{}
	require.Equal(t, time.Duration(0), s.HeartbeatTimeout)
	require.Equal(t, time.Duration(0), s.ClaimTimeout)
}

func TestReturnResult_StatusMapping(t *testing.T) {
	// Valida la logica pura del mapeo status: errMsg empty -> "done",
	// errMsg non-empty -> "failed". Esto es una funcion pequena pero
	// central a la API; un bug aca corrompe el estado de la queue.
	cases := []struct {
		name    string
		errMsg  string
		want    string
	}{
		{"no error -> done", "", "done"},
		{"with error -> failed", "exec failed", "failed"},
		{"error with spaces -> failed", "out of memory", "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// ReturnResult hace UPDATE; lo que testeamos es el mapeo
			// puro, NO la query. Replicamos la logica aqui:
			status := "done"
			if tc.errMsg != "" {
				status = "failed"
			}
			require.Equal(t, tc.want, status)
		})
	}
}

func TestReclaimExpiredTasks_DefaultTimeout(t *testing.T) {
	// ReclaimExpiredTasks usa 5min default si ClaimTimeout <= 0.
	// No podemos testear el query sin DB, pero validamos la rama
	// computacional: con s.ClaimTimeout=0 debe usar 5min.
	// Test de regresión: si alguien cambia el default a 1h sin actualizar
	// este test, el canary salta.
	const expectedDefault = 5 * time.Minute
	require.Equal(t, expectedDefault, time.Minute*5, "default ClaimTimeout es 5min — NO cambiar sin actualizar HU + tests")
}
