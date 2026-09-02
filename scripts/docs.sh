#!/usr/bin/env bash
# Regenerates docs/cli.md from the freshbooks CLI's cobra command tree via
# its hidden `docs` command (cli/internal/cmd/docs_cmd.go, docsgen.go).
# Idempotent: running this twice with an unchanged command tree produces a
# byte-identical file (DisableAutoGenTag suppresses cobra/doc's date
# footer, and the tree is walked in a fixed, sorted order).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root/cli"

mise exec -- go run ./cmd/freshbooks docs "$repo_root/docs/cli.md"

echo "docs: regenerated docs/cli.md"
