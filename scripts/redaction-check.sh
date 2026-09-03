#!/usr/bin/env -S usage bash
# Greps a diff for the agent-ops redaction term list, plus a 6+-digit
# integer sweep over every changed file under freshbooks/testdata/seed/.
# Default mode scans the staged index (for a pre-commit check). --range
# scans a branch diff's added lines instead, for a gate lane that cannot
# run against the index (Phase 7 security A9). Optional for outside
# contributors: if the resolver script isn't present (it lives in a
# sibling private repo), this exits 0 with a notice instead of failing.
#USAGE flag "--range <range>" help="Scan a git diff range (base..head)'s added lines instead of the staged index, e.g. main..phase-8/converge"

set -euo pipefail

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
# freshbooks/testdata/seed/ file) is a known synthetic placeholder rather
# than a possible real value: FreshBooks-style synthetic ids (700NN, the
# range every capture in this repo already uses -- Phase 7 security A1),
# well-known filler numbers (8675309 = Jenny's number, 4242424 = the Stripe
# test-card digits, 5555550100 and 5550100100 = synthetic phone numbers --
# the plan's D3 decision names 5555550100, and Phase 1's users_me.json
# capture already carries 5550100100 at line 33, so both are allowlisted
# rather than choking the sweep on pre-existing legitimate data (QA Q4),
# 999999999 and 1111111 = other conventional filler, also pre-existing --
# QA Q4), and any run of all zeros (the all-zero uuids and account numbers
# the seed captures already use).
seed_number_allowed() {
  local n="$1"
  case "$n" in
  8675309 | 4242424 | 5555550100 | 5550100100 | 999999999 | 1111111) return 0 ;;
  esac
  [[ "$n" =~ ^700[0-9]{2}$ ]] && return 0
  [[ "$n" =~ ^0+$ ]] && return 0
  return 1
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
# staged mode (git show ":$file"), or its added lines in range mode.
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
  local file="$1"
  case "$file" in
  freshbooks/testdata/seed/*) ;;
  *) return 0 ;;
  esac
  while IFS=$'\t' read -r lineno content; do
    [ -z "$lineno" ] && continue
    # Strip UUID-shaped tokens (8-4-4-4-12 hex) before the digit sweep: a
    # UUID is a hyphenated hex identifier, not an integer, but a segment
    # that happens to be all-decimal (e.g. this repo's own
    # "00000000-0000-4000-8000-000000000001" convention) still matches
    # [0-9]{6,} on its own. UUID redaction is a separate concern from this
    # sweep (the term-list scan / manual review), so any UUID-shaped
    # substring is exempted outright rather than allowlisted per value.
    content=$(printf '%s' "$content" | sed -E 's/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}//g')
    while [[ "$content" =~ ([0-9]{6,}) ]]; do
      local n="${BASH_REMATCH[1]}"
      if ! seed_number_allowed "$n"; then
        echo "redaction-check: unallowlisted 6+-digit number $n in $file:$lineno" >&2
        found=1
      fi
      content="${content/$n/}"
    done
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
  if [ -n "${usage_range:-}" ]; then
    content=$(git diff "$usage_range" -U0 -- "$file" | grep -E '^\+' | grep -v '^\+\+\+' | sed 's/^\+//') || true
  else
    content=$(git show ":$file" 2>/dev/null) || continue # deleted in this commit; nothing staged to scan
  fi
  scan_terms "$file" "$content"
  scan_seed_numbers "$file"
done < <("${files_cmd[@]}")

if [ "$found" -ne 0 ]; then
  exit 1
fi

echo "redaction-check: clean"
