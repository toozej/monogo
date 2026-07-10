package cmd

import "testing"

func TestIsLoopbackHost(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		if !isLoopbackHost(host) {
			t.Fatalf("expected %q to be loopback", host)
		}
	}
	for _, host := range []string{"0.0.0.0", "192.168.1.2", "example.com"} {
		if isLoopbackHost(host) {
			t.Fatalf("expected %q to require authentication", host)
		}
	}
}
