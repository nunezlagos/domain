package backpressure

import (
	"errors"
	"testing"
)

func TestRetryAfterSeconds(t *testing.T) {
	if got := RetryAfterSeconds(ErrQueueFull); got != 60 {
		t.Fatalf("queue_full = %d, want 60", got)
	}
	if got := RetryAfterSeconds(ErrOrgQuotaExceeded); got != 5 {
		t.Fatalf("org quota = %d, want 5", got)
	}
	if got := RetryAfterSeconds(errors.New("other")); got != 30 {
		t.Fatalf("other = %d, want 30 default", got)
	}
}

func TestPredefinedQueues_Catalog(t *testing.T) {
	for _, name := range []string{"agent_runs", "flow_runs", "outbound_webhook_deliveries"} {
		q, ok := PredefinedQueues[name]
		if !ok {
			t.Fatalf("missing predefined queue %s", name)
		}
		if q.Table == "" || q.PendingCondition == "" || q.OrgColumn == "" {
			t.Fatalf("queue %s incomplete: %+v", name, q)
		}
		if q.GlobalCap == 0 && q.PerOrgCap == 0 {
			t.Fatalf("queue %s has no caps", name)
		}
	}
}
