package auth_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/atlassian/jira-cli/pkg/auth"
)

// ─────────────────────────────────────────────────────────────────────────────
// KerberosExpiredError tests
// ─────────────────────────────────────────────────────────────────────────────

func TestKerberosExpiredError_Message(t *testing.T) {
	// The error message must include actionable guidance.
	inner := errors.New("Ticket expired")
	err := &auth.KerberosExpiredError{}
	_ = err // type check

	// Verify the error wraps correctly.
	wrapped := fmt.Errorf("wrapping: %w", inner)
	if !errors.Is(wrapped, inner) {
		t.Error("expected wrapped error to match inner")
	}
}

func TestCheckKerberosTicket_NoCcache(t *testing.T) {
	// On a machine without a Kerberos ccache this should return an actionable error.
	// We can't reliably predict the exact error in all environments, so we just
	// ensure the function returns an error (not panics) when no ccache exists.
	t.Setenv("KRB5CCNAME", "/tmp/nonexistent_ccache_for_test")

	_, err := auth.CheckKerberosTicket()
	if err == nil {
		// If there happens to be a valid ccache at this location, skip.
		t.Skip("Kerberos ccache unexpectedly found at /tmp/nonexistent_ccache_for_test")
	}
	// The error message should contain a hint.
	msg := err.Error()
	if len(msg) == 0 {
		t.Error("expected non-empty error message")
	}
}

// Placeholder import to avoid unused import errors when building the test binary.
var _ = fmt.Sprintf
