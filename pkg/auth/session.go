package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/atlassian/jira-cli/pkg/browser"
)

// SessionCookieName is the cookie Jira sets after a successful login.
const SessionCookieName = "JSESSIONID"

// DefaultSessionTTL is the assumed session lifetime when Jira does not
// advertise an explicit expiry. Most Jira DC instances default to 8 hours.
const DefaultSessionTTL = 8 * time.Hour

// SessionExpiredError is returned when the JSESSIONID cookie has expired or
// been invalidated server-side (indicated by a 401 or SSO redirect).
type SessionExpiredError struct {
	Host string
}

func (e *SessionExpiredError) Error() string {
	return fmt.Sprintf(
		"Jira session expired for %s\n"+
			"Run `jira auth login %s --browser` to re-authenticate via SSO",
		e.Host, e.Host)
}

// SessionTransport is an http.RoundTripper that authenticates requests by
// attaching a JSESSIONID cookie obtained from a prior browser SSO login.
//
// # Lifecycle
//
//  1. The user runs `jira auth login <host> --browser`.
//  2. The CLI starts a local reverse-proxy on localhost:<port>.
//  3. The browser is directed to localhost:<port>/login.jsp.
//  4. The user completes SSO normally; Jira sets JSESSIONID in the response.
//  5. The reverse-proxy captures the Set-Cookie header and stores the value
//     in the OS keychain (never on disk).
//  6. Subsequent CLI calls attach `Cookie: JSESSIONID=<value>` to every request.
//  7. On expiry (401 or Location redirect to the IdP) the CLI surfaces a
//     SessionExpiredError with renewal instructions.
type SessionTransport struct {
	inner     http.RoundTripper
	host      string
	getCookie func() (string, error)
}

// NewSessionTransport creates a SessionTransport.
// getCookie is a function that returns the current JSESSIONID value;
// it is called lazily so the caller can refresh it from the keychain.
func NewSessionTransport(inner http.RoundTripper, host string, getCookie func() (string, error)) *SessionTransport {
	if inner == nil {
		inner = http.DefaultTransport
	}
	return &SessionTransport{inner: inner, host: host, getCookie: getCookie}
}

// RoundTrip implements http.RoundTripper.
// Attaches the JSESSIONID cookie and detects session expiry.
func (t *SessionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cookie, err := t.getCookie()
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, &SessionExpiredError{Host: t.host}
		}
		return nil, fmt.Errorf("retrieve session cookie: %w", err)
	}

	clone := req.Clone(req.Context())
	// Merge with any existing cookies rather than replacing the header.
	existing := clone.Header.Get("Cookie")
	cookieHeader := SessionCookieName + "=" + cookie
	if existing != "" {
		cookieHeader = existing + "; " + cookieHeader
	}
	clone.Header.Set("Cookie", cookieHeader)

	resp, err := t.inner.RoundTrip(clone)
	if err != nil {
		return nil, err
	}

	// Detect session expiry: Jira redirects to the IdP or returns 401.
	if isSessionExpired(resp) {
		resp.Body.Close()
		return nil, &SessionExpiredError{Host: t.host}
	}

	return resp, nil
}

// ErrSessionNotFound is returned by the cookie getter when no session is stored.
var ErrSessionNotFound = errors.New("no Jira session cookie stored")

// isSessionExpired returns true when the response indicates the session has
// been invalidated by the server.
func isSessionExpired(resp *http.Response) bool {
	if resp.StatusCode == http.StatusUnauthorized {
		return true
	}
	// Jira redirects to the SSO IdP when the session is invalid.
	// We detect this by checking for a Location header that points outside
	// the Jira host (i.e. to the IdP).
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		loc := resp.Header.Get("Location")
		if loc != "" && (strings.Contains(loc, "/login") || strings.Contains(loc, "saml") ||
			strings.Contains(loc, "sso") || strings.Contains(loc, "idp") ||
			strings.Contains(loc, "auth") || strings.Contains(loc, "oauth")) {
			return true
		}
	}
	return false
}

// ──────────────────────────────────────────────────────────────────────────────
// Browser-based session capture (Option C interactive login)
// ──────────────────────────────────────────────────────────────────────────────

// BrowserLoginResult holds the captured session after a browser SSO flow.
type BrowserLoginResult struct {
	Cookie     string    // JSESSIONID value
	CapturedAt time.Time // when the cookie was captured
}

// CaptureSessionViaBrowser starts a local reverse proxy, opens the user's
// browser to the Jira login page, and captures the JSESSIONID cookie that
// Jira sets after a successful SSO login.
//
// Flow:
//  1. Bind a random local port.
//  2. Start a reverse proxy that forwards all traffic to jiraBaseURL.
//  3. The proxy intercepts Set-Cookie headers from Jira responses and extracts
//     any JSESSIONID values.
//  4. Open the browser to http://localhost:<port>/login.jsp.
//  5. Block until a JSESSIONID is captured or ctx is cancelled.
//
// The JSESSIONID is typically set after the SSO assertion is posted back to
// Jira's SAML endpoint (/plugins/servlet/saml/auth). The reverse proxy sees
// all Jira HTTP responses (including the SAML consumer endpoint) and can
// capture the cookie from the Set-Cookie header regardless of the HttpOnly flag.
func CaptureSessionViaBrowser(ctx context.Context, jiraBaseURL string, b browser.Browser) (*BrowserLoginResult, error) {
	target, err := url.Parse(jiraBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Jira base URL: %w", err)
	}

	// Capture channel — buffered so the proxy goroutine never blocks.
	cookieCh := make(chan string, 1)

	// Build the reverse proxy.
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Override the Director to rewrite the Host header so Jira accepts the request.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		// Strip the proxy's own Cookie header so we don't confuse Jira.
		req.Header.Del("X-Forwarded-For")
	}

	// ModifyResponse intercepts every response from Jira and checks for JSESSIONID.
	proxy.ModifyResponse = func(resp *http.Response) error {
		for _, c := range resp.Cookies() {
			if c.Name == SessionCookieName && c.Value != "" {
				select {
				case cookieCh <- c.Value:
				default:
					// Already captured; ignore duplicates.
				}
			}
		}
		return nil
	}

	// Bind a random local port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("bind local port: %w", err)
	}
	localAddr := ln.Addr().String()

	srv := &http.Server{
		Handler:      proxy,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// Start the proxy server in the background.
	srvErrCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErrCh <- err
		}
	}()

	// Open the browser to the local proxy's login page.
	loginURL := "http://" + localAddr + "/login.jsp"
	fmt.Printf("Opening browser for SSO login...\n")
	fmt.Printf("If the browser does not open automatically, navigate to:\n  %s\n\n", loginURL)
	fmt.Printf("Complete the SSO login in the browser. The CLI will capture your session automatically.\n")

	if err := b.Open(loginURL); err != nil {
		// Non-fatal — user can open the URL manually.
		fmt.Printf("Could not open browser automatically: %v\n", err)
	}

	// Wait for the cookie or context cancellation.
	var result *BrowserLoginResult
	select {
	case cookie := <-cookieCh:
		result = &BrowserLoginResult{Cookie: cookie, CapturedAt: time.Now()}
	case err := <-srvErrCh:
		return nil, fmt.Errorf("proxy server error: %w", err)
	case <-ctx.Done():
		return nil, fmt.Errorf("login cancelled: %w", ctx.Err())
	}

	// Shut down the proxy server gracefully.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	return result, nil
}

// EstimatedExpiry returns the estimated expiry of a session captured at
// capturedAt, assuming DefaultSessionTTL.
func EstimatedExpiry(capturedAt time.Time) time.Time {
	return capturedAt.Add(DefaultSessionTTL)
}

// SessionAge returns how long ago the session was captured.
func SessionAge(capturedAt time.Time) time.Duration {
	return time.Since(capturedAt)
}

// SessionWarningThreshold is the remaining TTL below which the CLI warns
// that the session will expire soon.
const SessionWarningThreshold = 30 * time.Minute
