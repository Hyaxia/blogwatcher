package httputil

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// MaxBodySize is the default maximum response body size (10 MB).
const MaxBodySize int64 = 10 * 1024 * 1024

const maxRedirects = 5

// AllowPrivate disables private/reserved IP blocking. This should only be set
// to true in tests that use httptest.Server on localhost.
var AllowPrivate bool

// privateRanges contains CIDR blocks for private/reserved IP ranges.
var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
	}
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("httputil: bad CIDR %q: %v", cidr, err))
		}
		privateRanges = append(privateRanges, network)
	}
}

// ValidateURL checks that a URL uses http/https and does not resolve to a
// private or reserved IP address.
func ValidateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	switch parsed.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", parsed.Scheme)
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL has no hostname")
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %q: %w", hostname, err)
	}

	if !AllowPrivate {
		for _, ip := range ips {
			if isPrivateIP(ip) {
				return fmt.Errorf("URL resolves to private/reserved address %s", ip)
			}
		}
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	for _, network := range privateRanges {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// SafeClient returns an *http.Client with the given timeout that validates
// redirect targets against SSRF, limiting redirects to 5.
func SafeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if err := ValidateURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect blocked: %w", err)
			}
			return nil
		},
	}
}

// LimitBody wraps a reader to cap the number of bytes read to MaxBodySize.
func LimitBody(r io.Reader) io.Reader {
	return io.LimitReader(r, MaxBodySize)
}
