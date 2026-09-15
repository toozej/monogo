package providers

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/toozej/monogo/pkg/urlsafe"
)

// publicClient permits HTTPS requests to public addresses only. Keep this
// client private so callers cannot replace its transport or redirect policy.
// OAuth token exchanges use this transport as well as discovery requests.
func publicClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	d := publicDialer{lookup: net.DefaultResolver.LookupIPAddr, dial: dialer.DialContext}
	return &http.Client{
		Timeout:       20 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &publicTransport{transport: &http.Transport{
			Proxy:       nil,
			DialContext: d.dialContext,
		}},
	}
}

type publicTransport struct {
	transport *http.Transport
}

func (t *publicTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := httpsURL(req.URL.String()); err != nil {
		return nil, err
	}
	return t.transport.RoundTrip(req)
}

func (t *publicTransport) CloseIdleConnections() { t.transport.CloseIdleConnections() }

type publicDialer struct {
	lookup func(context.Context, string) ([]net.IPAddr, error)
	dial   func(context.Context, string, string) (net.Conn, error)
}

func (d publicDialer) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := d.lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("provider address is unavailable")
	}
	for _, ip := range ips {
		if ip.Zone != "" || urlsafe.IsInternalIP(ip.IP) {
			return nil, errors.New("provider address is not public")
		}
	}
	// Dial the checked IP, not the hostname. A second DNS lookup could return
	// an internal address. The transport retains the hostname for TLS checks.
	return d.dial(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

func (a *Adapter) do(req *http.Request) (*http.Response, error) {
	if err := httpsURL(req.URL.String()); err != nil {
		return nil, err
	}
	// codeql[go/request-forgery] publicClient rejects redirects and proxies.
	// Its transport rejects non-public DNS answers and dials only the checked IP.
	return a.http.Do(req)
}
