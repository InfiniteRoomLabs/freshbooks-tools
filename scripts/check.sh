#!/usr/bin/env -S usage bash
#USAGE arg "<subcommand>" help="fmt-check|vet|lint|test|cover|vuln|inventory-check|actionlint|shellcheck|redaction-selftest|release-selftest|readme-drift-check|site-build|repo-wide|build|docs|all"
#USAGE arg "[modules]" var=#true help="Modules to check (default: freshbooks mcp cli)"

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# A filter means "check this one module". The module-independent steps
# (actionlint, shellcheck, the two selftests, readme-drift-check) then do
# NOT run: CI invokes `mise run check -- <module>` once per module, and
# running them in each of the three jobs paid for the same ~30s of work
# three times (A12). CI gets them exactly once from the `repo-wide` job;
# a bare `mise run check` locally still runs everything.
if [ -z "${usage_modules:-}" ]; then
  modules=(freshbooks mcp cli)
  module_filter=false
else
  read -ra modules <<<"$usage_modules"
  module_filter=true
fi

run_fmt_check() {
  local module="$1" out
  echo "== fmt-check: $module =="
  out=$(cd "$repo_root/$module" && gofmt -l .)
  if [ -n "$out" ]; then
    echo "$out" >&2
    echo "fmt-check: $module has unformatted files" >&2
    return 1
  fi
}

run_vet() {
  local module="$1"
  echo "== vet: $module =="
  (cd "$repo_root/$module" && go vet ./...)
}

run_lint() {
  local module="$1"
  echo "== lint: $module =="
  (cd "$repo_root/$module" && golangci-lint run ./...)
}

run_test() {
  local module="$1"
  echo "== test: $module =="
  (cd "$repo_root/$module" && go test -race -coverprofile=coverage.out -covermode=atomic ./...)
  # docsgen is a no-op tag outside cli/ (only cli/internal/cmd's docs_cmd.go
  # and docs_test.go are //go:build docsgen); harmless to pass everywhere.
  (cd "$repo_root/$module" && go test -race -tags integration,docsgen ./...)
}

run_cover() {
  local module="$1"
  echo "== cover: $module =="
  if [ ! -f "$repo_root/$module/coverage.out" ]; then
    echo "cover: $repo_root/$module/coverage.out missing -- run the test step first" >&2
    return 1
  fi
  "$repo_root/scripts/coverage-gate.sh" 90 "$repo_root/$module/coverage.out"
}

run_vuln() {
  local module="$1"
  echo "== vuln: $module =="
  (cd "$repo_root/$module" && go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...)
}

run_inventory_check() {
  local module="$1"
  if [ "$module" != "freshbooks" ]; then
    echo "== inventory-check: $module (skipped -- only freshbooks has an inventory) =="
    return 0
  fi
  echo "== inventory-check: $module =="
  (cd "$repo_root/freshbooks" && go run ./internal/inventory -check ./...)
}

run_actionlint() {
  echo "== actionlint =="
  actionlint "$repo_root"/.github/workflows/*.yml
}

# scripts/ now carries release automation with push and tag power; two of
# the four blocking review findings on it (a dead `sha`, a dead `readme`)
# were things shellcheck flags for free (R15).
run_shellcheck() {
  echo "== shellcheck =="
  # SC1008: the `#!/usr/bin/env -S usage bash` shebang shellcheck does not
  # know. SC2154: $usage_* variables, set by the `usage` runtime.
  shellcheck -S warning -e SC1008,SC2154 "$repo_root"/scripts/*.sh
}

# Repo-wide, like actionlint: the redaction check is a security control, so
# its own regression test runs once per gate (Phase 8 security A1/A9).
run_redaction_selftest() {
  echo "== redaction-selftest =="
  "$repo_root/scripts/redaction-selftest.sh"
}

# scripts/release.sh is release automation with network-mutating power
# (git push, tag push); its regression test runs once per gate, entirely
# against scratch repos and fake gh/go (Phase 9 D6).
run_release_selftest() {
  echo "== release-selftest =="
  "$repo_root/scripts/release-selftest.sh"
}

run_build() {
  echo "== build =="
  local buildable=()
  for m in "${modules[@]}"; do
    case "$m" in mcp | cli) buildable+=("$m") ;; esac
  done
  if [ "${#buildable[@]}" -eq 0 ]; then
    echo "build: no buildable modules in the filter (${modules[*]}) -- skipping"
    return 0
  fi
  "$repo_root/scripts/build.sh" "${buildable[@]}"
}

run_docs() {
  echo "== docs =="
  "$repo_root/scripts/docs.sh"
}

# D4: the Docusaurus docs site build joins the repo-wide gate because a
# warm build measured well under the 60s budget (~5s locally); it catches
# a broken guide cross-link (onBrokenLinks / onBrokenMarkdownLinks:
# 'throw') before it ever reaches the GitHub Pages deploy workflow.
run_site_build() {
  echo "== site-build =="
  "$repo_root/scripts/site-build.sh"
}

# README.md's Status column (D5) is regenerated from git tags by
# `scripts/release.sh docs`, a pure, deterministic, local-only rewrite (no
# network write). It renders into a temp file via RELEASE_README_OUT and
# diffs, the same way docs_drift_test.go generates into a temp dir: a
# verification step must never mutate a tracked file. Rewriting README.md
# in place and reading `git diff` silently reverted an operator's
# uncommitted Status edit (and reported OK), and left a modified README
# behind whenever the check genuinely failed (A2/R8).
run_readme_drift_check() {
  echo "== readme-drift-check =="
  local rendered drift status=0
  rendered=$(mktemp)
  RELEASE_README_OUT="$rendered" "$repo_root/scripts/release.sh" docs || { rm -f "$rendered"; return 1; }
  drift=$(diff -u "$repo_root/README.md" "$rendered") || status=$?
  rm -f "$rendered"
  if [ "$status" -ne 0 ]; then
    echo "readme-drift-check: README.md Status column is stale -- run 'mise run release -- docs'" >&2
    echo "$drift" >&2
    return 1
  fi
}

# The module-independent steps, in gate order. Run once per gate: from a
# bare `scripts/check.sh all`, or from the `repo-wide` subcommand that
# .github/workflows/ci.yml calls in its own job.
run_repo_wide() {
  run_actionlint
  run_shellcheck
  run_redaction_selftest
  run_release_selftest
  run_readme_drift_check
  run_site_build
}

run_step() {
  local step="$1" module="$2"
  case "$step" in
  fmt-check) run_fmt_check "$module" ;;
  vet) run_vet "$module" ;;
  lint) run_lint "$module" ;;
  test) run_test "$module" ;;
  cover) run_cover "$module" ;;
  vuln) run_vuln "$module" ;;
  inventory-check) run_inventory_check "$module" ;;
  *)
    echo "check.sh: unknown subcommand: $step" >&2
    exit 2
    ;;
  esac
}

# The ordered step list for a single module's pass through the "all" gate.
# Single source of truth so the all-path and the single-step dispatcher
# above can never silently drift apart.
steps=(fmt-check vet lint test cover vuln inventory-check)

case "$usage_subcommand" in
actionlint) run_actionlint ;;
shellcheck) run_shellcheck ;;
repo-wide) run_repo_wide ;;
redaction-selftest) run_redaction_selftest ;;
release-selftest) run_release_selftest ;;
readme-drift-check) run_readme_drift_check ;;
build) run_build ;;
docs) run_docs ;;
site-build) run_site_build ;;
all)
  for module in "${modules[@]}"; do
    for step in "${steps[@]}"; do
      run_step "$step" "$module"
    done
  done
  run_build
  if [ "$module_filter" = false ]; then
    run_repo_wide
  else
    echo "== repo-wide steps skipped (module filter: ${modules[*]}) -- run 'mise run repo-wide' =="
  fi
  ;;
fmt-check | vet | lint | test | cover | vuln | inventory-check)
  for module in "${modules[@]}"; do
    run_step "$usage_subcommand" "$module"
  done
  ;;
*)
  echo "check.sh: unknown subcommand: $usage_subcommand" >&2
  exit 2
  ;;
esac

if [ "$usage_subcommand" = "all" ]; then
  # Exclude docs/phases/*/reports/: the QA lane writes its report while
  # this gate is still running, and that in-flight write is not the kind
  # of dirty tree this banner exists to catch (D8).
  dirty=$(cd "$repo_root" && git status --porcelain -- . ':(exclude)docs/phases/*/reports/*')
  if [ -n "$dirty" ]; then
    echo "DIRTY TREE:"
    echo "$dirty"
    exit 1
  fi
fi

echo "check.sh: $usage_subcommand OK"
