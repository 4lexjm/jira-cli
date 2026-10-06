# MCP — Model Context Protocol Server

The `jira mcp` command starts a Model Context Protocol server that exposes
Jira operations as structured tools to MCP-compatible AI agents (Claude Code,
Cursor, Copilot, Gemini CLI, and others).

## jira mcp serve

Start the MCP server. The server reads from stdin and writes to stdout using
the MCP JSON-RPC protocol. Most agents launch it automatically via the MCP
configuration file.

### Usage

```
jira mcp serve [flags]
```

### Flags

| Flag | Short | Description |
|---|---|---|
| `--read-only` | | Expose only read-only tools (no create/edit/transition/delete) |
| `--context` | `-c` | Pin the server to a specific context |
| `--project` | `-p` | Override the default project key |
| `--board` | `-b` | Override the default board ID |

### Examples

```bash
# Start the MCP server (stdio transport)
jira mcp serve

# Read-only mode (safe for untrusted agents)
jira mcp serve --read-only

# Pinned to a context
jira mcp serve --context cloud-myorg --project MYPROJ
```

---

## Agent Configuration

### Claude Code (`~/.claude/settings.json`)

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

### Cursor (`.cursor/mcp.json` or `~/.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "jira": {
      "command": "jira",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Gemini CLI (`~/.gemini/settings.json`)

```json
{
  "mcpServers": {
    "jira": {
      "command": "jira",
      "args": ["mcp", "serve", "--read-only"]
    }
  }
}
```

### Environment Variables (CI / headless)

```json
{
  "mcpServers": {
    "jira": {
      "command": "jira",
      "args": ["mcp", "serve"],
      "env": {
        "JIRA_HOST": "https://myorg.atlassian.net",
        "JIRA_TOKEN": "${JIRA_API_TOKEN}",
        "JIRA_EMAIL": "bot@example.com",
        "JIRA_PROJECT": "MYPROJ"
      }
    }
  }
}
```

---

## Available MCP Tools

The MCP server exposes the following tools. Read-only tools are always
available; write tools are suppressed when `--read-only` is passed.

### Read-only tools

| Tool | Description |
|---|---|
| `jira_get_issue` | Get full details for an issue by key |
| `jira_list_issues` | List issues by JQL or filter flags |
| `jira_list_comments` | List comments on an issue |
| `jira_list_worklogs` | List worklogs on an issue |
| `jira_list_transitions` | List available transitions for an issue |
| `jira_list_projects` | List accessible projects |
| `jira_get_project` | Get project details by key |
| `jira_list_sprints` | List sprints for a board |
| `jira_get_sprint` | Get sprint details by ID |
| `jira_list_sprint_issues` | List issues in a sprint |
| `jira_list_boards` | List boards |
| `jira_list_attachments` | List attachments on an issue |
| `jira_list_fields` | List all custom and system fields |
| `jira_get_user` | Look up a user by account ID or email |
| `jira_search_users` | Search for users by name or email |

### Write tools (disabled in `--read-only` mode)

| Tool | Description |
|---|---|
| `jira_create_issue` | Create a new issue |
| `jira_edit_issue` | Edit issue fields |
| `jira_transition_issue` | Transition an issue to a new status |
| `jira_assign_issue` | Assign or unassign an issue |
| `jira_add_comment` | Add a comment to an issue |
| `jira_edit_comment` | Edit a comment |
| `jira_delete_comment` | Delete a comment |
| `jira_add_worklog` | Log time on an issue |
| `jira_delete_worklog` | Delete a worklog entry |
| `jira_add_attachment` | Upload an attachment |
| `jira_delete_attachment` | Delete an attachment |
| `jira_link_issues` | Create an issue link |
| `jira_create_sprint` | Create a sprint |
| `jira_add_to_sprint` | Add an issue to a sprint |
| `jira_remove_from_sprint` | Remove an issue from a sprint |
| `jira_create_webhook` | Register a webhook |
| `jira_delete_webhook` | Delete a webhook |

---

## Tool Schemas (selected)

### `jira_get_issue`

**Input:**
```json
{
  "type": "object",
  "properties": {
    "key": {
      "type": "string",
      "description": "Issue key, e.g. MYPROJ-42"
    },
    "include_comments": {
      "type": "boolean",
      "description": "Include comments in the response"
    },
    "include_worklogs": {
      "type": "boolean",
      "description": "Include worklogs in the response"
    }
  },
  "required": ["key"]
}
```

**Output:**
```json
{
  "type": "object",
  "properties": {
    "key":        { "type": "string" },
    "summary":    { "type": "string" },
    "status":     { "type": "string" },
    "type":       { "type": "string" },
    "priority":   { "type": "string" },
    "assignee":   { "type": ["null", "object"] },
    "reporter":   { "type": "object" },
    "description":{ "type": ["null", "string"] },
    "labels":     { "type": "array", "items": { "type": "string" } },
    "components": { "type": "array" },
    "sprint":     { "type": ["null", "object"] },
    "epic":       { "type": ["null", "string"] },
    "created_at": { "type": "string" },
    "updated_at": { "type": "string" },
    "url":        { "type": "string" },
    "comments":   { "type": ["null", "array"] },
    "worklogs":   { "type": ["null", "array"] }
  }
}
```

### `jira_list_issues`

**Input:**
```json
{
  "type": "object",
  "properties": {
    "jql": {
      "type": "string",
      "description": "JQL query string"
    },
    "project": {
      "type": "string",
      "description": "Project key shorthand (ignored when jql is set)"
    },
    "status": { "type": "string" },
    "assignee": { "type": "string" },
    "limit": {
      "type": "integer",
      "description": "Max issues to return; defaults to 25, capped at 100"
    }
  }
}
```

### `jira_add_comment`

**Input:**
```json
{
  "type": "object",
  "properties": {
    "key":        { "type": "string", "description": "Issue key" },
    "body":       { "type": "string", "description": "Comment text" },
    "visibility": { "type": "string", "description": "Visibility restriction, e.g. role:Developer" },
    "internal":   { "type": "boolean", "description": "Mark as internal (JSM only)" }
  },
  "required": ["key", "body"]
}
```

### `jira_transition_issue`

**Input:**
```json
{
  "type": "object",
  "properties": {
    "key":        { "type": "string", "description": "Issue key" },
    "to":         { "type": "string", "description": "Target status or transition name" },
    "comment":    { "type": "string", "description": "Optional comment during transition" },
    "resolution": { "type": "string", "description": "Resolution to set (e.g. Fixed)" }
  },
  "required": ["key", "to"]
}
```
