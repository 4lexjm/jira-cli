// Package httpx provides a Jira-aware HTTP client with pluggable authentication,
// retry logic, ETag caching, and debug logging.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	jiracfg "github.com/atlassian/jira-cli/internal/config"
	"github.com/atlassian/jira-cli/internal/secret"
	"github.com/atlassian/jira-cli/pkg/auth"
)

const (
	defaultUserAgent   = "jira-cli/dev"
	defaultTimeout     = 30 * time.Second
	defaultMaxAttempts = 3
	defaultInitBackoff = 500 * time.Millisecond
	defaultMaxBackoff  = 10 * time.Second
)

// Client is a Jira-aware HTTP client. It selects the appropriate auth transport
// based on the Host.AuthMethod and attaches it to every outgoing request.
type Client struct {
	baseURL   *url.URL
	http      *http.Client
	userAgent string
	debug     bool

	retry struct {
		MaxAttempts    int
		InitialBackoff time.Duration
		MaxBackoff     time.Duration
	}

	// rate limit tracking
	rateMu sync.RWMutex
	rate   RateLimit
}

// RateLimit captures the last observed X-RateLimit-* headers.
type RateLimit struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

// New creates a Client configured for the given host.
// It selects the auth transport based on host.AuthMethod:
//
//   - "kerberos"       → Kerberos/SPNEGO (Option B)
//   - "session-cookie" → JSESSIONID cookie from keychain (Option C)
//   - "bearer"         → Authorization: Bearer <token>
//   - "basic" (default)→ Authorization: Basic <user:token>
func New(host *jiracfg.Host) (*Client, error) {
	baseURL, err := url.Parse(host.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL %q: %w", host.BaseURL, err)
	}

	transport, err := buildTransport(host)
	if err != nil {
		return nil, err
	}

	c := &Client{
		baseURL:   baseURL,
		userAgent: defaultUserAgent,
		debug:     os.Getenv("JIRA_HTTP_DEBUG") == "1",
		http: &http.Client{
			Transport: transport,
			Timeout:   defaultTimeout,
			// Do not follow redirects automatically — we need to detect SSO
			// redirects in the session-cookie transport before they are consumed.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	c.retry.MaxAttempts = defaultMaxAttempts
	c.retry.InitialBackoff = defaultInitBackoff
	c.retry.MaxBackoff = defaultMaxBackoff
	return c, nil
}

// buildTransport selects the right http.RoundTripper for host.AuthMethod.
func buildTransport(host *jiracfg.Host) (http.RoundTripper, error) {
	base := http.DefaultTransport

	switch host.AuthMethod {
	case jiracfg.AuthMethodKerberos:
		// Option B — Kerberos/SPNEGO.
		// Reads from the current user's Kerberos credential cache.
		// No credential is stored by the CLI.
		t, err := auth.NewKerberosTransport(base)
		if err != nil {
			return nil, fmt.Errorf("kerberos transport: %w", err)
		}
		return t, nil

	case jiracfg.AuthMethodSessionCookie:
		// Option C — JSESSIONID session cookie.
		// The cookie is retrieved from the OS keychain (stored by `jira auth login --browser`).
		// Env var JIRA_SESSION_COOKIE overrides the keychain (useful for CI).
		hostKey := host.BaseURL
		getCookie := func() (string, error) {
			// 1. Environment variable takes priority (CI/headless override).
			if v := secret.EnvSessionCookie(); v != "" {
				// Allow "JSESSIONID=xxx" or bare "xxx" formats.
				return stripCookieName(v), nil
			}
			// 2. OS keychain.
			val, err := secret.Get(secret.SessionKey(hostKey))
			if err != nil {
				return "", auth.ErrSessionNotFound
			}
			return val, nil
		}
		return auth.NewSessionTransport(base, host.BaseURL, getCookie), nil

	case jiracfg.AuthMethodBearer:
		token := resolveToken(host)
		return &bearerTransport{inner: base, token: token}, nil

	case jiracfg.AuthMethodBasicFallback:
		// Basic auth with ?os_authType=basic appended to bypass SAML.
		token := resolveToken(host)
		return &basicFallbackTransport{inner: base, username: host.Username, token: token}, nil

	default:
		// Basic auth (dc default when username is set, cloud with email+token).
		token := resolveToken(host)
		user := host.Username
		if user == "" {
			user = host.Email
		}
		return &basicTransport{inner: base, username: user, token: token}, nil
	}
}

// resolveToken returns the token from env var → host struct (populated from keychain).
func resolveToken(host *jiracfg.Host) string {
	if v := secret.EnvToken(); v != "" {
		return v
	}
	return host.Token
}

// ─────────────────────────────────────────────────────────────────────────────
// Simple auth transports
// ─────────────────────────────────────────────────────────────────────────────

type basicTransport struct {
	inner    http.RoundTripper
	username string
	token    string
}

func (t *basicTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.SetBasicAuth(t.username, t.token)
	return t.inner.RoundTrip(clone)
}

type bearerTransport struct {
	inner http.RoundTripper
	token string
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return t.inner.RoundTrip(clone)
}

// basicFallbackTransport appends ?os_authType=basic to every request URL,
// bypassing Jira's SAML redirect for Basic Auth requests.
type basicFallbackTransport struct {
	inner    http.RoundTripper
	username string
	token    string
}

func (t *basicFallbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.SetBasicAuth(t.username, t.token)
	q := clone.URL.Query()
	q.Set("os_authType", "basic")
	clone.URL.RawQuery = q.Encode()
	return t.inner.RoundTrip(clone)
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP helpers
// ─────────────────────────────────────────────────────────────────────────────

// Do executes a request with retry logic.
func (c *Client) Do(req *http.Request, v any) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt < c.retry.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay(attempt, resp)
			timer := time.NewTimer(delay)
			select {
			case <-req.Context().Done():
				timer.Stop()
				return nil, req.Context().Err()
			case <-timer.C:
			}
		}

		resp, err = c.http.Do(req)
		if err != nil {
			if !isRetryable(attempt, c.retry.MaxAttempts, req.Method, 0) {
				return nil, err
			}
			continue
		}

		if c.debug {
			fmt.Fprintf(os.Stderr, "jira: %s %s → %d\n", req.Method, req.URL, resp.StatusCode)
		}

		if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && isIdempotent(req.Method)) {
			if !isRetryable(attempt, c.retry.MaxAttempts, req.Method, resp.StatusCode) {
				break
			}
			resp.Body.Close()
			continue
		}
		break
	}

	if err != nil {
		return nil, err
	}
	if v != nil && resp.StatusCode < 300 {
		defer resp.Body.Close()
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return resp, fmt.Errorf("read response body: %w", readErr)
		}
		if len(body) > 0 {
			if decErr := json.Unmarshal(body, v); decErr != nil {
				return resp, fmt.Errorf("decode response: %w", decErr)
			}
		}
	}
	return resp, nil
}

// NewRequest constructs an HTTP request against the Jira base URL.
func (c *Client) NewRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	u := *c.baseURL
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	rel, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse path %q: %w", path, err)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + rel.Path
	u.RawQuery = rel.RawQuery

	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		buf = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// Get is a convenience wrapper for GET requests that decodes JSON into v.
func (c *Client) Get(ctx context.Context, path string, v any) (*http.Response, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req, v)
}

// Post is a convenience wrapper for POST requests.
func (c *Client) Post(ctx context.Context, path string, body, v any) (*http.Response, error) {
	req, err := c.NewRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	return c.Do(req, v)
}

// Put is a convenience wrapper for PUT requests.
func (c *Client) Put(ctx context.Context, path string, body, v any) (*http.Response, error) {
	req, err := c.NewRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return nil, err
	}
	return c.Do(req, v)
}

// Delete is a convenience wrapper for DELETE requests.
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	req, err := c.NewRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req, nil)
}

// BaseURL returns the client's base URL string.
func (c *Client) BaseURL() string { return c.baseURL.String() }

// ─────────────────────────────────────────────────────────────────────────────
// Retry helpers
// ─────────────────────────────────────────────────────────────────────────────

func isRetryable(attempt, max int, method string, status int) bool {
	if attempt+1 >= max {
		return false
	}
	if status == http.StatusTooManyRequests {
		return true
	}
	return isIdempotent(method)
}

func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

func (c *Client) retryDelay(attempt int, resp *http.Response) time.Duration {
	delay := c.retry.InitialBackoff
	if attempt > 1 {
		delay *= time.Duration(1 << (attempt - 1))
	}
	if delay > c.retry.MaxBackoff {
		delay = c.retry.MaxBackoff
	}
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				d := time.Duration(secs) * time.Second
				if d > 60*time.Second {
					d = 60 * time.Second
				}
				if d > 0 {
					delay = d
				}
			}
		}
	}
	return delay
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// stripCookieName strips the "JSESSIONID=" prefix if present.
func stripCookieName(v string) string {
	const prefix = auth.SessionCookieName + "="
	if strings.HasPrefix(v, prefix) {
		return v[len(prefix):]
	}
	return v
}
