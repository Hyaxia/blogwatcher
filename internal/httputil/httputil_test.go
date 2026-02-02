package httputil

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestValidateURL_BlocksPrivateIPs(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1",
		"http://127.0.0.1:8080",
		"http://10.0.0.1",
		"http://172.16.0.1",
		"http://192.168.1.1",
		"http://169.254.169.254", // cloud metadata
		"http://[::1]",
	}
	for _, u := range blocked {
		if err := ValidateURL(u); err == nil {
			t.Errorf("ValidateURL(%q) should have been blocked", u)
		}
	}
}

func TestValidateURL_AllowsPublicURLs(t *testing.T) {
	allowed := []string{
		"https://example.com",
		"http://example.com/feed.xml",
		"https://example.org/rss",
	}
	for _, u := range allowed {
		if err := ValidateURL(u); err != nil {
			t.Errorf("ValidateURL(%q) should be allowed, got: %v", u, err)
		}
	}
}

func TestValidateURL_BlocksNonHTTPSchemes(t *testing.T) {
	blocked := []string{
		"ftp://example.com",
		"file:///etc/passwd",
		"gopher://example.com",
		"javascript:alert(1)",
	}
	for _, u := range blocked {
		err := ValidateURL(u)
		if err == nil {
			t.Errorf("ValidateURL(%q) should have been blocked", u)
			continue
		}
		if !strings.Contains(err.Error(), "unsupported URL scheme") {
			t.Errorf("ValidateURL(%q) error should mention scheme, got: %v", u, err)
		}
	}
}

func TestValidateURL_RejectsEmptyHostname(t *testing.T) {
	err := ValidateURL("http://")
	if err == nil {
		t.Error("ValidateURL with empty hostname should fail")
	}
}

func TestSafeClient_ReturnsClientWithTimeout(t *testing.T) {
	client := SafeClient(5 * time.Second)
	if client == nil {
		t.Fatal("SafeClient returned nil")
	}
	if client.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", client.Timeout)
	}
	if client.CheckRedirect == nil {
		t.Error("SafeClient should set CheckRedirect")
	}
}

func TestLimitBody_LimitsRead(t *testing.T) {
	data := strings.NewReader(strings.Repeat("a", 100))
	limited := LimitBody(data)

	buf := make([]byte, 200)
	n, _ := limited.Read(buf)
	if n != 100 {
		t.Errorf("expected to read 100 bytes, got %d", n)
	}
}

func TestIsPrivateIP_Coverage(t *testing.T) {
	tests := []struct {
		addr    string
		private bool
	}{
		{"127.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.0", true},
		{"172.31.255.255", true},
		{"172.32.0.0", false},
		{"192.168.0.1", true},
		{"169.254.1.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.addr)
		if ip == nil {
			t.Fatalf("failed to parse IP %q", tt.addr)
		}
		got := isPrivateIP(ip)
		if got != tt.private {
			t.Errorf("isPrivateIP(%s) = %v, want %v", tt.addr, got, tt.private)
		}
	}
}
