# Changelog

All notable changes to this project will be documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-10-06
### Added
- Initial release of `jira` CLI for Jira Cloud and Jira Data Center.
- Enterprise authentication support:
  - Option B: Kerberos/SPNEGO negotiation via ticket cache (`kinit`) without stored credentials.
  - Option C: Session cookie capture via browser SSO reverse proxy and manual fallback.
  - Cloud basic auth (API tokens) and Data Center Personal Access Tokens (PAT).
  - Headless / CI configuration via environment variables.
- Issue management: list, view, create, edit, transition, assign, comment, worklog, link, clone, delete.
- Agile support: sprints, boards, backlog management.
- Built-in Model Context Protocol (MCP) server exposing tools for AI agents (`jira mcp serve`).
- Agent Skills specification (`skills/jira/SKILL.md`) with comprehensive rules and mirrors.
- Direct REST API escape hatch (`jira api <path>`).
- Output formatting via `--json`, `--yaml`, and `--jq`.

[Unreleased]: https://github.com/4lexjm/jira-cli/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/4lexjm/jira-cli/releases/tag/v0.1.0
