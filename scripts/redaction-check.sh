#!/usr/bin/env -S usage bash
# Greps a diff for the agent-ops redaction term list, plus a 6+-digit
# integer sweep over every changed file under freshbooks/testdata/.
# Default mode scans the staged index (for a pre-commit check). --range
# scans a branch diff's added lines instead, for a gate lane that cannot
# run against the index (Phase 7 security A9). Optional for outside
# contributors: it needs `usage` on PATH (it is this script's interpreter,
# so `mise install` first), and if the resolver script isn't present (it
# lives in a sibling private repo) this exits 0 with a notice instead of
# failing. scripts/redaction-selftest.sh is this script's regression test
# and runs once per `mise run check`.
#USAGE flag "--range <range>" help="Scan a git diff range (base..head)'s added lines instead of the staged index, e.g. main..phase-8/converge"

set -euo pipefail

# Validate the range before scanning anything. The file list is produced by
# a process substitution below, whose exit status is discarded, so an
# unresolvable range -- a typo, or a shallow CI checkout that never fetched
# the base -- would otherwise yield an empty file list and a green result
# (Phase 8 security A2).
if [ -n "${usage_range:-}" ] && ! git rev-list --count "$usage_range" >/dev/null 2>&1; then
  echo "redaction-check: unusable range: $usage_range" >&2
  exit 2
fi

resolver="$HOME/projects/infinite-room-labs/agent-ops/scripts/resolve-redaction-terms.py"

if [ ! -f "$resolver" ]; then
  echo "redaction-check: term list not available (optional for outside contributors)"
  exit 0
fi

terms_raw=$(cd "$(dirname "$resolver")" && uv run "$(basename "$resolver")")
if [ -z "$terms_raw" ]; then
  echo "redaction-check: no redaction terms configured"
  exit 0
fi

mapfile -t terms <<<"$terms_raw"

# Short terms (< 8 chars) are ordinary-English-word collision risks (e.g. a
# configured term "Delete" matching the API-vocabulary sentence "List, All,
# Get, Create, Update, Delete"); require word boundaries for those. Longer
# terms are specific enough that a fixed-string substring match is fine and
# catches more (e.g. a leak embedded mid-identifier).
short_term_threshold=8

# seed_number_allowed reports whether n (a 6+-digit run matched in a
# freshbooks/testdata/ file) is a known synthetic placeholder rather than a
# possible real value.
#
# Conventional filler: 8675309 = Jenny's number, 4242424 = the repo's
# synthetic identity_id, 5555550100 and 5550100100 = synthetic phone
# numbers (the plan's D3 decision names 5555550100, and Phase 1's
# users_me.json capture already carries 5550100100 at line 33, so both are
# allowlisted rather than choking the sweep on pre-existing legitimate data
# -- QA Q4), 999999999 and 1111111 = other conventional filler, also
# pre-existing (QA Q4), plus any run of all zeros (the all-zero uuids and
# account numbers the seed captures already use).
#
# FreshBooks' own published example ids: 1825574, 2003170, 2003174,
# 47634496 and 2976412 each appear in
# freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json,
# and 900123 is a hand-written transactionid added in Phase 2, before this
# repo had a live token at all. All six predate the seed corpus and were
# traced to source by the Phase 8 security lane; they are vendor sample
# data, not account data. They need entries because Phase 8 A5 widened this
# sweep from freshbooks/testdata/seed/ to freshbooks/testdata/, which is
# where the fixtures carrying them live.
#
# FreshBooks-style synthetic ids (700NN, the range every capture in this
# repo uses -- Phase 7 security A1) need no entry: 5 digits is below this
# sweep's 6-digit threshold, so they are never matched in the first place
# (Phase 8 code review R3, which found the old ^700[0-9]{2}$ branch dead).
seed_number_allowed() {
  [[ "$1" =~ ^(8675309|4242424|5555550100|5550100100|999999999|1111111|1825574|2003170|2003174|900123|47634496|2976412|0+)$ ]]
}

# timestamp_re matches an ISO-8601-ish instant, space- or T-separated, with
# an optional fractional part and offset. A timestamp's microsecond field
# is a 6-digit run and its date is not an identifier at all, so the sweep
# strips instants before counting digits -- otherwise every capture's
# updated_at is a finding. Whether an instant is itself too revealing is a
# separate concern (Phase 8 security A6, rounding), not this sweep's.
timestamp_re='[0-9]{4}-[0-9]{2}-[0-9]{2}[T ][0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-][0-9]{2}:?[0-9]{2})?'

uuid_re='[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}'

# uuid_exempt reports whether a UUID-shaped (8-4-4-4-12) token may be
# dropped before the digit sweep, which would otherwise match an
# all-decimal segment of it as an integer. Only two shapes qualify:
#
#   1. it carries a non-decimal hex digit (a-f), so it cannot be a
#      FreshBooks integer id at all; or
#   2. it is this repo's synthetic convention -- zeros apart from the
#      version and variant nibbles and a short trailing counter, e.g.
#      00000000-0000-4000-8000-000000000123.
#
# Everything else stays subject to the sweep, so an entirely decimal
# uuid-shaped token (18255740-1234-5678-9012-123456789012) or a real id
# wearing a synthetic tail (12345678-0000-4000-8000-000000000001) still
# fails, rather than being exempted by shape alone (Phase 8 security A3,
# code review R4).
uuid_exempt() {
  if [[ "$1" =~ [a-fA-F] ]]; then
    return 0
  fi
  [[ "$1" =~ ^0{8}-0{4}-[0-9]0{3}-[0-9]0{3}-0{4}[0-9]{8}$ ]]
}

# added_lines_with_numbers emits "lineno<TAB>content" for each added
# (non-context) line in file's diff over range, using the new-file line
# numbers from the unified diff's hunk headers -- pure bash, no gawk
# extension, so it runs the same on any system's /usr/bin/awk (or none).
added_lines_with_numbers() {
  local range="$1" file="$2" lineno=0 dline
  while IFS= read -r dline; do
    if [[ "$dline" =~ ^@@\ -[0-9]+(,[0-9]+)?\ \+([0-9]+) ]]; then
      lineno="${BASH_REMATCH[2]}"
    elif [[ "$dline" == +++* ]]; then
      continue
    elif [[ "$dline" == +* ]]; then
      printf '%s\t%s\n' "$lineno" "${dline:1}"
      lineno=$((lineno + 1))
    fi
  done < <(git diff "$range" -U0 -- "$file")
}

# numbered_lines emits "lineno<TAB>content" for file's full content in
# staged mode (git show ":$file"), or its added lines in range mode. It is
# the one extractor both scans read, so neither mode can scan a different
# set of lines than the other.
numbered_lines() {
  local file="$1"
  if [ -n "${usage_range:-}" ]; then
    added_lines_with_numbers "$usage_range" "$file"
  else
    git show ":$file" 2>/dev/null | cat -n | sed -E 's/^ *([0-9]+)\t/\1\t/'
  fi
}

found=0

scan_seed_numbers() {
  local file="$1" lineno content u n
  case "$file" in
  freshbooks/testdata/*) ;;
  *) return 0 ;;
  esac
  while IFS=$'\t' read -r lineno content; do
    [ -z "$lineno" ] && continue
    content=$(printf '%s' "$content" | sed -E "s/$timestamp_re//g")
    # Drop exempt UUID-shaped tokens before the sweep; anything uuid-shaped
    # that is not exempt stays in the content and is swept like any other
    # digit run.
    while IFS= read -r u; do
      uuid_exempt "$u" && content="${content//"$u"/}"
    done < <(printf '%s\n' "$content" | grep -oE "$uuid_re")
    while IFS= read -r n; do
      seed_number_allowed "$n" && continue
      echo "redaction-check: unallowlisted 6+-digit number $n in $file:$lineno" >&2
      found=1
    done < <(printf '%s\n' "$content" | grep -oE '[0-9]{6,}')
  done < <(numbered_lines "$file")
}

scan_terms() {
  local file="$1" content="$2" i term search escaped hit
  for i in "${!terms[@]}"; do
    term="${terms[$i]}"
    search="${term%%==>*}"
    [ -z "$search" ] && continue
    if [ "${#search}" -lt "$short_term_threshold" ]; then
      escaped=$(printf '%s' "$search" | sed 's/[][\.^$*+?(){}|\\]/\\&/g')
      hit=$(printf '%s' "$content" | grep -qiE "\\b${escaped}\\b" && echo 1 || true)
    else
      hit=$(printf '%s' "$content" | grep -qiF -- "$search" && echo 1 || true)
    fi
    if [ -n "$hit" ]; then
      echo "redaction-check: possible leak in $file (term #$i)" >&2
      found=1
    fi
  done
}

if [ -n "${usage_range:-}" ]; then
  files_cmd=(git diff "$usage_range" --name-only)
else
  files_cmd=(git diff --cached --name-only)
fi

while IFS= read -r file; do
  [ -z "$file" ] && continue
  # One extractor for both scans (Phase 8 security A1 / simplification S6):
  # range mode's old term-scan pipeline ended in a *basic*-regexp
  # `grep -v '^\+\+\+'`, where \+ is the one-or-more operator, so it
  # discarded every added line and the term scan silently scanned nothing.
  content=$(numbered_lines "$file" | cut -f2-) || continue # deleted in this commit; nothing staged to scan
  scan_terms "$file" "$content"
  scan_seed_numbers "$file"
done < <("${files_cmd[@]}")

if [ "$found" -ne 0 ]; then
  exit 1
fi

echo "redaction-check: clean"
