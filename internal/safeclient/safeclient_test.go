package safeclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSafeGet_PublicURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	// httptest.NewServer binds to 127.0.0.1, so we must AllowPrivate for the test to reach it.
	resp, err := SafeGet(server.URL, 2*time.Second, AllowPrivate())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSafeGet_BlocksLoopback(t *testing.T) {
	_, err := SafeGet("http://127.0.0.1/secret", 2*time.Second)
	if err == nil {
		t.Fatal("expected error for loopback address")
	}
	if !IsSSRFError(err) {
		t.Fatalf("expected SSRFError, got %T: %v", err, err)
	}
}

func TestSafeGet_BlocksIPv6Loopback(t *testing.T) {
	_, err := SafeGet("http://[::1]/secret", 2*time.Second)
	if err == nil {
		t.Fatal("expected error for IPv6 loopback")
	}
	if !IsSSRFError(err) {
		t.Fatalf("expected SSRFError, got %T: %v", err, err)
	}
}

func TestSafeGet_BlocksMetadataEndpoint(t *testing.T) {
	_, err := SafeGet("http://169.254.169.254/latest/meta-data/", 2*time.Second)
	if err == nil {
		t.Fatal("expected error for metadata endpoint")
	}
	if !IsSSRFError(err) {
		t.Fatalf("expected SSRFError, got %T: %v", err, err)
	}
}

func TestSafeGet_BlocksRFC1918(t *testing.T) {
	for _, addr := range []string{
		"http://10.0.0.1/",
		"http://172.16.0.1/",
		"http://192.168.1.1/",
	} {
		_, err := SafeGet(addr, 2*time.Second)
		if err == nil {
			t.Fatalf("expected error for private address %s", addr)
		}
		if !IsSSRFError(err) {
			t.Fatalf("expected SSRFError for %s, got %T: %v", addr, err, err)
		}
	}
}

func TestSafeGet_BlocksNonHTTPScheme(t *testing.T) {
	for _, u := range []string{
		"ftp://example.com/file",
		"file:///etc/passwd",
		"gopher://example.com",
	} {
		_, err := SafeGet(u, 2*time.Second)
		if err == nil {
			t.Fatalf("expected error for scheme in %s", u)
		}
		if !IsSSRFError(err) {
			t.Fatalf("expected SSRFError for %s, got %T: %v", u, err, err)
		}
	}
}

func TestSafeGet_AllowPrivateBypassesDenyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := SafeGet(server.URL, 2*time.Second, AllowPrivate())
	if err != nil {
		t.Fatalf("AllowPrivate should bypass deny list, got error: %v", err)
	}
	resp.Body.Close()
}

func TestIsSSRFError(t *testing.T) {
	err := &SSRFError{URL: "http://127.0.0.1", Message: "blocked"}
	if !IsSSRFError(err) {
		t.Fatal("expected IsSSRFError to return true")
	}

	if IsSSRFError(nil) {
		t.Fatal("expected IsSSRFError(nil) to return false")
	}
}
