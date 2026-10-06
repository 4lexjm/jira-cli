package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atlassian/jira-cli/pkg/auth"
)

// ─────────────────────────────────────────────────────────────────────────────
// SessionTransport tests (Option C)
// ─────────────────────────────────────────────────────────────────────────────

func TestSessionTransport_AttachesCookie(t *testing.T) {
	// Arrange: a server that echoes the Cookie header.
	var receivedCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	getCookie := func() (string, error) { return "MYSESSIONID123", nil }
	transport := auth.NewSessionTransport(http.DefaultTransport, srv.URL, getCookie)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/rest/api/2/myself", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// Assert: JSESSIONID is present in the Cookie header.
	if !strings.Contains(receivedCookie, "JSESSIONID=MYSESSIONID123") {
		t.Errorf("expected JSESSIONID cookie, got: %q", receivedCookie)
	}
}

func TestSessionTransport_DetectsExpiry_401(t *testing.T) {
	// Arrange: server that returns 401.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	getCookie := func() (string, error) { return "EXPIRED", nil }
	transport := auth.NewSessionTransport(http.DefaultTransport, srv.URL, getCookie)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	_, err := transport.RoundTrip(req)

	if err == nil {
		t.Fatal("expected SessionExpiredError, got nil")
	}
	var expErr *auth.SessionExpiredError
	if !isType(err, expErr) {
		t.Errorf("expected *SessionExpiredError, got %T: %v", err, err)
	}
}

func TestSessionTransport_DetectsExpiry_SSORedirect(t *testing.T) {
	// Arrange: server that redirects to the IdP (simulates SAML redirect).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://idp.example.com/saml/sso?RelayState=...", http.StatusFound)
	}))
	defer srv.Close()

	// Use a client that does NOT follow redirects (mirroring our httpx.Client behaviour).
	inner := &http.Transport{}
	getCookie := func() (string, error) { return "OLDSESSION", nil }
	transport := auth.NewSessionTransport(inner, srv.URL, getCookie)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/rest/api/2/issue/PROJ-1", nil)
	_, err := transport.RoundTrip(req)

	if err == nil {
		t.Fatal("expected SessionExpiredError for SSO redirect, got nil")
	}
}

func TestSessionTransport_MissingCookie_ReturnsSessionNotFound(t *testing.T) {
	getCookie := func() (string, error) { return "", auth.ErrSessionNotFound }
	transport := auth.NewSessionTransport(http.DefaultTransport, "https://jira.example.com", getCookie)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://jira.example.com/rest", nil)
	_, err := transport.RoundTrip(req)

	if err == nil {
		t.Fatal("expected error for missing session, got nil")
	}
	var expErr *auth.SessionExpiredError
	if !isType(err, expErr) {
		t.Errorf("expected *SessionExpiredError, got %T: %v", err, err)
	}
}

func TestSessionTransport_MergesExistingCookies(t *testing.T) {
	var receivedCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	getCookie := func() (string, error) { return "SESSION456", nil }
	transport := auth.NewSessionTransport(http.DefaultTransport, srv.URL, getCookie)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	req.Header.Set("Cookie", "existingCookie=abc")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// Both cookies must be present.
	if !strings.Contains(receivedCookie, "existingCookie=abc") {
		t.Errorf("existing cookie missing from header: %q", receivedCookie)
	}
	if !strings.Contains(receivedCookie, "JSESSIONID=SESSION456") {
		t.Errorf("JSESSIONID missing from header: %q", receivedCookie)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Session helpers tests
// ─────────────────────────────────────────────────────────────────────────────

func TestEstimatedExpiry(t *testing.T) {
	now := time.Now()
	expiry := auth.EstimatedExpiry(now)
	expected := now.Add(auth.DefaultSessionTTL)

	diff := expiry.Sub(expected)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("EstimatedExpiry = %v, want ~%v", expiry, expected)
	}
}

func TestSessionAge(t *testing.T) {
	capturedAt := time.Now().Add(-2 * time.Hour)
	age := auth.SessionAge(capturedAt)
	if age < 2*time.Hour-time.Second || age > 2*time.Hour+time.Second {
		t.Errorf("SessionAge = %v, want ~2h", age)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// isSessionExpired tests (via exported wrapper for testability)
// ─────────────────────────────────────────────────────────────────────────────

// isType is a helper for type assertion in tests without generics.
func isType(err error, target interface{}) bool {
	if err == nil {
		return false
	}
	switch target.(type) {
	case *auth.SessionExpiredError:
		_, ok := err.(*auth.SessionExpiredError)
		return ok
	case *auth.KerberosExpiredError:
		_, ok := err.(*auth.KerberosExpiredError)
		return ok
	}
	return false
}
