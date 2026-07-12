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
	"fmt"
	"net"
	"net/url"
)

// Validate reports whether rawURL is safe to request. It requires an http or
// https scheme and a resolvable hostname. Unless allowPrivate is true, it
// rejects hostnames resolving to any non-global, private, or shared-space
// address, blocking SSRF against internal/metadata services.
func Validate(rawURL string, allowPrivate bool) error {
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only HTTP and HTTPS schemes are allowed, got: %q", u.Scheme)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("no hostname found in URL")
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
			return fmt.Errorf("requests to private/internal IP addresses are not allowed: %s resolves to %s", hostname, ip.String())
		}
	}

	return nil
}

var sharedAddressSpace = &net.IPNet{
	IP:   net.IPv4(100, 64, 0, 0),
	Mask: net.CIDRMask(10, 32),
}

// IsInternalIP reports whether ip is in a range that must not be reachable
// from user-supplied URLs. In addition to non-global and private addresses, it
// rejects RFC 6598 shared address space; cloud providers can expose metadata
// endpoints there (for example 100.100.100.200).
func IsInternalIP(ip net.IP) bool {
	return !ip.IsGlobalUnicast() || ip.IsPrivate() || sharedAddressSpace.Contains(ip)
}
