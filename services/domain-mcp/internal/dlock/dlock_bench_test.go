package dlock

import "testing"

func BenchmarkHashKey(b *testing.B) {
	name := "cron_scheduler_org_00000000-0000-0000-0000-000000000001"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HashKey(name)
	}
}
