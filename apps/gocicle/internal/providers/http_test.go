package providers

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPublicDialerRejectsUnsafeAnswers(t *testing.T) {
	for _, address := range []string{
		"0.0.0.0", "127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.0.1",
		"169.254.169.254", "100.100.100.200", "192.0.0.1", "192.0.2.1",
		"198.18.0.1", "198.51.100.1", "203.0.113.1", "224.0.0.1", "240.0.0.1",
		"::", "::1", "fc00::1", "fe80::1", "ff02::1", "2001:db8::1",
		"::ffff:127.0.0.1", "::ffff:169.254.169.254",
	} {
		t.Run(address, func(t *testing.T) {
			// Reject the whole answer even when the first address is public.
			d := publicDialer{
				lookup: func(context.Context, string) ([]net.IPAddr, error) {
					return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP(address)}}, nil
				},
				dial: func(context.Context, string, string) (net.Conn, error) {
					t.Fatal("an unsafe DNS answer reached the dialer")
					return nil, nil
				},
			}
			if _, err := d.dialContext(context.Background(), "tcp", "provider.example:443"); err == nil {
				t.Fatal("an unsafe DNS answer was accepted")
			}
		})
	}
}

func TestPublicDialerPinsAddressAndRechecksNewConnections(t *testing.T) {
	for _, address := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		t.Run(address, func(t *testing.T) {
			lookups, dials := 0, 0
			dialErr := errors.New("test dial result")
			d := publicDialer{
				lookup: func(_ context.Context, host string) ([]net.IPAddr, error) {
					lookups++
					if host != "provider.example" {
						t.Fatalf("unexpected hostname: %s", host)
					}
					if lookups > 1 {
						return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
					}
					return []net.IPAddr{{IP: net.ParseIP(address)}}, nil
				},
				dial: func(_ context.Context, network, target string) (net.Conn, error) {
					dials++
					if network != "tcp" || target != net.JoinHostPort(address, "8443") {
						t.Fatalf("the dialer did not use the checked address: %s %s", network, target)
					}
					return nil, dialErr
				},
			}
			if _, err := d.dialContext(context.Background(), "tcp", "provider.example:8443"); !errors.Is(err, dialErr) {
				t.Fatalf("unexpected dial error: %v", err)
			}
			if lookups != 1 || dials != 1 {
				t.Fatalf("the connection did not use one DNS lookup: lookups=%d dials=%d", lookups, dials)
			}
			if _, err := d.dialContext(context.Background(), "tcp", "provider.example:8443"); err == nil || errors.Is(err, dialErr) {
				t.Fatalf("the next connection accepted a rebound address: %v", err)
			}
			if lookups != 2 || dials != 1 {
				t.Fatalf("the rebound address reached the dialer: lookups=%d dials=%d", lookups, dials)
			}
		})
	}
}

func TestPublicDialerRejectsMissingAnswersAndZones(t *testing.T) {
	for name, answers := range map[string][]net.IPAddr{
		"empty":  nil,
		"nil IP": {{IP: nil}},
		"zone":   {{IP: net.ParseIP("2606:4700:4700::1111"), Zone: "eth0"}},
	} {
		t.Run(name, func(t *testing.T) {
			d := publicDialer{
				lookup: func(context.Context, string) ([]net.IPAddr, error) { return answers, nil },
				dial: func(context.Context, string, string) (net.Conn, error) {
					t.Fatal("an invalid DNS answer reached the dialer")
					return nil, nil
				},
			}
			if _, err := d.dialContext(context.Background(), "tcp", "provider.example:443"); err == nil {
				t.Fatal("an invalid DNS answer was accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := publicDialer{lookup: func(ctx context.Context, _ string) ([]net.IPAddr, error) { return nil, ctx.Err() }}
	if _, err := d.dialContext(ctx, "tcp", "provider.example:443"); !errors.Is(err, context.Canceled) {
		t.Fatalf("DNS cancellation was lost: %v", err)
	}
}

func TestProviderRequestsRejectUnsafeURLs(t *testing.T) {
	for _, target := range []string{
		"http://provider.example/token", "ftp://provider.example/token", "https:opaque",
		"https://user:password@provider.example/token", "https://provider.example/token#fragment", "https://:443/token",
	} {
		t.Run(target, func(t *testing.T) {
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
			if err != nil {
				t.Fatal(err)
			}
			a := &Adapter{http: &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
				t.Fatal("an unsafe URL reached the HTTP client")
				return nil, nil
			})}}
			if _, err := a.do(req); err == nil {
				t.Fatal("an unsafe URL was accepted")
			}
			// The transport also guards requests made by the OAuth library.
			transport := &publicTransport{}
			if _, err := transport.RoundTrip(req); err == nil {
				t.Fatal("the OAuth transport accepted an unsafe URL")
			}
		})
	}
}

func TestPublicClientPinsTLSRequestsAndRejectsRedirects(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", r.URL.Query().Get("target"))
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()
	client := publicClient()
	defer client.CloseIdleConnections()
	transport := providerTestTransport(t, client)
	transport.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	if transport.Proxy != nil {
		t.Fatal("the provider client permits a proxy to bypass its address checks")
	}
	// Only this test dialer maps a checked public address to the TLS fixture.
	d := publicDialer{
		lookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
		},
		dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(address)
			if err != nil || host != "8.8.8.8" {
				t.Errorf("the transport did not pin its checked IP: %s", address)
				return nil, errors.New("unexpected dial address")
			}
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		},
	}
	transport.DialContext = d.dialContext
	a := &Adapter{http: client}
	var result struct{ OK bool }
	if err := a.request(context.Background(), http.MethodGet, server.URL, "", nil, &result); err != nil || !result.OK {
		t.Fatalf("the checked HTTPS request failed: %v", err)
	}
	for _, target := range []string{"https://127.0.0.1/private", "https://169.254.169.254/metadata", "http://provider.example", "https://other.example"} {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/redirect", nil)
		if err != nil {
			t.Fatal(err)
		}
		query := req.URL.Query()
		query.Set("target", target)
		req.URL.RawQuery = query.Encode()
		before := hits.Load()
		resp, err := a.do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusTemporaryRedirect || hits.Load() != before+1 {
			t.Fatalf("the provider client followed a redirect to %s", target)
		}
	}
	// The hostname must still pass certificate validation after IP pinning.
	if err := a.request(context.Background(), http.MethodGet, "https://wrong.example", "", nil, &result); err == nil {
		t.Fatal("IP pinning disabled TLS hostname verification")
	}
}

func providerTestTransport(t *testing.T, client *http.Client) *http.Transport {
	t.Helper()
	guard, ok := client.Transport.(*publicTransport)
	if !ok {
		t.Fatal("the provider client has no public-address guard")
	}
	return guard.transport
}

func TestATProtocolRejectsInternalDIDHost(t *testing.T) {
	a, err := New("tangled", Config{}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.http.CloseIdleConnections()
	for _, did := range []string{"did:web:127.0.0.1", "did:web:169.254.169.254"} {
		if _, _, err := a.Begin(context.Background(), "state", did); err == nil || !strings.Contains(err.Error(), "provider request failed") {
			t.Fatalf("internal DID host was not blocked: %s: %v", did, err)
		}
	}
}
