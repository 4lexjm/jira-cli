# jira-cli

`jira` is a unified CLI for **Jira Cloud** and **Jira Data Center**. It is optimised for
AI agents — providing structured JSON/YAML output, a built-in MCP server, and
an agent skill — but is equally useful for humans in the terminal.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/atlassian/jira-cli)](https://goreportcard.com/report/github.com/atlassian/jira-cli)

---

## Features

- **Issues** — list, view, create, edit, transition, assign, clone, delete
- **Comments** — add, list, edit, delete; support for visibility restrictions
- **Worklogs** — log time, list, delete; automatic estimate adjustment
- **Attachments** — upload, download, list, delete
- **Sprints & Boards** — list, view, add/remove issues, create/start/close sprints
- **Projects** — list, view, create; components, versions, issue types
- **Transitions** — trigger any workflow transition by name or ID
- **Custom fields** — read and write any field by ID
- **Labels & Components** — manage metadata on issues
- **Webhooks** — register and manage event hooks
- **JQL** — full JQL search support via `--jql` or `--json | jq`
- **MCP server** — expose all tools to AI agents via Model Context Protocol
- **Headless / CI** — config-free via `JIRA_TOKEN` + `JIRA_HOST` env vars
- **Structured output** — `--json`, `--yaml`, `--jq`, `--template` on every command
- **Raw API** — `jira api <path>` for any endpoint

---

## Installation

| Platform | Command |
|----------|---------|
| macOS / Linux (Homebrew) | `brew install atlassian/tap/jira-cli` |
| Go | `go install github.com/atlassian/jira-cli/cmd/jira@latest` |
| Binary | Download from [GitHub Releases](https://github.com/atlassian/jira-cli/releases) |

---

## Quick Start

### 1. Authenticate

```bash
# Jira Cloud (API token — https://id.atlassian.com/manage-profile/security/api-tokens)
jira auth login https://myorg.atlassian.net \
  --email me@example.com \
  --token <API_TOKEN>

# Jira Data Center (Personal Access Token)
jira auth login https://jira.example.com --token <PAT>
```

### 2. Create a context

```bash
# Cloud
jira context create cloud-myorg \
  --host myorg.atlassian.net \
  --project MYPROJ \
  --set-active

# Data Center
jira context create dc-prod \
  --host jira.example.com \
  --project ABC \
  --board 10 \
  --set-active
```

### 3. Start working

```bash
# List open issues
jira issue list

# View an issue with comments
jira issue view MYPROJ-42 --comments

# Create a bug
jira issue create -p MYPROJ -t "Login button broken" -T bug -p High

# Transition to In Progress
jira issue transition MYPROJ-42 --to "In Progress"

# Add a comment
jira issue comment add MYPROJ-42 -b "Working on this now"

# Log 2 hours
jira issue worklog add MYPROJ-42 --time 2h
```

---

## Common Workflows

### 1. Issue management

```bash
jira issue list --project MYPROJ --status "In Progress"
jira issue view MYPROJ-123
jira issue view MYPROJ-123 --comments
jira issue create -p MYPROJ -t "Fix login bug" -T bug
jira issue edit MYPROJ-123 --assignee me --priority High
jira issue comment add MYPROJ-123 -b "Fixed in v2.0"
jira issue worklog add MYPROJ-123 --time 2h30m
```

### 2. JQL search

```bash
# Issues assigned to me in the current sprint
jira issue list --jql "assignee = currentUser() AND sprint in openSprints()"

# Bugs created this week
jira issue list --jql "issuetype = Bug AND created >= -7d" --json

# All issues in an epic
jira issue list --jql "\"Epic Link\" = MYPROJ-10"
```

### 3. Sprint management

```bash
jira sprint list --board 42
jira sprint issues 123 --status "In Progress"
jira sprint add MYPROJ-123 --sprint current --board 42
jira sprint close 123 --move-to 124 --confirm
```

### 4. Batch operations via JQL + jq

```bash
# Close all resolved issues
jira issue list \
  --jql "project = MYPROJ AND status = Resolved" \
  --json --jq '.[].key' \
  | xargs -I{} jira issue transition {} --to "Done" --resolution "Fixed"

# Assign all unassigned high-priority bugs to yourself
jira issue list \
  --jql "project = MYPROJ AND issuetype = Bug AND priority = High AND assignee is EMPTY" \
  --json --jq '.[].key' \
  | xargs -I{} jira issue assign {} me
```

### 5. Structured output for scripting

```bash
jira issue list --json | jq '.[].key'
jira issue view MYPROJ-42 --json --jq '.fields.status.name'
jira project list --yaml
```

### 6. Branch, permission, webhook, and field management

```bash
jira webhook create --name "CI" --url https://ci.example.com/hook \
  --event jira:issue_updated
jira field list --type custom --json | jq '.[] | {id, name}'
jira issue edit MYPROJ-42 --custom customfield_10016=5
```

### 7. MCP server for AI agents

```bash
# Start the MCP server (agents typically launch this automatically)
jira mcp serve

# Read-only mode for untrusted agents
jira mcp serve --read-only
```

Configure in your agent:

```json
{
  "mcpServers": {
    "jira": {
      "command": "jira",
      "args": ["mcp", "serve", "--context", "cloud-myorg"]
    }
  }
}
```

### Structured output & raw API access

Every command supports the global `--json` and `--yaml` flags for automation-ready output.

For endpoints that are not yet wrapped, reach directly for the API escape hatch:

```bash
jira api /rest/api/3/issue/MYPROJ-42 --json
jira api /rest/api/3/search --param "jql=project=MYPROJ" --param maxResults=10
jira api /rest/agile/1.0/board/10/sprint --param state=active --json
```

---

## Headless / CI Authentication

Use `JIRA_TOKEN` + `JIRA_HOST` to run `jira` in containers and CI without
any prior setup:

```bash
# Jira Cloud
export JIRA_HOST=https://myorg.atlassian.net
export JIRA_TOKEN=my-api-token
export JIRA_EMAIL=bot@example.com
export JIRA_PROJECT=MYPROJ
jira issue list

# Jira Data Center (PAT)
export JIRA_HOST=https://jira.example.com
export JIRA_TOKEN=my-pat
export JIRA_PROJECT=ABC
jira issue list
```

See [headless authentication](skills/jira/rules/headless.md) for the full
environment variable reference and CI/CD examples.

---

## Agent Skill

The `skills/jira/` directory contains an [Agent Skills](https://agentskills.io/specification)
skill that teaches AI agents how to use `jira`. Install it in your agent:

```bash
# Using bkt (Bitbucket CLI)
bkt skill install myteam/jira-cli jira

# Manual install (Claude Code)
cp -r skills/jira ~/.claude/skills/jira

# Manual install (universal agents)
cp -r skills/jira .agents/skills/jira
```

The skill provides:
- Command reference for all subcommands (`rules/*.md`)
- Setup and authentication guide (`rules/guide.md`)
- Headless/CI patterns (`rules/headless.md`)
- MCP server configuration (`rules/mcp.md`)

---

## Security

- Credentials are stored in the OS keychain by default.
- Use `JIRA_TOKEN` for CI; never hardcode tokens in scripts or committed files.
- Set `JIRA_HTTP_DEBUG=1` to debug API requests (redacts `Authorization` headers).

Found a security issue? See [SECURITY.md](SECURITY.md) for responsible disclosure.

---

## Development

### Project Layout

```
cmd/jira/             # CLI entry point
internal/jiracmd/     # Main() wiring (factory + root command)
internal/build/       # Version metadata (overridden via ldflags)
internal/config/      # Context and host configuration
pkg/cmd/              # Cobra command implementations (auth, issue, project, ...)
pkg/cmdutil/          # Shared command helpers and factory wiring
pkg/iostreams/        # IO stream abstractions
pkg/jiracloud/        # Jira Cloud REST API client
pkg/jiradc/           # Jira Data Center REST API client
pkg/format/           # Output rendering helpers
pkg/httpx/            # Shared HTTP client and retry logic
skills/jira/          # Agent skill (SKILL.md + rules/)
```

### Building & Testing

```bash
make build      # Build the binary to ./bin/jira
make test       # Run unit tests
make fmt        # Format code
make lint       # Run linters
make tidy       # Tidy go modules
```

### Debugging HTTP Requests

```bash
JIRA_HTTP_DEBUG=1 jira issue view MYPROJ-42
```

---

## Support

- **Questions / Ideas**: File an [issue](https://github.com/atlassian/jira-cli/issues/new?template=feature_request.md)
- **Bug Reports**: File an [issue](https://github.com/atlassian/jira-cli/issues/new?template=bug_report.md)

## License

`jira` is available under the [MIT License](LICENSE).
