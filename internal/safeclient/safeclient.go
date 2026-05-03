package safeclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// denyList contains CIDR ranges that must never be contacted.
var denyList []*net.IPNet

// testAllowPrivate is a global flag that, when true, disables the IP deny-list
// for all SafeGet calls. It is set via SetTestAllowPrivate and intended for
// test infrastructure only (e.g. tests using httptest.NewServer on 127.0.0.1).
var testAllowPrivate bool

// SetTestAllowPrivate globally disables (or re-enables) the IP deny-list.
// This is intended for use in TestMain of packages that test against
// httptest.NewServer, which binds to loopback.
func SetTestAllowPrivate(allow bool) {
	testAllowPrivate = allow
}

func init() {
	for _, cidr := range []string{
		// IPv4
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // RFC 1918
		"172.16.0.0/12",  // RFC 1918
		"192.168.0.0/16", // RFC 1918
		"169.254.0.0/16", // link-local / cloud metadata
		"0.0.0.0/8",      // "this" network

		// IPv6
		"::1/128",   // loopback
		"fc00::/7",  // unique local
		"fe80::/10", // link-local
	} {
		_, network, _ := net.ParseCIDR(cidr)
		denyList = append(denyList, network)
	}
}

// SSRFError is returned when a URL resolves to a blocked IP range.
type SSRFError struct {
	URL     string
	Message string
}

func (e *SSRFError) Error() string {
	return fmt.Sprintf("ssrf blocked: %s — %s", e.URL, e.Message)
}

// IsSSRFError reports whether err (or any error in its chain) is an SSRFError.
func IsSSRFError(err error) bool {
	var target *SSRFError
	return errors.As(err, &target)
}

// Option configures SafeGet behaviour.
type Option func(*options)

type options struct {
	allowPrivate bool
}

// AllowPrivate disables the IP deny-list check.
// This is intended for use in tests only.
func AllowPrivate() Option {
	return func(o *options) { o.allowPrivate = true }
}

// SafeGet performs an HTTP GET after validating the URL against SSRF rules.
//
// It enforces:
//   - http or https scheme only
//   - hostname must not resolve to a private/reserved IP range
func SafeGet(rawURL string, timeout time.Duration, opts ...Option) (*http.Response, error) {
	cfg := &options{}
	for _, o := range opts {
		o(cfg)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, &SSRFError{URL: rawURL, Message: "malformed URL"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, &SSRFError{URL: rawURL, Message: fmt.Sprintf("unsupported scheme %q", parsed.Scheme)}
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return nil, &SSRFError{URL: rawURL, Message: "empty hostname"}
	}

	if !cfg.allowPrivate && !testAllowPrivate {
		if err := checkHost(hostname, timeout); err != nil {
			return nil, err
		}
	}

	client := &http.Client{Timeout: timeout}
	return client.Get(rawURL)
}

// checkHost resolves hostname and rejects it if every IP falls in the deny-list.
func checkHost(hostname string, timeout time.Duration) error {
	// Fast path: if hostname is already an IP literal, check it directly.
	if ip := net.ParseIP(hostname); ip != nil {
		if isBlocked(ip) {
			return &SSRFError{URL: hostname, Message: fmt.Sprintf("IP %s is in a blocked range", ip)}
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, hostname)
	if err != nil {
		return &SSRFError{URL: hostname, Message: fmt.Sprintf("DNS resolution failed: %v", err)}
	}
	if len(addrs) == 0 {
		return &SSRFError{URL: hostname, Message: "DNS returned no addresses"}
	}

	for _, addr := range addrs {
		if !isBlocked(addr.IP) {
			return nil // at least one public IP — allow
		}
	}
	return &SSRFError{URL: hostname, Message: "all resolved IPs are in blocked ranges"}
}

func isBlocked(ip net.IP) bool {
	for _, network := range denyList {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
