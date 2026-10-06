# Jira CLI (jira)

`jira` is a unified CLI for **Jira Cloud** and **Jira Data Center**. It provides
structured JSON/YAML output for automation and is optimised for AI agents working
with Jira issues, projects, sprints, and workflows.

## Before You Start

**1. Verify installation** — always check before running any `jira` command:

```bash
jira --version
```

If not installed:

| Platform | Command |
|----------|---------|
| macOS/Linux | `brew install atlassian/tap/jira-cli` |
| Go | `go install github.com/atlassian/jira-cli/cmd/jira@latest` |
| Binary | Download from [GitHub Releases](https://github.com/atlassian/jira-cli/releases) |

**2. Check authentication** — most commands require an active session:

```bash
jira auth status
```

**Jira Cloud API Token Requirements:**
- Generate a token at https://id.atlassian.com/manage-profile/security/api-tokens
- Use your Atlassian account email as username
- Token has the same permissions as your account

**Jira Data Center PAT Requirements:**
- Create a Personal Access Token at `{instance}/secure/ViewProfile.jspa#security`
- PATs require Jira 8.14+ (Data Center) or Jira 9.0+ (Server)

For config-free use in containers and CI pipelines, see [headless authentication](headless.md).

If not authenticated, log in:

```bash
# Jira Cloud (API token — recommended)
jira auth login https://myorg.atlassian.net --email me@example.com --token <API_TOKEN>

# Jira Data Center (PAT-based)
jira auth login https://jira.example.com --token <PAT>

# Jira Data Center (username + password — less secure)
jira auth login https://jira.example.com --username admin --password <PASSWORD>
```

**3. Set up a context** — contexts bind an instance to a project and optional default
settings, so you don't repeat flags on every command:

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
  --set-active
```

## Platform Awareness

Some commands are **Data Center only** or **Cloud only** — check the command
reference for `*(DC)*` and `*(Cloud)*` badges. Key differences:

| Feature | Data Center | Cloud |
|---------|:-----------:|:-----:|
| Issues (CRUD) | yes | yes |
| Projects | yes | yes |
| Sprints / Boards | yes | yes |
| Comments | yes | yes |
| Worklogs | yes | yes |
| Transitions | yes | yes |
| Attachments | yes | yes |
| Custom fields | yes | yes |
| Labels | yes | yes |
| Webhooks | yes | yes |
| JQL search | yes | yes |
| Agile/Software boards | yes | yes |
| Automation rules | — | yes |
| Issue navigator export | yes | — |

When a user's context is DC, do not suggest Cloud-only commands (and vice versa).
If the platform is unknown, ask or check with `jira auth status`.

## Common Workflows

### 1. Issue management

```bash
jira issue list --project MYPROJ --status "In Progress"  # List in-progress issues
jira issue view MYPROJ-123                                # View an issue
jira issue view MYPROJ-123 --comments                    # View with comments
jira issue create -p MYPROJ -t "Fix login bug" -T bug    # Create a bug
jira issue edit MYPROJ-123 --summary "Updated title"     # Edit summary
jira issue assign MYPROJ-123 --assignee me               # Assign to yourself
jira issue comment add MYPROJ-123 -b "Fixed in v2.0"     # Add comment
jira issue worklog add MYPROJ-123 --time 2h30m           # Log 2h30m
```

### 2. JQL search

```bash
jira issue list --jql "project = MYPROJ AND status = Open AND assignee = currentUser()"
jira issue list --jql "sprint in openSprints() AND priority = High" --json
jira issue list --jql "created >= -7d AND issuetype = Bug" --limit 50
```

### 3. Transitions

```bash
jira issue transition MYPROJ-123                         # Interactive picker
jira issue transition MYPROJ-123 --to "In Progress"      # Transition by name
jira issue transition MYPROJ-123 --to "Done" --comment "Completed" # With comment
```

### 4. Sprint management (Agile)

```bash
jira sprint list --board 42                              # List sprints for board 42
jira sprint view 123                                     # View sprint details
jira sprint issues 123                                   # List issues in sprint
jira sprint add MYPROJ-123 --sprint 123                  # Add issue to sprint
```

### 5. Structured output for scripting

```bash
jira issue list --json | jq '.[].key'
jira issue view MYPROJ-123 --json --jq '.fields.status.name'
jira project list --yaml
```

### 6. Raw API escape hatch

```bash
jira api /rest/api/3/issue/MYPROJ-123
jira api /rest/api/3/search --param "jql=project=MYPROJ" --param maxResults=10
jira api /rest/agile/1.0/board --method GET --json
```

## Global Flags

Every command accepts these inherited flags:

| Flag | Short | Purpose |
|------|-------|---------|
| `--context` | `-c` | Use a specific named context |
| `--json` | | JSON output |
| `--yaml` | | YAML output |
| `--jq` | | Apply a jq expression (requires `--json`) |
| `--template` | | Render with Go template |
| `--no-pager` | | Disable output paging |
| `--debug` | | Show HTTP request/response details |

## References

- [headless / env vars](headless.md) — Config-free CI/container auth (JIRA_TOKEN, JIRA_HOST) and full env var reference

<!-- auto-generated by cmd/docgen — do not edit below this line -->

- [issue](issue.md) — Work with Jira issues
- [project](project.md) — Work with Jira projects
- [sprint](sprint.md) — Manage sprints
- [board](board.md) — Work with Jira boards
- [comment](comment.md) — Manage issue comments
- [worklog](worklog.md) — Log work on issues
- [attachment](attachment.md) — Manage issue attachments
- [transition](transition.md) — Transition issue status
- [field](field.md) — Work with Jira custom fields
- [label](label.md) — Manage issue labels
- [auth](auth.md) — Manage Jira authentication credentials
- [context](context.md) — Manage Jira CLI contexts
- [webhook](webhook.md) — Manage Jira webhooks
- [mcp](mcp.md) — Model Context Protocol server for agents
- [other](other.md) — api

<!-- end auto-generated -->
