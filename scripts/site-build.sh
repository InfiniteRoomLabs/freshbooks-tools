#!/usr/bin/env bash
# Regenerates site/docs/ then builds the Docusaurus site: what
# `mise run site-build` runs directly, and what scripts/check.sh's
# repo-wide step calls to validate the docs site builds cleanly as part
# of the gate (onBrokenLinks / markdown.hooks.onBrokenMarkdownLinks:
# 'throw' catch a broken guide cross-link before it reaches GitHub
# Pages). D4: warm build measured ~5s locally, well under the 60s budget
# for joining the gate.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

"$repo_root/scripts/site-sync.sh"
mise exec -- pnpm --dir "$repo_root/site" install --frozen-lockfile
mise exec -- pnpm --dir "$repo_root/site" build

echo "site-build: OK"
