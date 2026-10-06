# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | ✅        |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Send a report to security@example.com with:
- A description of the vulnerability
- Steps to reproduce
- Potential impact

We aim to respond within 72 hours and will coordinate disclosure timing with you.

## Credential Handling

- Credentials are stored in the OS keychain by default.
- `JIRA_TOKEN` is never logged; `JIRA_HTTP_DEBUG=1` redacts the `Authorization` header.
- Never commit tokens to source control. Use environment variables or the CLI's credential store.
