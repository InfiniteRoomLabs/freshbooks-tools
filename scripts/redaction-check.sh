#!/usr/bin/env bash
# Greps the staged diff for the agent-ops redaction term list before a
# commit. Optional for outside contributors: if the resolver script isn't
# present (it lives in a sibling private repo), this exits 0 with a notice
# instead of failing the commit.
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

found=0
while IFS= read -r file; do
  [ -z "$file" ] && continue
  if ! git cat-file -e ":$file" 2>/dev/null; then
    continue # deleted in this commit; nothing staged to scan
  fi
  content=$(git show ":$file" 2>/dev/null || true)
  for i in "${!terms[@]}"; do
    term="${terms[$i]}"
    search="${term%%==>*}"
    [ -z "$search" ] && continue
    if printf '%s' "$content" | grep -qiF -- "$search"; then
      echo "redaction-check: possible leak in $file (term #$i)" >&2
      found=1
    fi
  done
done < <(git diff --cached --name-only)

if [ "$found" -ne 0 ]; then
  exit 1
fi

echo "redaction-check: clean"
