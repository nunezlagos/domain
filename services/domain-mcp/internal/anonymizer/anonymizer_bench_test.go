package anonymizer

import "testing"

func BenchmarkFakerRUT(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FakerRUT(42, i)
	}
}

func BenchmarkFakerEmail(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FakerEmail(42, i)
	}
}

func BenchmarkRedactJSON_Small(b *testing.B) {
	in := []byte(`{"email":"a@b.c","name":"Ana","phone":"+56000"}`)
	keys := DefaultSensitiveJSONKeys
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RedactJSON(in, keys)
	}
}

func BenchmarkRedactJSON_Nested(b *testing.B) {
	in := []byte(`{"user":{"email":"x@y.z","profile":{"phone":"+56","name":"Ana"}},"tags":["a","b"]}`)
	keys := DefaultSensitiveJSONKeys
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RedactJSON(in, keys)
	}
}
