#!/bin/bash
# Standard commit script with Conventional Commits validation
# Usage: scripts/commit.sh "feat(api): add skill endpoint"

set -euo pipefail

MESSAGE="${1:-}"

if [ -z "$MESSAGE" ]; then
  echo "Usage: scripts/commit.sh \"<type>(<scope>): <description>\""
  echo ""
  echo "Types: feat, fix, docs, style, refactor, perf, test, chore, ci, build"
  echo "Examples:"
  echo "  scripts/commit.sh \"feat(api): add skill import endpoint\""
  echo "  scripts/commit.sh \"fix(store): handle nil body in skill upsert\""
  exit 1
fi

# Validate conventional commit format
if ! echo "$MESSAGE" | grep -qE '^(feat|fix|docs|style|refactor|perf|test|chore|ci|build)(\(.+\))?: .+'; then
  echo "Error: Commit message must follow Conventional Commits format."
  echo "  Format: <type>(<scope>): <description>"
  echo "  Types: feat, fix, docs, style, refactor, perf, test, chore, ci, build"
  exit 1
fi

echo "Committing: $MESSAGE"
git commit -m "$MESSAGE"
