#!/usr/bin/env bash
# check-generated-skill.sh — Verify that .claude/skills/jira and .agents/skills/jira
# are identical to skills/jira (the canonical source).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SRC="$REPO_ROOT/skills/jira"

EXIT=0

for dest in ".claude/skills/jira" ".agents/skills/jira"; do
  target="$REPO_ROOT/$dest"
  if ! diff -rq "$SRC" "$target" >/dev/null 2>&1; then
    echo "ERROR: $dest is out of sync with skills/jira. Run 'make sync-skills' to update."
    EXIT=1
  else
    echo "OK: $dest is up-to-date"
  fi
done

exit $EXIT
