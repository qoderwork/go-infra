#!/usr/bin/env bash
# Fail when the working tree introduces a breaking change to the public API.
#
# The baseline is a git revision: the PR base sha, or the previous commit on
# push. apidiff compares exported declarations only, so additive changes
# (new functions, methods, packages) are compatible and pass the gate.
#
# Tool: go install golang.org/x/exp/cmd/apidiff@latest
# (note: golang.org/x/exp/apidiff is the library, not the command)
set -euo pipefail

BASE_REF="${1:-HEAD^}"
MODULE="$(go list -m)"

if ! command -v apidiff >/dev/null 2>&1; then
  echo "::error::apidiff not found. Install: go install golang.org/x/exp/cmd/apidiff@latest"
  exit 1
fi

tmp="$(mktemp -d)"
cleanup() {
  git worktree remove "$tmp/base" --force >/dev/null 2>&1 || true
  git worktree prune >/dev/null 2>&1 || true
}
trap cleanup EXIT

git worktree add --detach "$tmp/base" "$BASE_REF" >/dev/null
(cd "$tmp/base" && apidiff -m -w "$tmp/baseline.api" "$MODULE")

out="$(apidiff -m -incompatible "$tmp/baseline.api" "$MODULE" 2>&1 || true)"
if [[ -n "$out" ]]; then
  echo "Breaking API change(s) against $BASE_REF:"
  echo "$out"
  echo "If this is intentional, bump the major version and document the migration."
  exit 1
fi
echo "Public API is compatible with $BASE_REF"
