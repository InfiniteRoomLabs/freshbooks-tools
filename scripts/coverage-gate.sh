#!/usr/bin/env -S usage bash
#USAGE arg "<threshold>" help="Minimum coverage percentage required, e.g. 90"
#USAGE arg "<coverprofile>" help="Path to a go test -coverprofile file"

set -euo pipefail

if [ ! -f "$usage_coverprofile" ]; then
  echo "coverage-gate: no such coverprofile: $usage_coverprofile" >&2
  exit 1
fi

# cmd/<binary>/main.go entry points are thin flag-parsing + os.Exit wiring
# that cannot be exercised by a test process (os.Exit would kill the test
# binary); the substantive logic they call into is required to live in a
# tested run()-style function instead (see docs/building.md). The filter is
# scoped to cmd/*/main.go specifically -- not any file named main.go, and
# not by directory -- so internal/cmd/ (a real, tested package) and
# freshbooks/internal/inventory/main.go (60+ statements of tested flag
# parsing and report rendering, not thin wiring) both stay counted. A
# profile with no measurable statements left after filtering is a hard
# FAIL: a module that is supposed to have code and doesn't measure any is a
# red gate, not a vacuous green one.
filtered=$(mktemp)
trap 'rm -f "$filtered"' EXIT
grep -v '/cmd/[^/]*/main\.go:' "$usage_coverprofile" >"$filtered" || true

if [ "$(wc -l <"$filtered")" -le 1 ]; then
  echo "coverage-gate: $usage_coverprofile has no measurable statements outside cmd/*/main.go" >&2
  exit 1
fi

total_line=$(go tool cover -func="$filtered" | tail -1)
percent=$(echo "$total_line" | awk '{print $NF}' | tr -d '%')

if [ -z "$percent" ]; then
  echo "coverage-gate: could not parse a total from: $total_line" >&2
  exit 1
fi

echo "coverage-gate: $usage_coverprofile total = ${percent}% (floor ${usage_threshold}%)"

# Compare as integers scaled by 10 so "89.5 < 90" fails correctly.
scaled_percent=$(awk -v p="$percent" 'BEGIN { printf "%d", p * 10 }')
scaled_threshold=$(awk -v t="$usage_threshold" 'BEGIN { printf "%d", t * 10 }')

if [ "$scaled_percent" -lt "$scaled_threshold" ]; then
  echo "coverage-gate: FAIL -- ${percent}% is below the ${usage_threshold}% floor" >&2
  exit 1
fi

echo "coverage-gate: PASS"
