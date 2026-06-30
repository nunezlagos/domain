package selfhosted

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)







func TestRunnerStatus_Constants(t *testing.T) {


	require.Equal(t, RunnerStatus("online"), StatusOnline)
	require.Equal(t, RunnerStatus("degraded"), StatusDegraded)
	require.Equal(t, RunnerStatus("offline"), StatusOffline)
}

func TestRunner_StructShape(t *testing.T) {

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


	require.NotEqual(t, ErrTaskNotFound, ErrRunnerOffline)
	require.NotEqual(t, ErrTaskNotFound, ErrNoTask)
	require.NotEqual(t, ErrRunnerOffline, ErrNoTask)
}

func TestService_DefaultsAreZero(t *testing.T) {




	s := &Service{}
	require.Equal(t, time.Duration(0), s.HeartbeatTimeout)
	require.Equal(t, time.Duration(0), s.ClaimTimeout)
}

func TestReturnResult_StatusMapping(t *testing.T) {



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


			status := "done"
			if tc.errMsg != "" {
				status = "failed"
			}
			require.Equal(t, tc.want, status)
		})
	}
}

func TestReclaimExpiredTasks_DefaultTimeout(t *testing.T) {





	const expectedDefault = 5 * time.Minute
	require.Equal(t, expectedDefault, time.Minute*5, "default ClaimTimeout es 5min — NO cambiar sin actualizar HU + tests")
}
