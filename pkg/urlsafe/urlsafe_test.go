package urlsafe

import (
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name         string
		rawURL       string
		allowPrivate bool
		wantErr      bool
		wantUnsafe   bool
	}{
		// Accepted: public literal IPs (literal IPs skip real DNS, keeping the test offline).
		{name: "public ipv4", rawURL: "https://8.8.8.8/feed.xml", wantErr: false},
		{name: "public ipv4 http", rawURL: "http://1.1.1.1/", wantErr: false},
		{name: "public ipv6", rawURL: "https://[2606:4700:4700::1111]/", wantErr: false},

		// Rejected schemes.
		{name: "empty", rawURL: "", wantErr: true, wantUnsafe: true},
		{name: "ftp scheme", rawURL: "ftp://example.com/x", wantErr: true, wantUnsafe: true},
		{name: "file scheme", rawURL: "file:///etc/passwd", wantErr: true, wantUnsafe: true},
		{name: "no host", rawURL: "http:///path", wantErr: true, wantUnsafe: true},

		// Rejected: private / internal / metadata targets.
		{name: "loopback v4", rawURL: "http://127.0.0.1/", wantErr: true},
		{name: "loopback name-ish v4", rawURL: "http://127.0.0.53:8080/x", wantErr: true},
		{name: "private 10", rawURL: "http://10.0.0.1/", wantErr: true},
		{name: "private 172.16", rawURL: "http://172.16.5.4/", wantErr: true},
		{name: "private 192.168", rawURL: "https://192.168.1.1/admin", wantErr: true},
		{name: "link-local metadata", rawURL: "http://169.254.169.254/latest/meta-data/", wantErr: true},
		{name: "shared address space metadata", rawURL: "http://100.100.100.200/latest/meta-data/", wantErr: true},
		{name: "multicast v4", rawURL: "http://224.0.0.1/", wantErr: true},
		{name: "unspecified v4", rawURL: "http://0.0.0.0/", wantErr: true},
		{name: "loopback v6", rawURL: "http://[::1]/", wantErr: true},
		{name: "unique-local v6", rawURL: "http://[fc00::1]/", wantErr: true},
		{name: "link-local v6", rawURL: "http://[fe80::1]/", wantErr: true},
		{name: "multicast v6", rawURL: "http://[ff02::1]/", wantErr: true},
		{name: "shared address space", rawURL: "http://100.64.0.1/", wantErr: true},
		{name: "benchmark network", rawURL: "http://198.18.0.1/", wantErr: true},
		{name: "reserved", rawURL: "http://240.0.0.1/", wantErr: true},
		{name: "documentation ipv6", rawURL: "http://[2001:db8::1]/", wantErr: true},

		// Opt-out allows private targets through (but still enforces scheme).
		{name: "private allowed", rawURL: "http://192.168.1.1/admin", allowPrivate: true, wantErr: false},
		{name: "loopback allowed", rawURL: "http://127.0.0.1:9000/", allowPrivate: true, wantErr: false},
		{name: "bad scheme not allowed even with opt-out", rawURL: "ftp://127.0.0.1/", allowPrivate: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.rawURL, tt.allowPrivate)
			if tt.wantErr && err == nil {
				t.Fatalf("Validate(%q, %v) = nil, want error", tt.rawURL, tt.allowPrivate)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate(%q, %v) = %v, want nil", tt.rawURL, tt.allowPrivate, err)
			}
			if tt.wantUnsafe && !errors.Is(err, ErrUnsafeURL) {
				t.Fatalf("Validate(%q, %v) error = %v, want ErrUnsafeURL", tt.rawURL, tt.allowPrivate, err)
			}
		})
	}
}
