package etag

import (
	"testing"
	"time"
)

func BenchmarkCompute(b *testing.B) {
	now := time.Now().UTC()
	id := "a3f9b1d0-0000-4000-8000-000000000001"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Compute(id, now)
	}
}

func BenchmarkLastModified(b *testing.B) {
	now := time.Now().UTC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = LastModified(now)
	}
}
