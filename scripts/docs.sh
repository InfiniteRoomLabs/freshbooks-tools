#!/usr/bin/env bash
# Regenerates docs/cli.md from the freshbooks CLI's cobra command tree via
# its hidden `docs` command (cli/internal/cmd/docs_cmd.go, gated behind the
# docsgen build tag so cobra/doc -- cli/internal/docsgen's only non-test
# importer -- never links into the release freshbooks binary; D6).
# Idempotent: running this twice with an unchanged command tree produces a
# byte-identical file (DisableAutoGenTag suppresses cobra/doc's date
# footer, and the tree is walked in a fixed, sorted order).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root/cli"

mise exec -- go run -tags docsgen ./cmd/freshbooks docs "$repo_root/docs/cli.md"

echo "docs: regenerated docs/cli.md"

# D5: README.md's Status column is generated from git tags by the release
# script. Refresh it here too, so `mise run docs` leaves no drift for
# `scripts/check.sh readme-drift-check` to catch (R9).
"$repo_root/scripts/release.sh" docs
