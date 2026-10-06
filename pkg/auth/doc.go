// Package auth implements HTTP transports for Jira Data Center authentication.
//
// Option B — Kerberos/SPNEGO: uses the current user's Kerberos credential cache
// (populated by kinit or a domain-joined workstation) to negotiate authentication
// via the HTTP Negotiate scheme. No credentials are stored by the CLI.
//
// Option C — Session cookie: captures a JSESSIONID from the user's browser SSO
// session and attaches it to every HTTP request. The cookie is stored in the OS
// keychain and expires with the Jira server session (typically 8 hours).
package auth
