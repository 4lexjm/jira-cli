package auth

import (
	"fmt"
	"net/http"
	"os"

	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/credentials"
	"github.com/jcmturner/gokrb5/v8/spnego"
)

// KerberosTransport is an http.RoundTripper that negotiates Kerberos/SPNEGO
// authentication (HTTP Negotiate scheme) against a Jira Data Center instance.
//
// It reads credentials from the current user's Kerberos credential cache
// (populated by `kinit` or automatically on domain-joined workstations).
// No credentials are stored by the CLI — the Kerberos ticket cache is the
// single source of truth, managed by the OS or `kinit`.
//
// On expiry the Kerberos library transparently renews the ticket if the TGT
// is still valid; otherwise the user must re-run `kinit`.
type KerberosTransport struct {
	// inner is the decorated transport (e.g. with retry / debug logging).
	inner http.RoundTripper
	// krb5Client is the SPNEGO-aware Kerberos client.
	krb5Client *client.Client
}

// loadKrb5Config attempts to load krb5.conf from KRB5_CONFIG env var, standard
// system locations, or returns an initialized default config if none is found.
func loadKrb5Config() (*config.Config, error) {
	if cfgPath := os.Getenv("KRB5_CONFIG"); cfgPath != "" {
		return config.Load(cfgPath)
	}
	for _, path := range []string{"/etc/krb5.conf", "/etc/krb5/krb5.conf"} {
		if _, err := os.Stat(path); err == nil {
			return config.Load(path)
		}
	}
	return config.New(), nil
}

// NewKerberosTransport creates a KerberosTransport by loading the current
// user's Kerberos credential cache from the system's default ccache location
// (controlled by the KRB5CCNAME environment variable or the system default,
// e.g. /tmp/krb5cc_<uid> on Linux or the in-memory cache on macOS).
func NewKerberosTransport(inner http.RoundTripper) (*KerberosTransport, error) {
	if inner == nil {
		inner = http.DefaultTransport
	}

	krb5Conf, err := loadKrb5Config()
	if err != nil {
		return nil, fmt.Errorf("load krb5.conf: %w", err)
	}

	// Load the credential cache. Uses KRB5CCNAME env var if set,
	// otherwise falls back to the OS default cache location.
	ccache, err := credentials.LoadCCache("")
	if err != nil {
		return nil, fmt.Errorf(
			"load Kerberos credential cache: %w\n"+
				"Hint: run `kinit <user>@<REALM>` to obtain a ticket, "+
				"or `klist` to inspect the current cache", err)
	}

	krb5Client, err := client.NewFromCCache(ccache, krb5Conf,
		client.DisablePAFXFAST(true))
	if err != nil {
		return nil, fmt.Errorf("create Kerberos client from ccache: %w", err)
	}

	return &KerberosTransport{inner: inner, krb5Client: krb5Client}, nil
}

// RoundTrip implements http.RoundTripper. It wraps the underlying transport
// with SPNEGO negotiation: on a 401 Negotiate challenge the library
// transparently re-sends the request with the `Authorization: Negotiate <token>`
// header derived from the Kerberos TGS ticket for the target service.
func (t *KerberosTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	wrapped := spnego.NewClient(t.krb5Client, &http.Client{Transport: t.inner}, "")

	// Build a shallow copy of the request so we can safely set headers.
	clone := req.Clone(req.Context())

	resp, err := wrapped.Do(clone)
	if err != nil {
		if isTicketExpired(err) {
			return nil, &KerberosExpiredError{cause: err}
		}
		return nil, fmt.Errorf("SPNEGO round-trip: %w", err)
	}
	return resp, nil
}

// KerberosExpiredError is returned when the Kerberos TGT has expired.
type KerberosExpiredError struct{ cause error }

func (e *KerberosExpiredError) Error() string {
	return fmt.Sprintf(
		"Kerberos ticket expired: %v\n"+
			"Run `kinit <user>@<REALM>` to renew your ticket, then retry.", e.cause)
}

func (e *KerberosExpiredError) Unwrap() error { return e.cause }

// isTicketExpired performs a best-effort check on the error message returned
// by the gokrb5 library when the credential cache is expired.
func isTicketExpired(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, phrase := range []string{"expired", "Ticket expired", "KRB_AP_ERR_TKT_EXPIRED"} {
		for i := 0; i+len(phrase) <= len(msg); i++ {
			if msg[i:i+len(phrase)] == phrase {
				return true
			}
		}
	}
	return false
}

// CheckKerberosTicket performs a lightweight validation of the current
// Kerberos credential cache and returns a human-readable status.
func CheckKerberosTicket() (principal string, err error) {
	ccache, err := credentials.LoadCCache("")
	if err != nil {
		return "", fmt.Errorf(
			"no Kerberos credentials found: %w\n"+
				"Run `kinit <user>@<REALM>` to obtain a ticket", err)
	}
	creds := ccache.GetClientCredentials()
	if creds == nil {
		return "", fmt.Errorf("credential cache is empty — run `kinit`")
	}
	return creds.DisplayName(), nil
}
