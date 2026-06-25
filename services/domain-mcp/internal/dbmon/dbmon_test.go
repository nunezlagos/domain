package dbmon

import "testing"

func TestEvaluate_IdleInTransaction(t *testing.T) {
	s := &Snapshot{Connections: ConnectionStats{IdleInTransaction: 15}}
	alerts := Evaluate(s)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Code != "idle_in_transaction_high" {
		t.Fatalf("code: %s", alerts[0].Code)
	}
}

func TestEvaluate_LongRunningQuery(t *testing.T) {
	s := &Snapshot{Connections: ConnectionStats{LongestQuerySeconds: 120}}
	alerts := Evaluate(s)
	if len(alerts) != 1 || alerts[0].Code != "long_running_query" {
		t.Fatalf("alerts: %+v", alerts)
	}
}

func TestEvaluate_LockWait(t *testing.T) {
	s := &Snapshot{Locks: LockStats{LongestWaitSeconds: 6}}
	alerts := Evaluate(s)
	if len(alerts) != 1 || alerts[0].Code != "lock_wait_high" {
		t.Fatalf("alerts: %+v", alerts)
	}
}

func TestEvaluate_DeadTuples(t *testing.T) {
	s := &Snapshot{Tables: []TableStats{{
		Name: "observations", LiveTuples: 10000, DeadTuples: 6000, DeadRatio: 0.6,
	}}}
	alerts := Evaluate(s)
	if len(alerts) != 1 || alerts[0].Code != "dead_tuples_high" {
		t.Fatalf("alerts: %+v", alerts)
	}
}

func TestEvaluate_NoAlertsCuandoSano(t *testing.T) {
	s := &Snapshot{
		Connections: ConnectionStats{Active: 3, Idle: 5, IdleInTransaction: 0, LongestQuerySeconds: 1.2},
		Locks:       LockStats{WaitingCount: 0, LongestWaitSeconds: 0},
		Tables: []TableStats{{
			Name: "users", LiveTuples: 10000, DeadTuples: 50, DeadRatio: 0.005,
		}},
	}
	if alerts := Evaluate(s); len(alerts) != 0 {
		t.Fatalf("expected 0 alerts in healthy snapshot, got %+v", alerts)
	}
}

func TestEvaluate_DeadTuples_IgnoraTablasPequeñas(t *testing.T) {

	s := &Snapshot{Tables: []TableStats{{
		Name: "small_table", LiveTuples: 50, DeadTuples: 100, DeadRatio: 2.0,
	}}}
	if alerts := Evaluate(s); len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for small table, got %+v", alerts)
	}
}
