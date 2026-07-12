package cmd

import "testing"

func TestIsLoopbackHost(t *testing.T) {
	for _, host := range []string{"localhost", "LOCALHOST", "localhost.", "127.0.0.1", "::1"} {
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

func TestCheckPortAvailabilitySupportsIPv6Loopback(t *testing.T) {
	if got, want := bindAddress("::1", 8080), "[::1]:8080"; got != want {
		t.Fatalf("bindAddress() = %q, want %q", got, want)
	}
	command := ServeCommand{Host: "::1", Port: 0}
	if err := command.checkPortAvailability(); err != nil {
		t.Skipf("IPv6 loopback is unavailable: %v", err)
	}
}
