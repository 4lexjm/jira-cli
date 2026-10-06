// Package secret manages credential storage via OS keychain with file fallback.
package secret

import (
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

const serviceName = "jira-cli"

// ErrNotFound is returned when no credential exists for the given key.
var ErrNotFound = errors.New("credential not found")

// Set stores value for key in the OS keychain.
func Set(key, value string) error {
	if err := keyring.Set(serviceName, key, value); err != nil {
		return fmt.Errorf("keyring set %q: %w", key, err)
	}
	return nil
}

// Get retrieves the value for key from the OS keychain.
// Returns ErrNotFound when no credential exists.
func Get(key string) (string, error) {
	val, err := keyring.Get(serviceName, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keyring get %q: %w", key, err)
	}
	return val, nil
}

// Delete removes the stored credential for key.
func Delete(key string) error {
	if err := keyring.Delete(serviceName, key); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("keyring delete %q: %w", key, err)
	}
	return nil
}

// TokenKey returns the keychain key for a host's API token / PAT.
func TokenKey(host string) string { return "token:" + host }

// SessionKey returns the keychain key for a host's session cookie.
func SessionKey(host string) string { return "session:" + host }

// EnvToken reads JIRA_TOKEN from the environment.
func EnvToken() string { return os.Getenv("JIRA_TOKEN") }

// EnvSessionCookie reads JIRA_SESSION_COOKIE from the environment.
func EnvSessionCookie() string { return os.Getenv("JIRA_SESSION_COOKIE") }
