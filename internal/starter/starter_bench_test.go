package starter

import (
	"bytes"
	"os"
	"testing"
)

func BenchmarkRun(b *testing.B) {
	// Redirect stdout to avoid printing during benchmarks
	old := os.Stdout
	defer func() { os.Stdout = old }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	username := "benchmark-user"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Run(username)
	}

	w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
}

func BenchmarkRunWithEmptyString(b *testing.B) {
	// Redirect stdout to avoid printing during benchmarks
	old := os.Stdout
	defer func() { os.Stdout = old }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Run("")
	}

	w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
}
