// Package urlsafe validates outbound-request URLs to mitigate server-side
// request forgery (SSRF). It rejects non-HTTP(S) schemes and, unless explicitly
// allowed, hostnames that resolve to non-global, private/unique-local, or shared
// address-space ranges (which include cloud metadata endpoints such as
// 169.254.169.254 and 100.100.100.200).
//
// Validation resolves the hostname and inspects the returned IPs. This is a
// point-in-time check and does not by itself defend against DNS rebinding
// (TOCTOU between validation and the actual dial); callers needing that
// guarantee should additionally pin or re-check the dialed address.
package urlsafe

import (
	"errors"
	"fmt"
	"net"
	"net/url"
)

// ErrUnsafeURL identifies URL syntax, scheme, host, and address-policy
// rejections. Resolution and other transient failures do not wrap this error,
// allowing callers to distinguish permanent policy failures from retryable
// network errors.
var ErrUnsafeURL = errors.New("unsafe URL")

var specialUseNetworks = mustNetworks(
	"100.64.0.0/10", // shared address space
	"192.0.0.0/24",  // IETF protocol assignments
	"192.0.2.0/24",  // documentation
	"198.18.0.0/15", // benchmarking
	"198.51.100.0/24",
	"203.0.113.0/24",
	"240.0.0.0/4",   // reserved
	"2001:db8::/32", // documentation
)

func mustNetworks(cidrs ...string) []*net.IPNet {
	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		networks = append(networks, network)
	}
	return networks
}

// Validate reports whether rawURL is safe to request. It requires an http or
// https scheme and a resolvable hostname. Unless allowPrivate is true, it
// rejects hostnames resolving to any non-global, private, or shared-space
// address, blocking SSRF against internal/metadata services.
func Validate(rawURL string, allowPrivate bool) error {
	if rawURL == "" {
		return fmt.Errorf("%w: URL cannot be empty", ErrUnsafeURL)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: invalid URL format: %v", ErrUnsafeURL, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: only HTTP and HTTPS schemes are allowed, got: %q", ErrUnsafeURL, u.Scheme)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("%w: no hostname found in URL", ErrUnsafeURL)
	}

	if allowPrivate {
		return nil
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %w", hostname, err)
	}

	for _, ip := range ips {
		if IsInternalIP(ip) {
			return fmt.Errorf("%w: requests to private/internal IP addresses are not allowed: %s resolves to %s", ErrUnsafeURL, hostname, ip.String())
		}
	}

	return nil
}

// IsInternalIP reports whether ip is non-public or belongs to a special-use
// range that user-supplied outbound requests must not reach.
func IsInternalIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return true
	}
	for _, network := range specialUseNetworks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
