#!/usr/bin/env -S usage bash
#USAGE arg "<module_dir>" help="Module directory containing CHANGELOG.md, e.g. freshbooks"
#USAGE arg "<version>" help="Released version, e.g. 1.2.3 (no leading v)"

set -euo pipefail

changelog="$usage_module_dir/CHANGELOG.md"
if [ ! -f "$changelog" ]; then
  echo "changelog-section: no such file: $changelog" >&2
  exit 1
fi

awk -v ver="$usage_version" '
  BEGIN { found = 0; printing = 0 }
  /^## \[/ {
    if (printing) exit
    if ($0 ~ "^## \\[" ver "\\]") { printing = 1; found = 1; next }
    next
  }
  printing { print }
  END { if (!found) exit 1 }
' "$changelog"
