#!/usr/bin/env -S usage bash
#USAGE arg "<subcommand>" help="fmt-check|vet|lint|test|cover|vuln|inventory-check|actionlint|build|docs|all"
#USAGE arg "[modules]" var=#true help="Modules to check (default: freshbooks mcp cli)"

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ -z "${usage_modules:-}" ]; then
  modules=(freshbooks mcp cli)
else
  read -ra modules <<<"$usage_modules"
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
build) run_build ;;
docs) run_docs ;;
all)
  for module in "${modules[@]}"; do
    for step in "${steps[@]}"; do
      run_step "$step" "$module"
    done
  done
  run_actionlint
  run_build
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
  dirty=$(cd "$repo_root" && git status --porcelain)
  if [ -n "$dirty" ]; then
    echo "DIRTY TREE:"
    echo "$dirty"
    exit 1
  fi
fi

echo "check.sh: $usage_subcommand OK"
