#!/usr/bin/env bash
# Ratchet gate: per-package statement coverage must not drop below the values
# recorded in coverage-baseline.txt. Raise the baseline when coverage grows;
# never lower it just to make a failing build pass.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
baseline="$here/coverage-baseline.txt"
if [[ ! -f "$baseline" ]]; then
  echo "::error::missing $baseline"
  exit 1
fi

declare -A min=()
while read -r pkg pct; do
  [[ -z "${pkg:-}" || "$pkg" == \#* ]] && continue
  min["$pkg"]="$pct"
done <"$baseline"

raw="$(mktemp)"
if ! go test -cover ./... -count=1 >"$raw" 2>&1; then
  cat "$raw"
  echo "::error::go test -cover failed"
  exit 1
fi

failed=0
while read -r pkg pct; do
  want="${min[$pkg]:-}"
  if [[ -z "$want" ]]; then
    echo "note: $pkg has no coverage baseline (new package), currently ${pct}%"
    continue
  fi
  if ! awk -v a="$pct" -v b="$want" 'BEGIN { exit !(a + 0 >= b + 0) }'; then
    echo "::error::$pkg coverage ${pct}% is below baseline ${want}%"
    failed=1
  fi
done < <(grep 'coverage:' "$raw" |
  sed -E 's/^ok[[:space:]]+([^[:space:]]+)[[:space:]].*coverage: ([0-9.]+)%.*/\1 \2/')

if ((failed)); then
  echo "Coverage ratchet failed"
  exit 1
fi
echo "Coverage ratchet OK"
