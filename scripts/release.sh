#!/usr/bin/env -S usage bash
# Release automation for freshbooks-tools: cuts changelogs, tags, watches CI
# and Release workflow runs, verifies published releases, and rewrites the
# README Status column. See docs/building.md "Release flow" and
# docs/phases/9/plan.md for the full design.
#
# Output contract: every step prints exactly one of
#   release: OK <step>
#   release: SKIP <step> -- <reason>
#   release: FAIL <step> -- expected <x>, observed <y>
# and nothing else reaches stdout. Under --dry-run, the git/gh/go commands
# that a mutating step WOULD run are echoed as "dry-run: <command>" instead
# of being executed; read-only checks (status, gh view/list, changelog
# inspection) always run for real, dry-run or not. This script never reads
# FRESHBOOKS_* or GITHUB_TOKEN -- auth is entirely gh's own.
#USAGE flag "--dry-run" help="Print every mutating git/gh/go command instead of running it; read-only checks still run for real"
#USAGE flag "--yes" help="Skip the TTY confirmation before the first push"
#USAGE flag "--binary-version <version>" help="Explicit mcp/cli version for bump and all (default: auto-derived)"
#USAGE flag "--timeout <seconds>" help="Override the CI/Release run watch timeout (default 1200s)"
#USAGE arg "<subcommand>" help="preflight|cut|bump|verify|docs|all"
#USAGE arg "[args]" var=#true help="Subcommand-specific positional arguments (module/version/tag)"

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# --- global flags ------------------------------------------------------

DRY_RUN=false
[ "${usage_dry_run:-false}" = "true" ] && DRY_RUN=true

YES=false
[ "${usage_yes:-false}" = "true" ] && YES=true

BINARY_VERSION="${usage_binary_version:-}"
TIMEOUT="${usage_timeout:-1200}"

if [ -z "${usage_args:-}" ]; then
  args=()
else
  read -ra args <<<"$usage_args"
fi

# RELEASE_LOCAL_BIN is where the dogfood step installs released binaries.
# Overridable so the self-test never touches the real operator's
# ~/.local/bin.
RELEASE_LOCAL_BIN="${RELEASE_LOCAL_BIN:-$HOME/.local/bin}"

LOG_FILE="$(mktemp -t release-cmd.XXXXXX.log)"
trap 'rm -f "$LOG_FILE"' EXIT

# GO_BIN resolves the mise.toml-pinned go toolchain by absolute path, so it
# is correct even for commands run outside the repo (e.g. the proxy-pickup
# check in /tmp), where `mise exec` would fall back to the global pin.
# RELEASE_GO_BIN overrides it -- used only by scripts/release-selftest.sh
# to point at a fake `go` shim instead of resolving mise in a scratch repo
# that carries no real mise.toml of its own.
GO_BIN="${RELEASE_GO_BIN:-$(cd "$repo_root" && mise where go)/bin/go}"

# --- output contract helpers --------------------------------------------

step_ok() { printf 'release: OK %s\n' "$1"; }
step_skip() { printf 'release: SKIP %s -- %s\n' "$1" "$2"; }
step_fail() {
  printf 'release: FAIL %s -- expected %s, observed %s\n' "$1" "$2" "$3" >&2
  if [ -s "$LOG_FILE" ]; then
    echo "release: last command output:" >&2
    tail -n 40 "$LOG_FILE" >&2
  fi
  exit 1
}

dry_echo() { printf 'dry-run: %s\n' "$*"; }

# run_cmd executes a mutating command for real (output to LOG_FILE) unless
# DRY_RUN, in which case it only echoes the command and performs no write.
# Returns the command's exit status (0 in dry-run mode).
run_cmd() {
  if [ "$DRY_RUN" = true ]; then
    dry_echo "$*"
    return 0
  fi
  printf -- '--- %s\n' "$*" >>"$LOG_FILE"
  "$@" >>"$LOG_FILE" 2>&1
}

# read_cmd always executes for real, dry-run or not: it never writes.
read_cmd() {
  printf -- '--- %s\n' "$*" >>"$LOG_FILE"
  "$@" 2>>"$LOG_FILE"
}

# --- confirmation --------------------------------------------------------

CONFIRMED=false
confirm_first_push() {
  [ "$DRY_RUN" = true ] && return 0
  [ "$CONFIRMED" = true ] && return 0
  if [ "$YES" = true ]; then
    CONFIRMED=true
    return 0
  fi
  if [ -t 0 ]; then
    local ans
    read -r -p "release: about to push to origin/main and/or push a tag. Continue? [y/N] " ans
    if [ "$ans" = "y" ] || [ "$ans" = "Y" ]; then
      CONFIRMED=true
      return 0
    fi
    step_fail "confirm" "y at the TTY prompt or --yes" "declined"
  fi
  step_fail "confirm" "--yes or a TTY" "neither available (non-interactive run without --yes)"
}

# --- json helpers (gh has an embedded jq; no external jq dependency) ----

gh_jq() {
  # gh_jq <jq-expr> -- <gh args...>
  local expr="$1"
  shift
  [ "$1" = "--" ] && shift
  gh "$@" --jq "$expr" 2>>"$LOG_FILE"
}

# --- changelog helpers ----------------------------------------------------

# changelog_unreleased_section <file> -- echoes the lines strictly between
# "## [Unreleased]" and the next "## [" heading (exclusive of both).
changelog_unreleased_section() {
  awk '
    found && /^## \[/ { exit }
    found { print }
    /^## \[Unreleased\]/ { found = 1 }
  ' "$1"
}

# changelog_has_section <file> <version> -- true if "## [<version>]" exists.
changelog_has_section() {
  grep -qxF "## [$2]" "$1" 2>/dev/null || grep -qE "^## \\[$2\\]" "$1" 2>/dev/null
}

# commit_with_subject_exists <subject> -- D3 says "release commit already
# on main (subject match in `git log -1`) -> SKIP", but a plain `git log
# -1` only sees HEAD, which is wrong inside a single `all` run: by the
# time bump/all-ship check whether an EARLIER step's commit already
# happened, later commits have moved HEAD past it. Search all of main's
# history for the exact subject instead -- safe here because
# scripts/branch-protection.sh enforces required_linear_history, so a
# commit's subject, once on main, never moves or gets rewritten.
commit_with_subject_exists() {
  git -C "$repo_root" log --format=%s 2>/dev/null | grep -qxF "$1"
}

# derive_bump_kind <changelog-file> <current-version> -- echoes
# major|minor|patch|none per docs/phases/9/plan.md step 1: **Breaking:** or
# ### Changed -> minor while major==0 (else major); ### Added -> minor;
# only ### Fixed -> patch.
derive_bump_kind() {
  local file="$1" current="$2" section major minor patch
  section="$(changelog_unreleased_section "$file")"
  if [ -z "$(printf '%s' "$section" | tr -d '[:space:]')" ]; then
    echo "none"
    return
  fi
  IFS=. read -r major minor patch <<<"$current"
  if printf '%s\n' "$section" | grep -qF '**Breaking:**' ||
    printf '%s\n' "$section" | grep -qE '^### Changed'; then
    if [ "$major" = "0" ]; then echo minor; else echo major; fi
    return
  fi
  if printf '%s\n' "$section" | grep -qE '^### Added'; then
    echo minor
    return
  fi
  if printf '%s\n' "$section" | grep -qE '^### Fixed'; then
    echo patch
    return
  fi
  echo patch
}

# bump_version <current> <kind>
bump_version() {
  local current="$1" kind="$2" major minor patch
  IFS=. read -r major minor patch <<<"$current"
  case "$kind" in
  major) echo "$((major + 1)).0.0" ;;
  minor) echo "$major.$((minor + 1)).0" ;;
  patch) echo "$major.$minor.$((patch + 1))" ;;
  *)
    echo "derive_bump_kind: unknown kind $kind" >&2
    return 1
    ;;
  esac
}

# latest_tag_version <module> -- echoes the version (no "v") of the highest
# semver tag "<module>/vX.Y.Z", or empty if none exist.
latest_tag_version() {
  local module="$1" tag
  tag=$(git -C "$repo_root" tag --list "$module/v*" | sort -V | tail -1)
  [ -n "$tag" ] && echo "${tag#"$module"/v}"
}

# changelog_cut_section <file> <version> <date> -- renames "## [Unreleased]"
# to "## [<version>] - <date>" in place, keeping its body, and inserts a
# fresh empty "## [Unreleased]" above it.
changelog_cut_section() {
  local file="$1" version="$2" date="$3" tmp
  tmp=$(mktemp)
  awk -v ver="$version" -v date="$date" '
    BEGIN { done = 0; skip_blank = 0 }
    skip_blank == 1 {
      skip_blank = 0
      if ($0 == "") next
    }
    /^## \[Unreleased\]/ && !done {
      print
      print ""
      print "## [" ver "] - " date
      done = 1
      skip_blank = 1
      next
    }
    { print }
  ' "$file" >"$tmp"
  mv "$tmp" "$file"
}

# changelog_add_bullet <file> <heading> <text> -- adds "- <text>" under
# "### <heading>" inside the "## [Unreleased]" section, creating the
# heading if it is not already present.
changelog_add_bullet() {
  local file="$1" heading="$2" text="$3" before section after tmp
  tmp=$(mktemp)
  before=$(awk '/^## \[Unreleased\]/{print; exit} {print}' "$file")
  section=$(changelog_unreleased_section "$file")
  after=$(awk '
    started { print; next }
    /^## \[Unreleased\]/ { seen = 1; next }
    seen && /^## \[/ { started = 1; print }
  ' "$file")

  if printf '%s\n' "$section" | grep -qxF "### $heading"; then
    section=$(printf '%s\n' "$section" | awk -v h="### $heading" -v b="- $text" '
      { print }
      $0 == h && !done { print b; done = 1 }
    ')
  else
    section=$(printf '%s\n' "$section" | sed '/./,$!d') # drop leading blanks
    if [ -z "$(printf '%s' "$section" | tr -d '[:space:]')" ]; then
      section=$(printf '### %s\n- %s' "$heading" "$text")
    else
      section=$(printf '### %s\n- %s\n\n%s' "$heading" "$text" "$section")
    fi
  fi

  {
    printf '%s\n\n' "$before"
    printf '%s\n\n' "$section"
    printf '%s\n' "$after"
  } >"$tmp"
  mv "$tmp" "$file"
}

# --- module metadata -------------------------------------------------------

module_kind() {
  case "$1" in
  freshbooks) echo lib ;;
  mcp | cli) echo binary ;;
  *)
    echo "release: unknown module: $1" >&2
    return 1
    ;;
  esac
}

binary_name() {
  case "$1" in
  mcp) echo freshbooks-mcp ;;
  cli) echo freshbooks ;;
  *) return 1 ;;
  esac
}

module_go_path() {
  case "$1" in
  freshbooks) echo "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks" ;;
  mcp) echo "github.com/InfiniteRoomLabs/freshbooks-tools/mcp" ;;
  cli) echo "github.com/InfiniteRoomLabs/freshbooks-tools/cli" ;;
  esac
}

# --- CI / Release run watching (D4) ---------------------------------------

# watch_run <step> <workflow> <branch> [expect-sha] -- finds the most
# recent run of <workflow> filtered by <branch>, polls its conclusion every
# 30s up to TIMEOUT seconds. Always executes for real: watching is a read,
# never a write, so it is not gated by DRY_RUN.
watch_run() {
  local step="$1" workflow="$2" branch="$3" expect_sha="${4:-}"
  local run_json id sha waited=0 status conclusion

  run_json=$(read_cmd gh run list --workflow "$workflow" --branch "$branch" --limit 1 \
    --json databaseId,headSha,conclusion,status --jq '.[0] // empty') || true
  if [ -z "$run_json" ]; then
    step_fail "$step" "a $workflow run on $branch" "no run found"
  fi
  id=$(printf '%s' "$run_json" | grep -oE '"databaseId":[0-9]+' | grep -oE '[0-9]+')
  sha=$(printf '%s' "$run_json" | grep -oE '"headSha":"[^"]*"' | sed -E 's/.*:"(.*)"/\1/')
  if [ -n "$expect_sha" ] && [ "$sha" != "$expect_sha" ]; then
    step_fail "$step" "a $workflow run for $expect_sha" "latest run is for $sha"
  fi

  while :; do
    status=$(gh_jq '.status' -- run view "$id" --json status)
    conclusion=$(gh_jq '.conclusion' -- run view "$id" --json conclusion)
    if [ "$status" = "completed" ]; then
      if [ "$conclusion" = "success" ]; then
        step_ok "$step"
        return 0
      fi
      step_fail "$step" "conclusion=success" "conclusion=$conclusion"
    fi
    if [ "$waited" -ge "$TIMEOUT" ]; then
      step_fail "$step" "completed within ${TIMEOUT}s" "still $status after ${TIMEOUT}s"
    fi
    sleep 30
    waited=$((waited + 30))
  done
}

# require_main_unless_dry_run enforces D8: the script refuses to run when
# not on main, except for verify, docs, and --dry-run. Called by cut,
# bump, and all (which are the only subcommands D8 restricts).
require_main_unless_dry_run() {
  local branch
  branch=$(git -C "$repo_root" rev-parse --abbrev-ref HEAD)
  if [ "$branch" != "main" ] && [ "$DRY_RUN" != true ]; then
    step_fail "branch-guard" "main" "$branch"
  fi
}

# --- preflight --------------------------------------------------------------

cmd_preflight() {
  local branch dirty auth_out sha ci_conclusion

  branch=$(git -C "$repo_root" rev-parse --abbrev-ref HEAD)
  if [ "$branch" = "main" ]; then
    step_ok "preflight-branch"
  elif [ "$DRY_RUN" = true ]; then
    step_skip "preflight-branch" "not on main ($branch), continuing under --dry-run"
  else
    step_fail "preflight-branch" "main" "$branch"
  fi

  dirty=$(git -C "$repo_root" status --porcelain)
  if [ -z "$dirty" ]; then
    step_ok "preflight-clean-tree"
  else
    step_fail "preflight-clean-tree" "clean working tree" "dirty tree"
  fi

  if auth_out=$(read_cmd gh auth status); then
    if printf '%s' "$auth_out" | grep -q "'repo'" && printf '%s' "$auth_out" | grep -q "'workflow'"; then
      step_ok "preflight-gh-auth"
    else
      step_fail "preflight-gh-auth" "repo+workflow scopes" "scopes missing"
    fi
  else
    step_fail "preflight-gh-auth" "gh authenticated" "gh auth status failed"
  fi

  sha=$(git -C "$repo_root" rev-parse HEAD)
  ci_conclusion=$(gh_jq '.[0].conclusion // "none"' -- run list --workflow CI --branch main --limit 1 --json headSha,conclusion) || ci_conclusion="none"
  if [ "$ci_conclusion" = "success" ]; then
    step_ok "preflight-ci-green"
  else
    step_fail "preflight-ci-green" "success" "$ci_conclusion"
  fi

  if read_cmd mise which go >/dev/null; then
    step_ok "preflight-mise-install"
  else
    step_fail "preflight-mise-install" "mise.toml toolchain resolvable" "mise which go failed -- run mise install"
  fi

  # D8: never edits branch protection or rulesets -- print the gh api call
  # that would apply/inspect the tag ruleset, unconditionally, and stop.
  dry_echo "gh api repos/InfiniteRoomLabs/freshbooks-tools/rulesets (tag ruleset for refs/tags/{freshbooks,mcp,cli}/v*, warn-only, never applied by this script)"
  step_ok "preflight-tag-ruleset-warn"
}

# --- docs (D5) --------------------------------------------------------------

cmd_docs() {
  local module tag readme=".README.md" tmp
  readme="$repo_root/README.md"
  for module in freshbooks mcp cli; do
    tag=$(git -C "$repo_root" tag --list "$module/v*" | sort -V | tail -1)
    [ -z "$tag" ] && continue
    if [ "$DRY_RUN" = true ]; then
      dry_echo "rewrite README.md Status cell for $module -> $tag"
      continue
    fi
    tmp=$(mktemp)
    # Matches the module's Status row: "| <Name> | `<module>/` | <import> | `<old-tag>` |"
    # and rewrites only the last cell (the Status column).
    awk -v mod="\`${module}/\`" -v tag="$tag" '
      {
        line = $0
        if (line ~ ("\\| " mod " \\|")) {
          sub(/\|[^|]*\|[ \t]*$/, "| `" tag "` |", line)
        }
        print line
      }
    ' "$readme" >"$tmp"
    mv "$tmp" "$readme"
  done
  step_ok "docs"
}

# --- verify (D1 verify subcommand; also called by cut/all -- D3: never skips) ---

do_verify() {
  local tag="$1" module version kind json assets isdraft name
  module="${tag%%/*}"
  version="${tag#*/v}"
  kind=$(module_kind "$module") || step_fail "verify-tag" "a tag shaped <module>/vX.Y.Z" "$tag"

  json=$(read_cmd gh release view "$tag" --json isDraft,name,assets) ||
    step_fail "verify-release" "a published release for $tag" "gh release view failed"
  isdraft=$(printf '%s' "$json" | jq -r '.isDraft' 2>>"$LOG_FILE") ||
    step_fail "verify-release" "valid JSON from gh release view" "unparsable"
  name=$(printf '%s' "$json" | jq -r '.name' 2>>"$LOG_FILE") ||
    step_fail "verify-release" "valid JSON from gh release view" "unparsable"
  assets=$(printf '%s' "$json" | jq -r '.assets | length' 2>>"$LOG_FILE") ||
    step_fail "verify-release" "valid JSON from gh release view" "unparsable"

  if [ "$isdraft" != "false" ]; then
    step_fail "verify-release" "isDraft=false" "isDraft=$isdraft"
  fi
  if [ "$name" != "$tag" ]; then
    step_fail "verify-release" "name=$tag" "name=$name"
  fi
  if [ "$kind" = "lib" ] && [ "$assets" != "0" ]; then
    step_fail "verify-release" "0 assets" "$assets assets"
  fi
  if [ "$kind" = "binary" ] && [ "$assets" != "13" ]; then
    step_fail "verify-release" "13 assets" "$assets assets"
  fi
  step_ok "verify-release"

  if [ "$kind" = "lib" ]; then
    do_verify_proxy_pickup "$module" "$version"
  else
    do_verify_binary "$module" "$version" "$tag"
  fi
}

# do_verify_proxy_pickup polls the Go module proxy for up to 6 tries, 20s
# apart, using the pinned toolchain by absolute path (mise exec outside
# the repo would resolve the global pin instead -- plan step 8).
do_verify_proxy_pickup() {
  local module="$1" version="$2" path try=0 out
  path=$(module_go_path "$module")
  while :; do
    try=$((try + 1))
    if out=$(cd /tmp && GOFLAGS=-mod=mod "$GO_BIN" list -m "${path}@v${version}" 2>>"$LOG_FILE"); then
      if printf '%s' "$out" | grep -qF "v${version}"; then
        step_ok "verify-proxy-pickup"
        return 0
      fi
    fi
    if [ "$try" -ge 6 ]; then
      step_fail "verify-proxy-pickup" "$path@v$version resolvable after 6 tries" "still unresolved"
    fi
    sleep 20
  done
}

do_verify_binary() {
  local module="$1" version="$2" tag="$3" name workdir archive out_version want_version
  name=$(binary_name "$module")
  workdir=$(mktemp -d)

  read_cmd gh release download "$tag" \
    --pattern "${name}_${version}_linux_amd64.tar.gz" \
    --pattern "checksums.txt" \
    -D "$workdir" >/dev/null ||
    step_fail "verify-download" "release assets for $tag" "gh release download failed"
  archive="$workdir/${name}_${version}_linux_amd64.tar.gz"
  if [ ! -f "$archive" ] || [ ! -f "$workdir/checksums.txt" ]; then
    step_fail "verify-download" "archive + checksums.txt" "missing file(s) in $workdir"
  fi
  step_ok "verify-download"

  if (cd "$workdir" && sha256sum -c checksums.txt --ignore-missing) >>"$LOG_FILE" 2>&1; then
    step_ok "verify-checksum"
  else
    step_fail "verify-checksum" "sha256sum -c to pass" "checksum mismatch"
  fi

  tar xzf "$archive" -C "$workdir" ||
    step_fail "verify-extract" "archive to extract" "tar xzf failed"
  if [ ! -x "$workdir/$name" ]; then
    chmod +x "$workdir/$name" 2>/dev/null || true
  fi
  out_version=$("$workdir/$name" version 2>>"$LOG_FILE") || step_fail "verify-run" "the extracted binary to run" "exec failed"
  case "$module" in
  mcp) want_version="freshbooks-mcp $version" ;;
  cli) want_version="$version" ;;
  esac
  if [ "$out_version" != "$want_version" ]; then
    step_fail "verify-run" "$want_version" "$out_version"
  fi
  step_ok "verify-run"

  local gobin_dir install_path installed_version want_installed
  gobin_dir=$(mktemp -d)
  install_path="$(module_go_path "$module")/cmd/${name}"
  if (cd /tmp && GOBIN="$gobin_dir" GOFLAGS=-mod=mod "$GO_BIN" install "${install_path}@v${version}") >>"$LOG_FILE" 2>&1; then
    step_ok "verify-go-install"
  else
    step_fail "verify-go-install" "go install ${install_path}@v${version} to succeed" "go install failed"
  fi
  installed_version=$("$gobin_dir/$name" version 2>>"$LOG_FILE") || step_fail "verify-go-install-run" "the go-installed binary to run" "exec failed"
  case "$module" in
  mcp) want_installed="freshbooks-mcp v$version" ;;
  cli) want_installed="v$version" ;;
  esac
  if [ "$installed_version" != "$want_installed" ]; then
    step_fail "verify-go-install-run" "$want_installed" "$installed_version"
  fi
  step_ok "verify-go-install-run"

  if [ "$module" = "cli" ]; then
    local modinfo
    modinfo=$("$GO_BIN" version -m "$gobin_dir/$name" 2>>"$LOG_FILE") || step_fail "verify-cli-no-md2man" "go version -m to succeed" "failed"
    if printf '%s' "$modinfo" | grep -qiE 'md2man|blackfriday'; then
      step_fail "verify-cli-no-md2man" "no md2man/blackfriday in build info" "found"
    fi
    step_ok "verify-cli-no-md2man"
  fi

  # Dogfood: the operator's local install becomes the released (goreleaser)
  # build, not the go-install fallback-versioned one. A local-machine write,
  # so it is gated by DRY_RUN like any other mutation.
  if [ "$DRY_RUN" = true ]; then
    dry_echo "install $workdir/$name -> $RELEASE_LOCAL_BIN/$name"
  else
    mkdir -p "$RELEASE_LOCAL_BIN"
    cp "$workdir/$name" "$RELEASE_LOCAL_BIN/$name"
    chmod +x "$RELEASE_LOCAL_BIN/$name"
  fi
  step_ok "verify-dogfood"

  rm -rf "$workdir" "$gobin_dir"
}

cmd_verify() {
  local tag="${args[0]:-}"
  [ -z "$tag" ] && step_fail "verify" "a <tag> argument" "none given"
  do_verify "$tag"
}

# verify_after_cut wraps do_verify for cut/all's internal use: under
# --dry-run the tag/release genuinely was not created (zero writes), so a
# real verification attempt would always and uninformatively FAIL. The
# standalone `verify <tag>` subcommand (cmd_verify, above) always runs
# do_verify for real regardless of --dry-run -- it has nothing to check
# but already-published state, and no mutation to suppress except the
# dogfood copy, which do_verify itself already gates on DRY_RUN.
verify_after_cut() {
  local tag="$1"
  if [ "$DRY_RUN" = true ]; then
    dry_echo "verify $tag (release view, checksum/extract/run, go install, dogfood)"
    step_skip "verify" "dry-run -- $tag was not actually cut"
    return 0
  fi
  do_verify "$tag"
}

# --- cut <module> <version> -------------------------------------------------

cmd_cut() {
  local module="${args[0]:-}" version="${args[1]:-}" kind
  [ -z "$module" ] || [ -z "$version" ] && step_fail "cut" "cut <module> <version>" "missing argument(s)"
  kind=$(module_kind "$module") || step_fail "cut" "module in {freshbooks,mcp,cli}" "$module"
  require_main_unless_dry_run

  if [ "$version" = "auto" ]; then
    accept_proposed_version "$module"
    version="$ACCEPTED_VERSION"
  fi

  local tag="$module/v$version"
  if [ "$kind" = "lib" ]; then
    cut_lib "$module" "$version" "$tag"
  else
    cut_binary "$module" "$version" "$tag"
  fi
  verify_after_cut "$tag"
}

# propose_version <module> -- pure: computes the next version via
# derive_bump_kind against the module's own CHANGELOG.md Unreleased
# section and echoes "<next> <kind> <current>". Safe to call via $(...).
propose_version() {
  local module="$1" current changelog kind next
  current=$(latest_tag_version "$module")
  [ -z "$current" ] && current="0.0.0"
  changelog="$repo_root/$module/CHANGELOG.md"
  kind=$(derive_bump_kind "$changelog" "$current")
  [ "$kind" = "none" ] && return 1
  next=$(bump_version "$current" "$kind")
  printf '%s %s %s\n' "$next" "$kind" "$current"
}

# accept_proposed_version <module> -- computes the proposal, prints the
# "release: OK version-propose" line for real (it must NOT be called via
# $(...), or that line would be captured instead of shown), gates
# acceptance behind --yes or a TTY "y", and leaves the accepted version in
# the global ACCEPTED_VERSION.
ACCEPTED_VERSION=""
accept_proposed_version() {
  local module="$1" proposal next kind current ans
  proposal=$(propose_version "$module") ||
    step_fail "version-propose" "a non-empty [Unreleased] section in $module/CHANGELOG.md" "empty"
  read -r next kind current <<<"$proposal"
  printf 'release: OK version-propose -- %s %s -> %s (%s)\n' "$module" "$current" "$next" "$kind"
  if [ "$YES" = true ]; then
    ACCEPTED_VERSION="$next"
    return 0
  fi
  if [ -t 0 ]; then
    read -r -p "release: accept $module $next? [y/N] " ans
    if [ "$ans" = "y" ] || [ "$ans" = "Y" ]; then
      ACCEPTED_VERSION="$next"
      return 0
    fi
    step_fail "version-propose" "y at the TTY prompt or --yes" "declined"
  fi
  step_fail "version-propose" "--yes or a TTY to accept $next" "neither available"
}

# cut_lib implements the library release flow (plan steps 1-8): changelog
# cut, root changelog line, commit, push, CI watch, tag, tag push, Release
# watch. Each mutating step checks its own postcondition first (D3).
cut_lib() {
  # NOTE: `local a=$1 b=$a` does NOT see the just-assigned a -- all RHS
  # expansions in one `local` statement happen before any of it takes
  # effect, so a same-line reference to an earlier name in the same
  # statement falls through to dynamic scope (the caller's variable of
  # that name, or unbound). changelog must be its own statement.
  local module="$1" version="$2" tag="$3" today
  local changelog="$repo_root/$module/CHANGELOG.md"
  today=$(date -u +%F)

  if changelog_has_section "$changelog" "$version"; then
    step_skip "cut-changelog" "$changelog already has ## [$version]"
  else
    if [ "$DRY_RUN" = true ]; then
      dry_echo "changelog_cut_section $changelog $version $today"
      dry_echo "changelog_add_bullet $repo_root/CHANGELOG.md Added \"$module cut to $version, ahead of the $tag tag.\""
    else
      changelog_cut_section "$changelog" "$version" "$today"
      changelog_add_bullet "$repo_root/CHANGELOG.md" "Added" "\`$module\` cut to $version, ahead of the \`$tag\` tag."
    fi
    step_ok "cut-changelog"
  fi

  local expect_subject="release($module): v$version"
  if commit_with_subject_exists "$expect_subject"; then
    step_skip "cut-commit" "a commit \"$expect_subject\" is already on main"
  else
    confirm_first_push
    run_cmd git -C "$repo_root" add "$module/CHANGELOG.md" CHANGELOG.md ||
      step_fail "cut-commit" "git add to succeed" "failed"
    run_cmd git -C "$repo_root" commit -m "$expect_subject" ||
      step_fail "cut-commit" "git commit to succeed" "failed"
    step_ok "cut-commit"

    confirm_first_push
    run_cmd git -C "$repo_root" push origin main ||
      step_fail "cut-push" "git push origin main to succeed" "failed"
    step_ok "cut-push"
  fi

  if [ "$DRY_RUN" = true ]; then
    dry_echo "watch CI on main for the release commit"
    step_skip "cut-ci-watch" "dry-run"
  else
    local sha
    sha=$(git -C "$repo_root" rev-parse HEAD)
    watch_run "cut-ci-watch" "CI" "main" "$sha"
  fi

  if git -C "$repo_root" ls-remote --tags origin "refs/tags/$tag" 2>>"$LOG_FILE" | grep -q "$tag"; then
    step_skip "cut-tag-push" "$tag already on origin"
  else
    confirm_first_push
    if ! git -C "$repo_root" rev-parse "$tag" >/dev/null 2>&1; then
      run_cmd git -C "$repo_root" tag -a "$tag" -m "$module $version" ||
        step_fail "cut-tag" "git tag -a $tag to succeed" "failed"
    fi
    step_ok "cut-tag"
    run_cmd git -C "$repo_root" push origin "$tag" ||
      step_fail "cut-tag-push" "git push origin $tag to succeed" "failed"
    step_ok "cut-tag-push"
  fi

  if [ "$DRY_RUN" = true ]; then
    dry_echo "watch the Release workflow for $tag"
    step_skip "cut-release-watch" "dry-run"
  else
    watch_run "cut-release-watch" "Release" "$tag"
  fi
}

# cut_binary implements the binary release flow (plan steps 12-16, minus
# the bump commit, which is the `bump` subcommand's job): tag, tag push,
# Release watch. do_verify (called by the caller) covers steps 14-17.
cut_binary() {
  local module="$1" version="$2" tag="$3"

  if git -C "$repo_root" ls-remote --tags origin "refs/tags/$tag" 2>>"$LOG_FILE" | grep -q "$tag"; then
    step_skip "cut-tag-push" "$tag already on origin"
  else
    confirm_first_push
    if ! git -C "$repo_root" rev-parse "$tag" >/dev/null 2>&1; then
      run_cmd git -C "$repo_root" tag -a "$tag" -m "$module $version" ||
        step_fail "cut-tag" "git tag -a $tag to succeed" "failed"
    fi
    step_ok "cut-tag"
    run_cmd git -C "$repo_root" push origin "$tag" ||
      step_fail "cut-tag-push" "git push origin $tag to succeed" "failed"
    step_ok "cut-tag-push"
  fi

  if [ "$DRY_RUN" = true ]; then
    dry_echo "watch the Release workflow for $tag"
    step_skip "cut-release-watch" "dry-run"
  else
    watch_run "cut-release-watch" "Release" "$tag"
  fi
}

# --- bump <lib-version> [--binary-version A.B.C] ----------------------------

cmd_bump() {
  local lib_version="${args[0]:-}" binary_version="$BINARY_VERSION"
  [ -z "$lib_version" ] && step_fail "bump" "bump <lib-version>" "missing argument"
  require_main_unless_dry_run
  if [ -z "$binary_version" ]; then
    binary_version=$(propose_binary_version)
  fi
  do_bump "$lib_version" "$binary_version"
}

# propose_binary_version defaults to a patch bump of the shared mcp/cli
# version (mcp and cli release in lockstep, one version between them --
# see docs/phases/9/plan.md; historical precedent is 0.1.0 -> 0.1.1 ->
# 0.1.2, patch-only, for both a dependency-only bump and an Added tool).
propose_binary_version() {
  local current
  current=$(latest_tag_version "mcp")
  [ -z "$current" ] && current=$(latest_tag_version "cli")
  [ -z "$current" ] && current="0.0.0"
  bump_version "$current" "patch"
}

# do_bump implements plan steps 9-11: go get + go mod tidy in mcp/ and
# cli/, a "Requires freshbooks vX.Y.Z" Changed line plus the changelog
# cut for both modules (sharing one version), the root changelog line, the
# per-module fmt-check/vet/lint/test/cover gate (never the dirty-tree
# banner -- the staged bump is expected to be dirty until committed), one
# shared commit, push, CI watch.
do_bump() {
  local lib_version="$1" binary_version="$2" module today
  today=$(date -u +%F)

  for module in mcp cli; do
    local changelog="$repo_root/$module/CHANGELOG.md" gopath="$repo_root/$module"
    if changelog_has_section "$changelog" "$binary_version"; then
      step_skip "bump-go-get-$module" "$changelog already has ## [$binary_version]"
      continue
    fi

    if [ "$DRY_RUN" = true ]; then
      dry_echo "cd $gopath && go get $(module_go_path freshbooks)@v$lib_version && go mod tidy"
      step_skip "bump-go-get-$module" "dry-run"
    else
      local try=0 ok=false
      while [ "$try" -lt 6 ]; do
        try=$((try + 1))
        if (cd "$gopath" && "$GO_BIN" get "$(module_go_path freshbooks)@v${lib_version}") >>"$LOG_FILE" 2>&1 &&
          (cd "$gopath" && "$GO_BIN" mod tidy) >>"$LOG_FILE" 2>&1; then
          ok=true
          break
        fi
        sleep 20
      done
      if [ "$ok" = true ]; then
        step_ok "bump-go-get-$module"
      else
        step_fail "bump-go-get-$module" "go get freshbooks@v$lib_version to succeed within 6 tries" "failed"
      fi
    fi

    if [ "$DRY_RUN" = true ]; then
      dry_echo "changelog_add_bullet $changelog Changed \"Requires \`freshbooks\` v$lib_version\""
      dry_echo "changelog_cut_section $changelog $binary_version $today"
      dry_echo "changelog_add_bullet $repo_root/CHANGELOG.md Added \"$module cut to $binary_version, ahead of the $module/v$binary_version tag.\""
    else
      changelog_add_bullet "$changelog" "Changed" "Requires \`freshbooks\` v$lib_version"
      changelog_cut_section "$changelog" "$binary_version" "$today"
      changelog_add_bullet "$repo_root/CHANGELOG.md" "Added" "\`$module\` cut to $binary_version, ahead of the \`$module/v$binary_version\` tag."
    fi
  done

  # Per-module fmt-check/vet/lint/test/cover, never the "all" dirty-tree
  # banner (the staged bump is expected to be dirty -- plan step 11).
  for module in mcp cli; do
    local step
    for step in fmt-check vet lint test cover; do
      if [ "$DRY_RUN" = true ]; then
        dry_echo "scripts/check.sh $step $module"
      else
        "$repo_root/scripts/check.sh" "$step" "$module" >>"$LOG_FILE" 2>&1 ||
          step_fail "bump-check-$module" "scripts/check.sh $step $module to pass" "failed"
      fi
    done
    step_ok "bump-check-$module"
  done

  local expect_subject="release(mcp,cli): require freshbooks v$lib_version and cut $binary_version"
  if commit_with_subject_exists "$expect_subject"; then
    step_skip "bump-commit" "a commit \"$expect_subject\" is already on main"
  else
    confirm_first_push
    run_cmd git -C "$repo_root" add mcp/go.mod mcp/go.sum mcp/CHANGELOG.md cli/go.mod cli/go.sum cli/CHANGELOG.md CHANGELOG.md ||
      step_fail "bump-commit" "git add to succeed" "failed"
    run_cmd git -C "$repo_root" commit -m "$expect_subject" ||
      step_fail "bump-commit" "git commit to succeed" "failed"
    step_ok "bump-commit"

    confirm_first_push
    run_cmd git -C "$repo_root" push origin main ||
      step_fail "bump-push" "git push origin main to succeed" "failed"
    step_ok "bump-push"
  fi

  if [ "$DRY_RUN" = true ]; then
    dry_echo "watch CI on main for the bump commit"
    step_skip "bump-ci-watch" "dry-run"
  else
    local sha
    sha=$(git -C "$repo_root" rev-parse HEAD)
    watch_run "bump-ci-watch" "CI" "main" "$sha"
  fi
}

# --- all <lib-version> [--binary-version A.B.C] -----------------------------

cmd_all() {
  local lib_version="${args[0]:-}" binary_version="$BINARY_VERSION"
  [ -z "$lib_version" ] && step_fail "all" "all <lib-version>" "missing argument"
  require_main_unless_dry_run

  cmd_preflight

  if [ "$lib_version" = "auto" ]; then
    accept_proposed_version "freshbooks"
    lib_version="$ACCEPTED_VERSION"
  fi

  cut_lib "freshbooks" "$lib_version" "freshbooks/v$lib_version"
  verify_after_cut "freshbooks/v$lib_version"

  if [ -z "$binary_version" ]; then
    binary_version=$(propose_binary_version)
  fi
  do_bump "$lib_version" "$binary_version"

  cut_binary "mcp" "$binary_version" "mcp/v$binary_version"
  verify_after_cut "mcp/v$binary_version"
  cut_binary "cli" "$binary_version" "cli/v$binary_version"
  verify_after_cut "cli/v$binary_version"

  cmd_docs

  # docs/progress.md's ledger row is free-form narrative prose (see
  # existing entries) -- intentionally left to the operator to write and
  # commit by hand, before or after this run. This script only guarantees
  # the machine-checkable parts (changelogs, README Status column) stay
  # correct on every run, resumed or not.

  local expect_subject="docs: ship v$lib_version"
  if commit_with_subject_exists "$expect_subject"; then
    step_skip "all-ship-commit" "a commit \"$expect_subject\" is already on main"
  else
    confirm_first_push
    run_cmd git -C "$repo_root" add README.md ||
      step_fail "all-ship-commit" "git add to succeed" "failed"
    run_cmd git -C "$repo_root" commit -m "$expect_subject" ||
      step_fail "all-ship-commit" "git commit to succeed" "failed"
    step_ok "all-ship-commit"

    confirm_first_push
    run_cmd git -C "$repo_root" push origin main ||
      step_fail "all-ship-push" "git push origin main to succeed" "failed"
    step_ok "all-ship-push"
  fi

  if [ "$DRY_RUN" = true ]; then
    dry_echo "watch CI on main for the ship commit"
    step_skip "all-ship-ci-watch" "dry-run"
  else
    local sha
    sha=$(git -C "$repo_root" rev-parse HEAD)
    watch_run "all-ship-ci-watch" "CI" "main" "$sha"
  fi
}

# --- main dispatch -----------------------------------------------------------

case "$usage_subcommand" in
preflight) cmd_preflight ;;
docs) cmd_docs ;;
verify) cmd_verify ;;
cut) cmd_cut ;;
bump) cmd_bump ;;
all) cmd_all ;;
*)
  echo "release.sh: unknown subcommand: $usage_subcommand" >&2
  exit 2
  ;;
esac
