package usage

import (
	"errors"
	"testing"
	"time"
)

func TestService_DayWindow_AlignsToUTCMidnight(t *testing.T) {
	cases := []struct {
		name      string
		now       time.Time
		wantStart string
		wantEnd   string
	}{
		{
			name:      "noon UTC",
			now:       time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC),
			wantStart: "2026-06-13T00:00:00Z",
			wantEnd:   "2026-06-14T00:00:00Z",
		},
		{
			name:      "near midnight UTC",
			now:       time.Date(2026, 6, 13, 23, 59, 59, 0, time.UTC),
			wantStart: "2026-06-13T00:00:00Z",
			wantEnd:   "2026-06-14T00:00:00Z",
		},
		{
			name:      "input non-UTC tz gets normalized",
			now:       time.Date(2026, 6, 13, 23, 0, 0, 0, time.FixedZone("CLT", -4*3600)),
			wantStart: "2026-06-14T00:00:00Z", // 23:00 -04 == 03:00 UTC next day
			wantEnd:   "2026-06-15T00:00:00Z",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Service{Now: func() time.Time { return tc.now }}
			start, end := s.dayWindow()
			if got := start.Format(time.RFC3339); got != tc.wantStart {
				t.Fatalf("start = %s, want %s", got, tc.wantStart)
			}
			if got := end.Format(time.RFC3339); got != tc.wantEnd {
				t.Fatalf("end = %s, want %s", got, tc.wantEnd)
			}
			if end.Sub(start) != 24*time.Hour {
				t.Fatalf("window = %s, want 24h", end.Sub(start))
			}
		})
	}
}

func TestService_Defaults(t *testing.T) {
	s := &Service{}
	if got := s.rateLimit(); got != defaultRateLimitPerMin {
		t.Fatalf("rateLimit default = %d, want %d", got, defaultRateLimitPerMin)
	}
	if got := s.maxFlowDuration(); got != defaultMaxFlowDuration {
		t.Fatalf("maxFlowDuration default = %d, want %d", got, defaultMaxFlowDuration)
	}

	s2 := &Service{DefaultRateLimitPerMin: 500, DefaultMaxFlowDuration: 120}
	if got := s2.rateLimit(); got != 500 {
		t.Fatalf("rateLimit override = %d, want 500", got)
	}
	if got := s2.maxFlowDuration(); got != 120 {
		t.Fatalf("maxFlowDuration override = %d, want 120", got)
	}
}

// TestHistory_DaysValidation cubre los límites del parámetro days sin tocar BD:
// - days=0 → service usa default 7 (lo valida internamente)
// - days<0 → ErrInvalidDays
// - days>365 → ErrInvalidDays
// La query real no se ejecuta porque la validación corta antes.
func TestHistory_DaysValidation(t *testing.T) {
	s := &Service{}

	cases := []struct {
		name    string
		days    int
		wantErr error
	}{
		{"negative", -1, ErrInvalidDays},
		{"over max", 366, ErrInvalidDays},
		{"way over", 100000, ErrInvalidDays},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			_, err := s.History(nil, fixedUUID, tc.days) //nolint:staticcheck // nil ctx ok: cortamos en validation
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
