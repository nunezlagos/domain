package cursor

import (
	"testing"
	"time"
)

// issue-27.4 benchmarks de cursor encode/decode.
//
//   go test -bench=. -benchmem ./internal/api/cursor/

func BenchmarkCursorEncode(b *testing.B) {
	c := Cursor{
		LastID:        "a3f9b1d0-0000-4000-8000-000000000001",
		LastSortValue: time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC),
		FiltersHash:   "deadbeefcafebabe",
		SortDir:       "desc",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Encode()
	}
}

func BenchmarkCursorDecode(b *testing.B) {
	c := Cursor{
		LastID:        "a3f9b1d0-0000-4000-8000-000000000001",
		LastSortValue: time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC),
		FiltersHash:   "deadbeefcafebabe",
		SortDir:       "desc",
	}
	enc := c.Encode()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Decode(enc, "deadbeefcafebabe", "desc")
	}
}

func BenchmarkHashFilters_3keys(b *testing.B) {
	in := map[string]string{
		"project_slug": "my-project",
		"org":          "00000000-0000-0000-0000-000000000001",
		"tag":          "production",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HashFilters(in)
	}
}
