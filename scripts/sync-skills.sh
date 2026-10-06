#!/usr/bin/env bash
# sync-skills.sh — Mirror skills/jira into .claude/skills/jira and .agents/skills/jira
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SRC="$REPO_ROOT/skills/jira"


for dest in ".claude/skills/jira" ".agents/skills/jira"; do
  target="$REPO_ROOT/$dest"
  echo "Syncing $SRC -> $target"
  rm -rf "$target"
  cp -r "$SRC" "$target"
done

echo "Done. Run 'git diff' to review."
