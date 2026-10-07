# Headless / CI Authentication

Use `JIRA_TOKEN` + `JIRA_HOST` (ou les credentials OAuth 2.0) pour exécuter
`jira` dans des conteneurs et pipelines CI sans `jira auth login` préalable.

> **API tokens bloqués ?** Utiliser [OAuth 2.0 M2M](#oauth-20-machine-to-machine-recommandé-si-api-tokens-bloqués)
> (client credentials) ou un [PAT Data Center](#jira-data-center--pat-bearer).

## Quick Start

```bash
# Jira Cloud — OAuth 2.0 Machine-to-Machine (recommandé en entreprise)
export JIRA_HOST=https://myorg.atlassian.net
export JIRA_OAUTH_CLIENT_ID=my-service-account-client-id
export JIRA_OAUTH_CLIENT_SECRET=my-service-account-client-secret
export JIRA_AUTH_METHOD=oauth2-m2m
export JIRA_PROJECT=MYPROJ
jira issue list

# Jira Cloud — API token (si autorisé par l'organisation)
export JIRA_HOST=https://myorg.atlassian.net
export JIRA_TOKEN=my-api-token
export JIRA_EMAIL=me@example.com
export JIRA_PROJECT=MYPROJ
jira issue list

# Jira Data Center — PAT bearer (recommandé)
export JIRA_HOST=https://jira.example.com
export JIRA_TOKEN=my-pat
export JIRA_PROJECT=ABC
jira issue list

# Jira Data Center — username + password (basic auth)
export JIRA_HOST=https://jira.example.com
export JIRA_USERNAME=admin
export JIRA_TOKEN=my-password
export JIRA_AUTH_METHOD=basic
jira issue list
```

---

## OAuth 2.0 Machine-to-Machine (recommandé si API tokens bloqués)

Le flow **Client Credentials (2LO)** permet à un agent/service de s'authentifier
sans compte utilisateur ni interaction humaine. Le CLI gère le renouvellement
automatique de l'access token (valable 1h).

**Prérequis :**
1. Créer une app OAuth 2.0 sur [developer.atlassian.com/console](https://developer.atlassian.com/console/myapps/)
2. Choisir "Service Account" comme type d'accès
3. Ajouter les scopes nécessaires : `read:jira-work`, `write:jira-work`

```bash
export JIRA_HOST=https://myorg.atlassian.net
export JIRA_OAUTH_CLIENT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export JIRA_OAUTH_CLIENT_SECRET=ATOAxxxxxxxxxxxxxxxx
export JIRA_AUTH_METHOD=oauth2-m2m
export JIRA_PROJECT=MYPROJ
jira issue list
```

## Auth Method Rules

| Scenario | Resolved auth method |
|---|---|
| Cloud, `JIRA_OAUTH_CLIENT_ID` + `JIRA_AUTH_METHOD=oauth2-m2m` | OAuth 2.0 Client Credentials (2LO) |
| Cloud, `JIRA_EMAIL` set, no `JIRA_AUTH_METHOD` | `basic` — token used as password |
| Cloud, `JIRA_AUTH_METHOD=bearer` | `bearer` — OAuth 2.0 access token direct |
| DC, no `JIRA_USERNAME`, no `JIRA_AUTH_METHOD` | `bearer` (default) — PAT bearer auth |
| DC, `JIRA_USERNAME` set, no `JIRA_AUTH_METHOD` | `basic` — username:token basic auth |
| DC, `JIRA_AUTH_METHOD=bearer` | `bearer` |
| DC, `JIRA_AUTH_METHOD=basic` + no `JIRA_USERNAME` | **error** — set `JIRA_USERNAME` |

## Environment Variables

| Variable | Description |
|---|---|
| `JIRA_TOKEN` | Authentication token or password. Bypasses keyring. |
| `JIRA_HOST` | Jira instance base URL. Required for config-free use. |
| `JIRA_EMAIL` | Email for Cloud basic auth (Atlassian account email). |
| `JIRA_USERNAME` | Username for Data Center basic auth. |
| `JIRA_AUTH_METHOD` | Auth method: `basic`, `bearer`, or `oauth2-m2m`. Cloud defaults to `basic`; DC defaults to `bearer`. |
| `JIRA_OAUTH_CLIENT_ID` | OAuth 2.0 client ID (app ID from developer.atlassian.com). Used for 3LO browser flow and 2LO M2M. |
| `JIRA_OAUTH_CLIENT_SECRET` | OAuth 2.0 client secret. Required with `JIRA_OAUTH_CLIENT_ID`. |
| `JIRA_PROJECT` | Default project key (e.g. `MYPROJ`). |
| `JIRA_BOARD` | Default board ID for sprint/board commands. |
| `JIRA_CONFIG_DIR` | Config directory override (default: `~/.config/jira`). |
| `JIRA_HTTP_DEBUG` | Set to `1` to print request URLs and response status codes. |
| `JIRA_ALLOW_INSECURE_STORE` | Allow plain-text credential storage (not recommended). |
| `JIRA_KEYRING_TIMEOUT` | Keyring operation timeout (e.g. `2m`). |

## CI/CD Examples

### GitHub Actions — OAuth 2.0 M2M (API tokens bloqués)

```yaml
- name: Transition Jira issue via OAuth 2.0
  env:
    JIRA_HOST: https://myorg.atlassian.net
    JIRA_OAUTH_CLIENT_ID: ${{ secrets.JIRA_OAUTH_CLIENT_ID }}
    JIRA_OAUTH_CLIENT_SECRET: ${{ secrets.JIRA_OAUTH_CLIENT_SECRET }}
    JIRA_AUTH_METHOD: oauth2-m2m
    JIRA_PROJECT: MYPROJ
  run: |
    jira issue transition MYPROJ-${{ env.ISSUE_KEY }} --to "In Review"
    jira issue comment add MYPROJ-${{ env.ISSUE_KEY }} \
      -b "PR ${{ github.event.pull_request.html_url }} opened"
```

### GitHub Actions — API token (si autorisé)

```yaml
- name: Comment on Jira issue
  env:
    JIRA_HOST: https://myorg.atlassian.net
    JIRA_TOKEN: ${{ secrets.JIRA_API_TOKEN }}
    JIRA_EMAIL: bot@example.com
    JIRA_PROJECT: MYPROJ
  run: |
    jira issue comment add MYPROJ-${{ github.event.issue.number }} \
      -b "PR ${{ github.event.pull_request.html_url }} merged"
```

### Bitbucket Pipelines — OAuth 2.0 M2M

```yaml
pipelines:
  default:
    - step:
        script:
          - jira issue transition MYPROJ-$ISSUE_KEY --to "In Review"
        variables:
          JIRA_HOST: https://myorg.atlassian.net
          JIRA_OAUTH_CLIENT_ID: $JIRA_OAUTH_CLIENT_ID_SECRET
          JIRA_OAUTH_CLIENT_SECRET: $JIRA_OAUTH_CLIENT_SECRET_SECRET
          JIRA_AUTH_METHOD: oauth2-m2m
```

### Data Center — PAT dans un pipeline

```yaml
# GitHub Actions — Jira DC avec PAT
- name: Log work on DC issue
  env:
    JIRA_HOST: https://jira.example.com
    JIRA_TOKEN: ${{ secrets.JIRA_DC_PAT }}
    JIRA_PROJECT: ABC
  run: |
    jira issue worklog add ABC-$ISSUE_NUMBER --time 2h \
      --comment "Automated: deployed from CI pipeline #${{ github.run_number }}"
```

### Docker — MCP server en mode headless OAuth

```dockerfile
ENV JIRA_HOST=https://myorg.atlassian.net
ENV JIRA_AUTH_METHOD=oauth2-m2m
# JIRA_OAUTH_CLIENT_ID et JIRA_OAUTH_CLIENT_SECRET injectés au runtime
CMD ["jira", "mcp", "serve", "--read-only"]
```

## Saved-Host Behaviour

When `JIRA_HOST` matches a host already in `~/.config/jira/config.yml`, the
saved entry is used as the base. `JIRA_TOKEN` always overrides the stored token.
`JIRA_EMAIL`, `JIRA_USERNAME`, `JIRA_AUTH_METHOD`, and OAuth credentials, when
set, override saved values.

