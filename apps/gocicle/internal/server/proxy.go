package server

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func (s *Server) proxyRequest(r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return
	}
	trusted := func(address netip.Addr) bool {
		for _, value := range s.Config.TrustedProxies {
			prefix, err := netip.ParsePrefix(value)
			if err == nil && prefix.Contains(address) {
				return true
			}
		}
		return false
	}
	if !trusted(address) {
		r.Header.Del("X-Forwarded-For")
		r.Header.Del("X-Forwarded-Proto")
		return
	}
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(forwarded) - 1; i >= 0; i-- {
		candidate, err := netip.ParseAddr(strings.TrimSpace(forwarded[i]))
		if err != nil {
			return
		}
		address = candidate
		if !trusted(address) {
			break
		}
	}
	r.RemoteAddr = net.JoinHostPort(address.String(), "0")
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		r.URL.Scheme = "https"
	}
}
