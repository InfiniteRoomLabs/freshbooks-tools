#!/usr/bin/env -S usage bash
# Regression test for scripts/release.sh. Every probe runs against a
# throwaway scratch repo under mktemp -d, with a bare local "origin" (no
# real GitHub remote) and fake `gh`/`go` binaries that log their argv and
# answer deterministically. `git`, `sha256sum`, and `tar` are the real
# binaries throughout -- only network-touching (gh) and toolchain (go)
# commands are faked, per docs/phases/9/plan.md D6. Nothing here reaches
# the network: GH_HOST=localhost, the fake gh never shells out, and every
# `git push` targets the bare repo under the scratch dir.
#
# Takes no arguments. Wired into `mise run check` beside
# redaction-selftest, and as `mise run release-selftest`.
#USAGE bin "release-selftest.sh"
#USAGE about "Regression test for scripts/release.sh; takes no arguments"

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
release_sh_src="$repo_root/scripts/release.sh"

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

case_n=0
failures=0
declare -a RELEASE_EXTRA_ENV=()
pass_msg() { printf 'release-selftest: PASS %s\n' "$1"; }
fail_msg() {
  printf 'release-selftest: FAIL %s -- %s\n' "$1" "$2" >&2
  failures=$((failures + 1))
}

# --- fake gh / go -----------------------------------------------------------

fakebin="$scratch/bin"
mkdir -p "$fakebin"

cat >"$fakebin/gh" <<'GHEOF'
#!/usr/bin/env bash
# Fake gh for scripts/release-selftest.sh. See that script for the env
# vars that control its answers. Logs every invocation to $FAKE_GH_LOG.
set -euo pipefail
: "${FAKE_GH_LOG:?}"
: "${FAKE_GH_REPO_DIR:?}"
: "${FAKE_GH_STATE:?}"
mkdir -p "$FAKE_GH_STATE"
printf '%s\n' "$*" >>"$FAKE_GH_LOG"

sub1="${1:-}"
sub2="${2:-}"
args=("$@")

jq_expr=""
branch=""
dir=""
i=0
while [ "$i" -lt "${#args[@]}" ]; do
  case "${args[$i]}" in
  --jq)
    jq_expr="${args[$((i + 1))]}"
    ;;
  --branch)
    branch="${args[$((i + 1))]}"
    ;;
  -D)
    dir="${args[$((i + 1))]}"
    ;;
  --workflow)
    printf '%s' "${args[$((i + 1))]}" >"$FAKE_GH_STATE/last-workflow"
    ;;
  esac
  i=$((i + 1))
done

emit_json() {
  # emit_json <json> -- prints raw JSON, or filters it through the
  # requested --jq expression with the real jq (present system-wide; gh
  # itself embeds one, so this mirrors gh's actual behavior).
  if [ -n "$jq_expr" ]; then
    printf '%s' "$1" | jq -cr "$jq_expr"
  else
    printf '%s\n' "$1"
  fi
}

case "$sub1 $sub2" in
"auth status")
  echo "  - Token scopes: 'repo', 'workflow', 'admin:public_key'"
  exit 0
  ;;
"run list")
  workflow=$(cat "$FAKE_GH_STATE/last-workflow" 2>/dev/null || echo CI)
  sha=$(git -C "$FAKE_GH_REPO_DIR" rev-parse "$branch" 2>/dev/null || echo "0000000000000000000000000000000000000000")
  if [ "$workflow" = "Release" ]; then
    conclusion="${FAKE_GH_RELEASE_CONCLUSION:-success}"
  else
    conclusion="${FAKE_GH_CI_CONCLUSION:-success}"
  fi
  json=$(printf '[{"databaseId":1001,"headSha":"%s","conclusion":"%s","status":"completed"}]' "$sha" "$conclusion")
  emit_json "$json"
  exit 0
  ;;
"run view")
  workflow=$(cat "$FAKE_GH_STATE/last-workflow" 2>/dev/null || echo CI)
  if [ "$workflow" = "Release" ]; then
    conclusion="${FAKE_GH_RELEASE_CONCLUSION:-success}"
  else
    conclusion="${FAKE_GH_CI_CONCLUSION:-success}"
  fi
  json=$(printf '{"status":"completed","conclusion":"%s"}' "$conclusion")
  emit_json "$json"
  exit 0
  ;;
"release view")
  tag="${args[2]:-}"
  module="${tag%%/*}"
  if [ "$module" = "freshbooks" ]; then
    assets=0
  else
    assets=13
  fi
  assets_json="["
  n=0
  while [ "$n" -lt "$assets" ]; do
    [ "$n" -gt 0 ] && assets_json+=","
    assets_json+="{\"name\":\"asset$n\"}"
    n=$((n + 1))
  done
  assets_json+="]"
  json=$(printf '{"isDraft":false,"name":"%s","assets":%s}' "$tag" "$assets_json")
  emit_json "$json"
  exit 0
  ;;
"release download")
  tag="${args[2]:-}"
  module="${tag%%/*}"
  version="${tag#*/v}"
  case "$module" in
  mcp) name="freshbooks-mcp" ;;
  cli) name="freshbooks" ;;
  *)
    echo "fake gh: release download for unexpected module $module" >&2
    exit 1
    ;;
  esac
  mkdir -p "$dir"
  workver="${FAKE_GH_ARCHIVE_VERSION:-$version}"
  stubdir=$(mktemp -d)
  if [ "$name" = "freshbooks-mcp" ]; then
    printf '#!/bin/sh\necho "freshbooks-mcp %s"\n' "$workver" >"$stubdir/$name"
  else
    printf '#!/bin/sh\necho "%s"\n' "$workver" >"$stubdir/$name"
  fi
  chmod +x "$stubdir/$name"
  tar czf "$dir/${name}_${version}_linux_amd64.tar.gz" -C "$stubdir" "$name"
  rm -rf "$stubdir"
  (cd "$dir" && sha256sum "${name}_${version}_linux_amd64.tar.gz" >checksums.txt)
  if [ -n "${FAKE_GH_CHECKSUM_CORRUPT:-}" ]; then
    sed -i 's/^[0-9a-f]\{16\}/0000000000000000/' "$dir/checksums.txt"
  fi
  exit 0
  ;;
*)
  echo "fake gh: unhandled invocation: $*" >&2
  exit 1
  ;;
esac
GHEOF
chmod +x "$fakebin/gh"

cat >"$fakebin/go" <<'GOEOF'
#!/usr/bin/env bash
# Fake go for scripts/release-selftest.sh (installed via RELEASE_GO_BIN,
# not PATH, since release.sh resolves the real toolchain by absolute
# path). Logs every invocation to $FAKE_GO_LOG.
set -euo pipefail
: "${FAKE_GO_LOG:?}"
printf '%s\n' "$*" >>"$FAKE_GO_LOG"

case "${1:-}" in
get)
  exit 0
  ;;
mod)
  [ "${2:-}" = "tidy" ] && exit 0
  echo "fake go: unhandled mod subcommand: $*" >&2
  exit 1
  ;;
list)
  # go list -m <path>@<version>
  target="${3:-}"
  path="${target%@*}"
  ver="${target#*@}"
  echo "$path $ver"
  exit 0
  ;;
install)
  target="${2:-}"
  path="${target%@*}"
  ver="${target#*@}" # e.g. v0.9.0
  bin="${path##*/}"
  outdir="${GOBIN:?GOBIN not set}"
  mkdir -p "$outdir"
  if [ "$bin" = "freshbooks-mcp" ]; then
    printline="freshbooks-mcp $ver"
  else
    printline="$ver"
  fi
  printf '#!/bin/sh\necho "%s"\n' "$printline" >"$outdir/$bin"
  chmod +x "$outdir/$bin"
  exit 0
  ;;
version)
  if [ "${2:-}" = "-m" ]; then
    echo "fake go version -m output -- no forbidden deps here"
    exit 0
  fi
  echo "fake go: unhandled version invocation: $*" >&2
  exit 1
  ;;
*)
  echo "fake go: unhandled invocation: $*" >&2
  exit 1
  ;;
esac
GOEOF
chmod +x "$fakebin/go"

# --- scratch repo scaffolding ------------------------------------------------

changelog_module() {
  cat <<EOF
# Changelog

All notable changes to this module are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
EOF
}

changelog_root() {
  cat <<EOF
# Changelog

## [Unreleased]

### Added
EOF
}

readme_stub() {
  cat <<EOF
# freshbooks-tools (selftest fixture)

| Module | Path | Binary/import | Status |
|---|---|---|---|
| Library | \`freshbooks/\` | \`github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks\` | \`freshbooks/v0.0.0\` |
| MCP server | \`mcp/\` | \`freshbooks-mcp\` | \`mcp/v0.0.0\` |
| CLI | \`cli/\` | \`freshbooks\` | \`cli/v0.0.0\` |
EOF
}

# new_scratch_repo <name> -- scaffolds a throwaway repo with the three
# module layouts, changelogs, README, a copy of scripts/release.sh under
# test, and a fake origin (a bare repo). Echoes the work dir path.
new_scratch_repo() {
  local n="$1" work origin
  work="$scratch/$n/work"
  origin="$scratch/$n/origin.git"
  mkdir -p "$work/freshbooks" "$work/mcp" "$work/cli" "$work/scripts"
  git init -q --bare "$origin"

  changelog_module >"$work/freshbooks/CHANGELOG.md"
  changelog_module >"$work/mcp/CHANGELOG.md"
  changelog_module >"$work/cli/CHANGELOG.md"
  changelog_root >"$work/CHANGELOG.md"
  readme_stub >"$work/README.md"

  # mcp/cli carry go.mod+go.sum in the real repo (bump's commit stages
  # them); the fake go never writes real module data, so these are
  # trackable stub files, not functional Go modules.
  printf 'module github.com/InfiniteRoomLabs/freshbooks-tools/mcp\n\ngo 1.26\n' >"$work/mcp/go.mod"
  printf '' >"$work/mcp/go.sum"
  printf 'module github.com/InfiniteRoomLabs/freshbooks-tools/cli\n\ngo 1.26\n' >"$work/cli/go.mod"
  printf '' >"$work/cli/go.sum"

  cp "$release_sh_src" "$work/scripts/release.sh"
  chmod +x "$work/scripts/release.sh"
  printf '#!/usr/bin/env bash\nexit 0\n' >"$work/scripts/check.sh"
  chmod +x "$work/scripts/check.sh"

  git -C "$work" init -q -b main >/dev/null 2>&1 || true # init -b main above already did this; harmless if already a repo
  git -C "$work" config user.email "selftest@example.com"
  git -C "$work" config user.name "release selftest"
  git -C "$work" config commit.gpgsign false
  git -C "$work" add -A
  git -C "$work" commit -q -m base
  git -C "$work" remote add origin "$origin"
  git -C "$work" push -q -u origin main

  echo "$work"
}

# release_run <work> <args...> -- invokes the scratch repo's copy of
# release.sh with the fake gh/go wiring plus any probe-specific env the
# caller staged into the global RELEASE_EXTRA_ENV array beforehand (bash
# cannot express `VAR=(array) cmd`, so it is not a one-liner prefix).
release_run() {
  local work="$1" state
  shift
  # State lives OUTSIDE the tracked work tree (a sibling dir) -- inside it,
  # these files would show up as untracked in `git status`, which would
  # make preflight-clean-tree (correctly) refuse to run on the second of
  # two back-to-back release_run calls against the same work dir.
  state="$(dirname "$work")/state"
  mkdir -p "$state"
  env \
    GH_HOST=localhost \
    RELEASE_GO_BIN="$fakebin/go" \
    RELEASE_LOCAL_BIN="$state/local-bin" \
    PATH="$fakebin:$PATH" \
    FAKE_GH_LOG="$state/fake-gh.log" \
    FAKE_GH_REPO_DIR="$work" \
    FAKE_GH_STATE="$state/fake-gh-state" \
    FAKE_GO_LOG="$state/fake-go.log" \
    "${RELEASE_EXTRA_ENV[@]}" \
    "$work/scripts/release.sh" "$@"
}

# --- probe: preflight fails on a dirty tree / non-main branch --------------

case_n=$((case_n + 1))
w=$(new_scratch_repo "preflight-dirty")
echo dirty >>"$w/README.md"
set +e
RELEASE_EXTRA_ENV=()
out=$(release_run "$w" preflight 2>&1)
status=$?
set -e
if [ "$status" -eq 1 ] && printf '%s' "$out" | grep -qF "release: FAIL preflight-clean-tree"; then
  pass_msg "preflight FAILs on a dirty tree"
else
  fail_msg "preflight FAILs on a dirty tree" "exit $status: $out"
fi

case_n=$((case_n + 1))
w=$(new_scratch_repo "preflight-branch")
git -C "$w" checkout -q -b not-main
set +e
RELEASE_EXTRA_ENV=()
out=$(release_run "$w" preflight 2>&1)
status=$?
set -e
if [ "$status" -eq 1 ] && printf '%s' "$out" | grep -qF "release: FAIL preflight-branch"; then
  pass_msg "preflight FAILs on a non-main branch"
else
  fail_msg "preflight FAILs on a non-main branch" "exit $status: $out"
fi

# --- probe: --version auto (patch for Fixed-only, minor for Added) --------

case_n=$((case_n + 1))
w=$(new_scratch_repo "auto-fixed")
{
  changelog_module
  echo
  echo "### Fixed"
  echo "- something"
} >"$w/freshbooks/CHANGELOG.md"
set +e
RELEASE_EXTRA_ENV=()
out=$(release_run "$w" cut freshbooks auto --yes --dry-run 2>&1)
status=$?
set -e
if [ "$status" -eq 0 ] && printf '%s' "$out" | grep -qF "release: OK version-propose -- freshbooks 0.0.0 -> 0.0.1 (patch)"; then
  pass_msg "--version auto proposes patch for a Fixed-only changelog"
else
  fail_msg "--version auto proposes patch for a Fixed-only changelog" "exit $status: $out"
fi

case_n=$((case_n + 1))
w=$(new_scratch_repo "auto-added")
{
  changelog_module
  echo
  echo "### Added"
  echo "- something new"
} >"$w/freshbooks/CHANGELOG.md"
set +e
RELEASE_EXTRA_ENV=()
out=$(release_run "$w" cut freshbooks auto --yes --dry-run 2>&1)
status=$?
set -e
if [ "$status" -eq 0 ] && printf '%s' "$out" | grep -qF "release: OK version-propose -- freshbooks 0.0.0 -> 0.1.0 (minor)"; then
  pass_msg "--version auto proposes minor for an Added changelog"
else
  fail_msg "--version auto proposes minor for an Added changelog" "exit $status: $out"
fi

# --- probe: all 0.9.0 --dry-run prints the full plan, zero pushes ---------

case_n=$((case_n + 1))
w=$(new_scratch_repo "dry-run-plan")
{
  changelog_module
  echo
  echo "### Added"
  echo "- v1"
} >"$w/freshbooks/CHANGELOG.md"
git -C "$w" add -A
git -C "$w" commit -q -m "seed unreleased notes"
git -C "$w" push -q origin main
before_refs=$(git -C "$w" ls-remote origin 2>&1)
set +e
RELEASE_EXTRA_ENV=()
out=$(release_run "$w" all 0.9.0 --dry-run 2>&1)
status=$?
set -e
after_refs=$(git -C "$w" ls-remote origin 2>&1)
if [ "$status" -eq 0 ] && [ "$before_refs" = "$after_refs" ] &&
  printf '%s' "$out" | grep -qF "dry-run: git" && ! printf '%s' "$out" | grep -qF "release: FAIL"; then
  pass_msg "all 0.9.0 --dry-run prints the full plan with zero pushes"
else
  fail_msg "all 0.9.0 --dry-run prints the full plan with zero pushes" "exit $status, refs changed=$([ "$before_refs" != "$after_refs" ] && echo yes || echo no): $out"
fi

# --- probe: all 0.9.0 --yes happy path + resume ----------------------------

case_n=$((case_n + 1))
w=$(new_scratch_repo "happy-path")
{
  changelog_module
  echo
  echo "### Added"
  echo "- v1"
} >"$w/freshbooks/CHANGELOG.md"
git -C "$w" add -A
git -C "$w" commit -q -m "seed unreleased notes"
git -C "$w" push -q origin main

set +e
RELEASE_EXTRA_ENV=()
out=$(release_run "$w" all 0.9.0 --yes 2>&1)
status=$?
set -e
origin_tags=$(git -C "$w" ls-remote --tags origin 2>&1)
if [ "$status" -eq 0 ] &&
  ! printf '%s' "$out" | grep -qF "release: FAIL" &&
  printf '%s' "$origin_tags" | grep -qF "refs/tags/freshbooks/v0.9.0" &&
  printf '%s' "$origin_tags" | grep -qF "refs/tags/mcp/v0.0.1" &&
  printf '%s' "$origin_tags" | grep -qF "refs/tags/cli/v0.0.1"; then
  pass_msg "all 0.9.0 --yes completes with every step OK and tags the fake origin"
else
  fail_msg "all 0.9.0 --yes completes with every step OK and tags the fake origin" "exit $status: $out"
fi

set +e
RELEASE_EXTRA_ENV=()
resume_out=$(release_run "$w" all 0.9.0 --yes 2>&1)
resume_status=$?
set -e
if [ "$resume_status" -eq 0 ] &&
  printf '%s' "$resume_out" | grep -qF "release: SKIP cut-commit" &&
  printf '%s' "$resume_out" | grep -qF "release: SKIP cut-tag-push" &&
  printf '%s' "$resume_out" | grep -qF "release: OK verify-release"; then
  pass_msg "re-running all prints SKIP for every mutating step and re-runs verify"
else
  fail_msg "re-running all prints SKIP for every mutating step and re-runs verify" "exit $resume_status: $resume_out"
fi

# --- probe: a failing Release run FAILs cut before the next module's tag --

case_n=$((case_n + 1))
w=$(new_scratch_repo "release-fails")
{
  changelog_module
  echo
  echo "### Added"
  echo "- v1"
} >"$w/freshbooks/CHANGELOG.md"
git -C "$w" add -A
git -C "$w" commit -q -m "seed unreleased notes"
git -C "$w" push -q origin main

set +e
RELEASE_EXTRA_ENV=(FAKE_GH_RELEASE_CONCLUSION=failure)
out=$(release_run "$w" all 0.9.0 --yes 2>&1)
status=$?
set -e
origin_tags=$(git -C "$w" ls-remote --tags origin 2>&1)
if [ "$status" -ne 0 ] &&
  printf '%s' "$out" | grep -qF "release: FAIL cut-release-watch" &&
  printf '%s' "$origin_tags" | grep -qF "refs/tags/freshbooks/v0.9.0" &&
  ! printf '%s' "$origin_tags" | grep -qE "refs/tags/(mcp|cli)/v"; then
  pass_msg "a failing Release run FAILs cut before the next module's tag push"
else
  fail_msg "a failing Release run FAILs cut before the next module's tag push" "exit $status: $out"
fi

# --- probe: verify FAILs on a bad version string / altered checksum -------

case_n=$((case_n + 1))
w=$(new_scratch_repo "verify-fail")
set +e
RELEASE_EXTRA_ENV=(FAKE_GH_ARCHIVE_VERSION=9.9.9)
out=$(release_run "$w" verify mcp/v0.1.0 2>&1)
status=$?
set -e
if [ "$status" -eq 1 ] && printf '%s' "$out" | grep -qF "release: FAIL verify-run"; then
  pass_msg "verify FAILs when the stub binary prints the wrong version string"
else
  fail_msg "verify FAILs when the stub binary prints the wrong version string" "exit $status: $out"
fi

case_n=$((case_n + 1))
set +e
RELEASE_EXTRA_ENV=(FAKE_GH_CHECKSUM_CORRUPT=1)
out=$(release_run "$w" verify mcp/v0.1.0 2>&1)
status=$?
set -e
if [ "$status" -eq 1 ] && printf '%s' "$out" | grep -qF "release: FAIL verify-checksum"; then
  pass_msg "verify FAILs when the checksum file is altered"
else
  fail_msg "verify FAILs when the checksum file is altered" "exit $status: $out"
fi

# ----------------------------------------------------------------------------

if [ "$failures" -ne 0 ]; then
  echo "release-selftest: $failures assertion(s) failed" >&2
  exit 1
fi

echo "release-selftest: OK"
