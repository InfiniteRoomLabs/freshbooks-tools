#!/usr/bin/env -S usage bash
#USAGE arg "<threshold>" help="Minimum coverage percentage required, e.g. 90"
#USAGE arg "<coverprofile>" help="Path to a go test -coverprofile file"

set -euo pipefail

if [ ! -f "$usage_coverprofile" ]; then
  echo "coverage-gate: no such coverprofile: $usage_coverprofile" >&2
  exit 1
fi

# main.go entry points are thin flag-parsing + os.Exit wiring that cannot
# be exercised by a test process (os.Exit would kill the test binary); the
# substantive logic they call into is required to live in a tested
# run()-style function instead (see CLAUDE.md). Excluding main.go files
# (by filename, not by directory -- internal/cmd/ is a real, tested
# package and must stay counted) from the gated total avoids penalizing
# that untestable-by-design sliver without hiding it from a human reading
# the full coverage.out.
filtered=$(mktemp)
trap 'rm -f "$filtered"' EXIT
{
  head -n 1 "$usage_coverprofile"
  tail -n +2 "$usage_coverprofile" | grep -v '/main\.go:' || true
} >"$filtered"

if [ "$(wc -l <"$filtered")" -le 1 ]; then
  echo "coverage-gate: $usage_coverprofile has no measurable statements outside main.go -- nothing to cover, PASS"
  exit 0
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
