package starter

import (
	"io"
	"testing"
)

func BenchmarkRun(b *testing.B) {
	username := "benchmark-user"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run(io.Discard, username)
	}
}

func BenchmarkRunWithEmptyString(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run(io.Discard, "")
	}
}
