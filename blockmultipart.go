// Package blockmultipart temporarily rejects multipart requests using headers only.
package blockmultipart

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// Config declares upload exceptions and the proxies allowed to report client IPs.
type Config struct {
	AllowedIPs     []string `json:"allowedIPs,omitempty"`
	TrustedProxies []string `json:"trustedProxies,omitempty"`
}

// CreateConfig returns the middleware configuration.
func CreateConfig() *Config { return &Config{} }

// BlockMultipart rejects multipart Content-Type values on every HTTP method.
type BlockMultipart struct {
	next           http.Handler
	allowedIPs     []*net.IPNet
	trustedProxies []*net.IPNet
}

// New constructs the middleware without reading any request body.
func New(_ context.Context, next http.Handler, config *Config, _ string) (http.Handler, error) {
	allowed, err := parseRanges(config.AllowedIPs)
	if err != nil {
		return nil, fmt.Errorf("allowedIPs: %w", err)
	}
	proxies, err := parseRanges(config.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("trustedProxies: %w", err)
	}
	return &BlockMultipart{next: next, allowedIPs: allowed, trustedProxies: proxies}, nil
}

// ServeHTTP checks every Content-Type header value, including duplicate headers.
func (m *BlockMultipart) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	for _, value := range req.Header.Values("Content-Type") {
		if strings.Contains(strings.ToLower(value), "multipart") {
			if contains(m.allowedIPs, m.clientIP(req)) {
				m.next.ServeHTTP(rw, req)
				return
			}
			http.Error(rw, "multipart requests are temporarily disabled", http.StatusForbidden)
			return
		}
	}
	m.next.ServeHTTP(rw, req)
}

func parseRanges(values []string) ([]*net.IPNet, error) {
	ranges := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if ip := net.ParseIP(value); ip != nil {
			bits := 128
			if ip.To4() != nil {
				ip, bits = ip.To4(), 32
			}
			ranges = append(ranges, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, subnet, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("invalid IP address or CIDR %q", value)
		}
		ranges = append(ranges, subnet)
	}
	return ranges, nil
}

func contains(ranges []*net.IPNet, ip net.IP) bool {
	for _, subnet := range ranges {
		if subnet.Contains(ip) {
			return true
		}
	}
	return false
}

func (m *BlockMultipart) clientIP(req *http.Request) net.IP {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return nil
	}
	peer := net.ParseIP(host)
	if !contains(m.trustedProxies, peer) {
		return peer
	}
	// Only a trusted immediate peer may supply a forwarding chain. Walk from the
	// nearest hop, stopping at the first untrusted address; never trust the leftmost
	// value supplied by a client. Missing/malformed chains grant no exception.
	forwarded := strings.Split(strings.Join(req.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(forwarded) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(forwarded[i]))
		if ip == nil {
			return nil
		}
		if !contains(m.trustedProxies, ip) {
			return ip
		}
	}
	return nil
}
