package systemcron

import (
	"context"
	"testing"
	"time"
)

func TestNextWeekly_NextMondayFromMidweek(t *testing.T) {
	// Miercoles 2026-06-24 10:00 -> proximo lunes 2026-06-29 03:00.
	from := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	got := nextWeekly(from, time.Monday, 3)
	want := time.Date(2026, 6, 29, 3, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("nextWeekly: got %v want %v", got, want)
	}
}

func TestNextWeekly_SameDayBeforeHour(t *testing.T) {
	// Lunes 2026-06-29 01:00 (antes de las 3) -> hoy 03:00.
	from := time.Date(2026, 6, 29, 1, 0, 0, 0, time.UTC)
	got := nextWeekly(from, time.Monday, 3)
	want := time.Date(2026, 6, 29, 3, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("nextWeekly same-day pre-hour: got %v want %v", got, want)
	}
}

func TestNextWeekly_SameDayAfterHourSkipsAWeek(t *testing.T) {
	// Lunes 2026-06-29 05:00 (despues de las 3) -> lunes siguiente 2026-07-06 03:00.
	from := time.Date(2026, 6, 29, 5, 0, 0, 0, time.UTC)
	got := nextWeekly(from, time.Monday, 3)
	want := time.Date(2026, 7, 6, 3, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("nextWeekly same-day post-hour: got %v want %v", got, want)
	}
}

func TestNextWeekly_AlwaysFuture(t *testing.T) {
	from := time.Now()
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		got := nextWeekly(from, wd, 3)
		if !got.After(from) {
			t.Fatalf("weekday %v: nextWeekly %v no es futuro respecto a %v", wd, got, from)
		}
		if got.Weekday() != wd {
			t.Fatalf("weekday %v: nextWeekly cayo en %v", wd, got.Weekday())
		}
		if got.Hour() != 3 {
			t.Fatalf("weekday %v: nextWeekly hora %d, esperaba 3", wd, got.Hour())
		}
	}
}

// TestSkillJudge_NilAggregatorDegrades: sin Aggregator inyectado, Start sale
// limpio sin panic (degradacion).
func TestSkillJudge_NilAggregatorDegrades(t *testing.T) {
	j := &SkillJudge{} // Aggregator nil
	done := make(chan struct{})
	go func() {
		j.Start(context.Background())
		close(done)
	}()
	select {
	case <-done:
		// ok: retorno inmediato
	case <-time.After(time.Second):
		t.Fatal("Start con Aggregator nil debe retornar de inmediato")
	}
}
