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

# Poll intervals in seconds. Overridable so scripts/release-selftest.sh can
# exercise the discovery retry (F2/R2) without sleeping through the real
# intervals; nothing else sets them.
DISCOVERY_INTERVAL="${RELEASE_DISCOVERY_INTERVAL:-15}"
POLL_INTERVAL="${RELEASE_POLL_INTERVAL:-30}"

if [ -z "${usage_args:-}" ]; then
  args=()
else
  read -ra args <<<"$usage_args"
fi

# RELEASE_LOCAL_BIN is where the dogfood step installs released binaries.
# Overridable so the self-test never touches the real operator's
# ~/.local/bin.
RELEASE_LOCAL_BIN="${RELEASE_LOCAL_BIN:-$HOME/.local/bin}"

# One EXIT trap owns every temp path this script creates (R13/A4): the
# verify workdirs and the changelog/README scratch files used to leak on
# every step_fail path between their mktemp and the success-path rm.
LOG_FILE="$(mktemp -t release-cmd.XXXXXX.log)"
declare -a TMP_PATHS=("$LOG_FILE")
track_tmp() { TMP_PATHS+=("$1"); }
cleanup_tmp() { rm -rf "${TMP_PATHS[@]}"; }
trap cleanup_tmp EXIT

# GO_BIN resolves the mise.toml-pinned go toolchain by absolute path, so it
# is correct even for commands run outside the repo (e.g. the proxy-pickup
# check in /tmp), where `mise exec` would fall back to the global pin.
# RELEASE_GO_BIN overrides it -- used only by scripts/release-selftest.sh
# to point at a fake `go` shim instead of resolving mise in a scratch repo
# that carries no real mise.toml of its own.
#
# Resolved lazily and memoised (S12): as a top-level command substitution
# it ran `mise where go` for `preflight` and `docs` too -- including on
# every `mise run check`, via readme-drift-check -- and a failure there
# killed the script before any `release:` line could be printed, the one
# place the D2 output contract could be violated silently.
GO_BIN=""
ensure_go_bin() {
  [ -n "$GO_BIN" ] && return 0
  if [ -n "${RELEASE_GO_BIN:-}" ]; then
    GO_BIN="$RELEASE_GO_BIN"
    return 0
  fi
  local where
  where=$(cd "$repo_root" && mise where go 2>>"$LOG_FILE") ||
    step_fail "go-toolchain" "mise where go to resolve the mise.toml pin" "mise where go failed -- run mise install"
  GO_BIN="$where/bin/go"
}

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

# step_note carries information that is not a step outcome: the follow-up
# work `all` deliberately leaves to the operator, the bump kind a module's
# own changelog would have argued for, the tag-ruleset call preflight
# reports but never makes. It never appears instead of a step line.
step_note() { printf 'release: NOTE %s\n' "$*"; }

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

# changelog_has_section <file> <version> -- true if a line starts with the
# literal "## [<version>]". Literal, never a regex: a version carrying a
# metacharacter (".*") would otherwise match "## [Unreleased]" and make
# the changelog cut SKIP itself (A3). require_semver rejects such versions
# at the entry points too; this is the second lock on the same door.
changelog_has_section() {
  awk -v prefix="## [$2]" '
    index($0, prefix) == 1 { found = 1; exit }
    END { exit found ? 0 : 1 }
  ' "$1" 2>/dev/null
}

# require_semver <version> -- the version argument reaches `git tag -a`,
# `git push origin <tag>` and `go get ...@v<version>`, and a bad tag on the
# public remote can never be deleted (D8). Mirror the regex
# .github/workflows/release.yml enforces on the tag so the two cannot
# drift.
require_semver() {
  if ! [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    step_fail "version-guard" "strict semver X.Y.Z" "$1"
  fi
}

# commit_with_subject_exists <subject> -- D3 says "release commit already
# on main (subject match in `git log -1`) -> SKIP", but a plain `git log
# -1` only sees HEAD, which is wrong inside a single `all` run: by the
# time bump/all-ship check whether an EARLIER step's commit already
# happened, later commits have moved HEAD past it. Search all of main's
# history for the exact subject instead -- safe here because
# scripts/branch-protection.sh enforces required_linear_history, so a
# commit's subject, once on main, never moves or gets rewritten.
#
# NOTE: do NOT pipe `git log` into `grep -q`. `set -o pipefail` is in
# force, `grep -q` exits on its first match, and the still-streaming
# `git log` dies with SIGPIPE -- so the pipeline reports 141 and the
# function answers "not found" for every commit that is not the repo's
# oldest. That made resume silently dead on any real-sized history (R1).
# Let git do the filtering and match the captured output with no pipe.
commit_with_subject_exists() {
  local subject="$1" found
  found=$(git -C "$repo_root" log main --format=%s --fixed-strings --grep="$subject" 2>/dev/null) || return 1
  grep -qxF -- "$subject" <<<"$found"
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
  # "no tags yet" is a normal answer, not an error: without this the
  # trailing AND-list's status would be the function's, and every direct
  # `x=$(latest_tag_version ...)` would kill the script under set -e with
  # no `release:` line printed (the one D2 hole -- see also F15).
  return 0
}

# changelog_cut_section <file> <version> <date> -- renames "## [Unreleased]"
# to "## [<version>] - <date>" in place, keeping its body, and inserts a
# fresh empty "## [Unreleased]" above it. The cut section reads
#   ## [X.Y.Z] - <date>
#   <blank>
#   ### Added
# like every hand-written section in all four changelogs (R5: the old
# skip_blank ate the separator and left the heading glued to the body).
changelog_cut_section() {
  local file="$1" version="$2" date="$3" tmp
  tmp=$(mktemp)
  track_tmp "$tmp"
  awk -v ver="$version" -v date="$date" '
    /^## \[Unreleased\]/ && !done {
      print
      print ""
      print "## [" ver "] - " date
      print ""
      done = 1
      eat_blank = 1
      next
    }
    eat_blank { eat_blank = 0; if ($0 == "") next }
    { print }
  ' "$file" >"$tmp" || step_fail "changelog-cut" "the changelog rewrite to succeed" "awk failed on $file"
  mv "$tmp" "$file"
}

# changelog_add_bullet <file> <heading> <text> -- adds "- <text>" under
# "### <heading>" inside the "## [Unreleased]" section, creating the
# heading if it is not already present.
#
# One awk pass, and it re-emits the section as exactly "## [Unreleased]",
# ONE blank line, then the body, however many times it is called. The old
# three-pass form (head awk + section + tail awk) re-added the section's
# own leading blank on top of a hard-coded one, so the root changelog --
# whose [Unreleased] is never cut -- gained a blank line on every release
# and had four after one and seven after two (R3).
changelog_add_bullet() {
  local file="$1" heading="$2" text="$3" tmp
  tmp=$(mktemp)
  track_tmp "$tmp"
  awk -v h="### $heading" -v b="- $text" '
    function emit_section(   i, start, end, found, inserted) {
      start = 0
      end = nbody - 1
      while (start <= end && body[start] == "") start++
      while (end >= start && body[end] == "") end--
      found = 0
      for (i = start; i <= end; i++) {
        if (body[i] == h) { found = 1; break }
      }
      print ""
      if (found) {
        inserted = 0
        for (i = start; i <= end; i++) {
          print body[i]
          if (!inserted && body[i] == h) { print b; inserted = 1 }
        }
      } else {
        print h
        print b
        if (end >= start) {
          print ""
          for (i = start; i <= end; i++) print body[i]
        }
      }
    }
    state == 0 {
      print
      if ($0 ~ /^## \[Unreleased\]/) state = 1
      next
    }
    state == 1 && /^## \[/ {
      emit_section()
      print ""
      print
      state = 2
      next
    }
    state == 1 { body[nbody++] = $0; next }
    { print }
    END { if (state == 1) emit_section() }
  ' "$file" >"$tmp" || step_fail "changelog-bullet" "the changelog rewrite to succeed" "awk failed on $file"
  mv "$tmp" "$file"
}

# --- module metadata -------------------------------------------------------

# module_kind <module> -- echoes lib|binary, or returns 1 SILENTLY: every
# caller turns that into a `release: FAIL` line, and a second stderr line
# of its own broke the one-line-per-step output contract (R15/D2).
module_kind() {
  case "$1" in
  freshbooks) echo lib ;;
  mcp | cli) echo binary ;;
  *) return 1 ;;
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

# watch_run <step> <workflow> <branch> [expect-sha] -- finds the run of
# <workflow> filtered by <branch>, then polls its conclusion every
# POLL_INTERVAL seconds. Always executes for real: watching is a read,
# never a write, so it is not gated by DRY_RUN.
#
# Discovery is itself a poll (R2/A7): GitHub takes seconds to register a
# workflow run after a push, so a one-shot `gh run list` right after the
# push returns the PREVIOUS commit's run (CI) or nothing at all (Release),
# and the old code turned that race into a hard FAIL on every real run.
# Retry the listing every DISCOVERY_INTERVAL seconds until a run whose
# headSha is $expect_sha appears (CI), or any run for the branch/tag
# appears (Release, which has no sha to pin -- see A8). Both waits share
# the one TIMEOUT budget.
watch_run() {
  local step="$1" workflow="$2" branch="$3" expect_sha="${4:-}"
  local listed id="" sha="" waited=0 view status conclusion

  while :; do
    listed=$(read_cmd gh run list --workflow "$workflow" --branch "$branch" --limit 1 \
      --json databaseId,headSha --jq '.[0] // empty | "\(.databaseId) \(.headSha)"') || listed=""
    if [ -n "$listed" ]; then
      read -r id sha <<<"$listed"
      if [ -z "$expect_sha" ] || [ "$sha" = "$expect_sha" ]; then
        break
      fi
    fi
    if [ "$waited" -ge "$TIMEOUT" ]; then
      if [ -n "$expect_sha" ]; then
        step_fail "$step" "a $workflow run for $expect_sha within ${TIMEOUT}s" "newest run is for ${sha:-no run at all}"
      fi
      step_fail "$step" "a $workflow run on $branch within ${TIMEOUT}s" "no run found"
    fi
    sleep "$DISCOVERY_INTERVAL"
    waited=$((waited + DISCOVERY_INTERVAL))
  done

  while :; do
    view=$(gh_jq '"\(.status) \(.conclusion // "")"' -- run view "$id" --json status,conclusion) ||
      step_fail "$step" "gh run view $id to succeed" "gh run view failed"
    read -r status conclusion <<<"$view"
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
    sleep "$POLL_INTERVAL"
    waited=$((waited + POLL_INTERVAL))
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

# require_clean_tree <step> -- A5: `cut` and `bump` are standalone entry
# points to a public tag push, and only `all` used to reach preflight's
# clean-tree check. Same rule, same strictness (a dirty tree fails under
# --dry-run too, exactly as preflight does).
require_clean_tree() {
  local step="$1" dirty
  dirty=$(git -C "$repo_root" status --porcelain)
  if [ -n "$dirty" ]; then
    step_fail "$step" "clean working tree" "dirty tree"
  fi
  step_ok "$step"
}

# --- preflight --------------------------------------------------------------

cmd_preflight() {
  local branch dirty auth_out sha ci_json ci_sha ci_status ci_conclusion

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

  # R4: the plan's precondition is "CI green FOR HEAD". Comparing only the
  # newest run's conclusion passes whenever main's last run was green --
  # including when local main is ahead of what CI has seen, which is the
  # one check standing between an unbuilt commit and a tag push.
  sha=$(git -C "$repo_root" rev-parse HEAD)
  ci_json=$(gh_jq '.[0] // empty | "\(.headSha) \(.status) \(.conclusion // "none")"' \
    -- run list --workflow CI --branch main --limit 1 --json headSha,status,conclusion) || ci_json=""
  if [ -z "$ci_json" ]; then
    step_fail "preflight-ci-green" "a completed CI run for HEAD $sha" "no CI run on main"
  fi
  read -r ci_sha ci_status ci_conclusion <<<"$ci_json"
  if [ "$ci_status" != "completed" ] || [ "$ci_conclusion" != "success" ]; then
    step_fail "preflight-ci-green" "status=completed conclusion=success" "status=$ci_status conclusion=$ci_conclusion"
  fi
  if [ "$ci_sha" = "$sha" ]; then
    step_ok "preflight-ci-green"
  elif [ "$DRY_RUN" = true ] && [ "$branch" != "main" ]; then
    # Same exemption preflight-branch takes above: off main under
    # --dry-run, HEAD is by definition not what CI built on main, so the
    # pin cannot hold and failing on it would make `all --dry-run`
    # unusable as a preview from a feature branch. main's newest run was
    # still required to be green.
    step_skip "preflight-ci-green" "HEAD is $branch, not main -- main's newest run ($ci_sha) is green"
  else
    step_fail "preflight-ci-green" "the newest CI run to be for HEAD $sha" "newest run is for $ci_sha"
  fi

  if read_cmd mise which go >/dev/null; then
    step_ok "preflight-mise-install"
  else
    step_fail "preflight-mise-install" "mise.toml toolchain resolvable" "mise which go failed -- run mise install"
  fi

  # D8: never edits branch protection or rulesets -- report the gh api call
  # that would apply/inspect the tag ruleset, unconditionally, and stop.
  # NOTE, not a "dry-run:" echo: this line prints on every preflight, and
  # the dry-run prefix read as if the command were pending outside
  # --dry-run (R15).
  step_note "gh api repos/InfiniteRoomLabs/freshbooks-tools/rulesets (tag ruleset for refs/tags/{freshbooks,mcp,cli}/v*, warn-only, never applied by this script)"
  step_ok "preflight-tag-ruleset-warn"
}

# --- docs (D5) --------------------------------------------------------------

# RELEASE_README_OUT redirects the rendered README somewhere other than
# README.md. scripts/check.sh's readme-drift-check uses it so a
# verification step never mutates a tracked file (A2/R8): it renders to a
# temp copy and diffs, instead of rewriting the tree and reading git diff
# -- which silently reverted an operator's uncommitted Status edit and
# left a modified README behind whenever the check failed.
cmd_docs() {
  local module tag tmp readme out
  readme="$repo_root/README.md"
  out="${RELEASE_README_OUT:-$readme}"
  if [ "$out" != "$readme" ] && [ "$DRY_RUN" != true ]; then
    cp "$readme" "$out" || step_fail "docs" "README.md to be copyable to $out" "cp failed"
  fi
  for module in freshbooks mcp cli; do
    tag=$(git -C "$repo_root" tag --list "$module/v*" | sort -V | tail -1) ||
      step_fail "docs" "git tag --list to succeed" "failed for $module"
    [ -z "$tag" ] && continue
    if [ "$DRY_RUN" = true ]; then
      dry_echo "rewrite README.md Status cell for $module -> $tag"
      continue
    fi
    tmp=$(mktemp)
    track_tmp "$tmp"
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
    ' "$out" >"$tmp" || step_fail "docs" "the README Status rewrite to succeed" "awk failed for $module"
    mv "$tmp" "$out"
  done
  step_ok "docs"
}

# --- verify (D1 verify subcommand; also called by cut/all -- D3: never skips) ---

do_verify() {
  local tag="$1" module version kind json assets isdraft name
  module="${tag%%/*}"
  version="${tag#*/v}"
  kind=$(module_kind "$module") || step_fail "verify-tag" "a tag shaped <module>/vX.Y.Z" "$tag"

  # gh embeds its own jq (--jq); shelling out to an external `jq` binary
  # was an undeclared runtime dependency whose absence produced a
  # misleading "unparsable" failure (R12/A9/S5). `name` is compared
  # against $tag, which never contains whitespace, so read -r splits it.
  json=$(gh_jq '"\(.isDraft) \(.name) \(.assets | length)"' -- release view "$tag" --json isDraft,name,assets) ||
    step_fail "verify-release" "a published release for $tag" "gh release view failed"
  read -r isdraft name assets <<<"$json"

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
  ensure_go_bin
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
  ensure_go_bin
  name=$(binary_name "$module")
  workdir=$(mktemp -d)
  track_tmp "$workdir"

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
  track_tmp "$gobin_dir"
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

  # Both are registered with the EXIT trap above, so the eight step_fail
  # paths between their mktemp and here no longer leak them (R13/A4);
  # removing them now just keeps /tmp small during a long `all` run.
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

# --- shared release-step blocks (S1-S3) --------------------------------------

# commit_and_push <step-prefix> <subject> <path>... -- emits
# <prefix>-commit and <prefix>-push, or SKIPs both when the subject is
# already on main (D3). Staging and committing stay separate run_cmd
# calls: the agent-ops guards inspect the index between them.
commit_and_push() {
  local prefix="$1" subject="$2"
  shift 2
  if commit_with_subject_exists "$subject"; then
    step_skip "$prefix-commit" "a commit \"$subject\" is already on main"
    return 0
  fi
  confirm_first_push
  run_cmd git -C "$repo_root" add "$@" ||
    step_fail "$prefix-commit" "git add to succeed" "failed"
  run_cmd git -C "$repo_root" commit -m "$subject" ||
    step_fail "$prefix-commit" "git commit to succeed" "failed"
  step_ok "$prefix-commit"

  confirm_first_push
  run_cmd git -C "$repo_root" push origin main ||
    step_fail "$prefix-push" "git push origin main to succeed" "failed"
  step_ok "$prefix-push"
}

# watch_head_ci <step> <what> -- watches the CI run for HEAD, or echoes the
# plan and SKIPs under --dry-run, where the commit was never made.
watch_head_ci() {
  if [ "$DRY_RUN" = true ]; then
    dry_echo "watch CI on main for the $2 commit"
    step_skip "$1" "dry-run"
    return 0
  fi
  local head_sha
  head_sha=$(git -C "$repo_root" rev-parse HEAD)
  watch_run "$1" "CI" "main" "$head_sha"
}

# tag_push_and_watch <module> <version> <tag> -- emits cut-tag,
# cut-tag-push and cut-release-watch. Shared verbatim by cut_lib and
# cut_binary, which is all cut_binary used to be.
tag_push_and_watch() {
  local module="$1" version="$2" tag="$3"

  if git -C "$repo_root" ls-remote --tags origin "refs/tags/$tag" 2>>"$LOG_FILE" | grep -q "$tag"; then
    step_skip "cut-tag-push" "$tag already on origin"
  else
    confirm_first_push
    # R7: an aborted earlier run can leave a local tag pointing at a commit
    # that has since been superseded. Pushing it publishes the WRONG commit
    # under this version, permanently -- D8 forbids ever deleting a tag. So
    # assert the existing local tag is HEAD and refuse otherwise; the
    # operator decides what to do with it.
    if git -C "$repo_root" rev-parse "$tag" >/dev/null 2>&1; then
      local tag_sha head_sha
      tag_sha=$(git -C "$repo_root" rev-parse "$tag^{commit}")
      head_sha=$(git -C "$repo_root" rev-parse HEAD)
      if [ "$tag_sha" != "$head_sha" ]; then
        step_fail "cut-tag" "local tag $tag to point at HEAD $head_sha" "it points at $tag_sha"
      fi
      step_skip "cut-tag" "$tag already exists locally at HEAD"
    else
      run_cmd git -C "$repo_root" tag -a "$tag" -m "$module $version" ||
        step_fail "cut-tag" "git tag -a $tag to succeed" "failed"
      step_ok "cut-tag"
    fi
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

# --- cut <module> <version> -------------------------------------------------

cmd_cut() {
  local module="${args[0]:-}" version="${args[1]:-}" kind
  if [ -z "$module" ] || [ -z "$version" ]; then
    step_fail "cut" "cut <module> <version>" "missing argument(s)"
  fi
  kind=$(module_kind "$module") || step_fail "cut" "module in {freshbooks,mcp,cli}" "$module"
  require_main_unless_dry_run
  require_clean_tree "cut-clean-tree"

  if [ "$version" = "auto" ]; then
    accept_proposed_version "$module"
    version="$ACCEPTED_VERSION"
  fi
  require_semver "$version"

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
  # D2 reserves the " -- <detail>" suffix for SKIP and FAIL; the proposal
  # detail goes on its own NOTE line so the OK shape stays uniform (R15).
  step_ok "version-propose"
  step_note "$module $current -> $next ($kind)"
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
      dry_echo "changelog_add_bullet $repo_root/CHANGELOG.md Added \"\`$module\` cut to $version, ahead of the \`$tag\` tag.\""
    else
      changelog_cut_section "$changelog" "$version" "$today"
      changelog_add_bullet "$repo_root/CHANGELOG.md" "Added" "\`$module\` cut to $version, ahead of the \`$tag\` tag."
    fi
    step_ok "cut-changelog"
  fi

  commit_and_push "cut" "release($module): v$version" "$module/CHANGELOG.md" CHANGELOG.md
  watch_head_ci "cut-ci-watch" "release"
  tag_push_and_watch "$module" "$version" "$tag"
}

# cut_binary implements the binary release flow (plan steps 12-16, minus
# the bump commit, which is the `bump` subcommand's job): CI watch, tag,
# tag push, Release watch. do_verify (called by the caller) covers steps
# 14-17.
cut_binary() {
  # A1: cut_lib gated its tag push behind the CI run for the exact pushed
  # sha; cut_binary did not, so `cut mcp <v>` could publish a permanent
  # public tag off a red main -- and D8 forbids ever deleting one. Inside
  # `all` this is a cheap no-op (do_bump's watch just went green); it
  # closes the standalone path.
  watch_head_ci "cut-ci-watch" "bump"
  tag_push_and_watch "$1" "$2" "$3"
}

# --- bump <lib-version> [--binary-version A.B.C] ----------------------------

cmd_bump() {
  local lib_version="${args[0]:-}" binary_version="$BINARY_VERSION"
  [ -z "$lib_version" ] && step_fail "bump" "bump <lib-version>" "missing argument"
  require_main_unless_dry_run
  require_clean_tree "bump-clean-tree"
  require_semver "$lib_version"
  if [ -z "$binary_version" ]; then
    binary_version=$(propose_binary_version)
    announce_binary_version "$binary_version"
  fi
  require_semver "$binary_version"
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

# announce_binary_version <proposed> -- prints the bump-version-propose
# step, then, for information only, what each module's own [Unreleased]
# section would have argued for. The patch default stays (mcp and cli
# release in lockstep, and 0.1.1 and 0.1.2 were both patches -- 0.1.2 even
# shipped a new tool), but an additive changelog now says so out loud
# instead of being silently under-bumped (R11).
announce_binary_version() {
  local proposed="$1" current module kind
  current=$(latest_tag_version "mcp")
  [ -z "$current" ] && current=$(latest_tag_version "cli")
  [ -z "$current" ] && current="0.0.0"
  step_ok "bump-version-propose"
  step_note "mcp/cli $current -> $proposed (patch, the default)"
  for module in mcp cli; do
    kind=$(derive_bump_kind "$repo_root/$module/CHANGELOG.md" "$current")
    if [ "$kind" != "patch" ] && [ "$kind" != "none" ]; then
      step_note "$module [Unreleased] argues for $kind -- pass --binary-version A.B.C to override the patch default"
    fi
  done
}

# do_bump implements plan steps 9-11: go get + go mod tidy in mcp/ and
# cli/, a "Requires freshbooks vX.Y.Z" Changed line plus the changelog
# cut for both modules (sharing one version), the root changelog line, the
# per-module fmt-check/vet/lint/test/cover gate (never the dirty-tree
# banner -- the staged bump is expected to be dirty until committed), one
# shared commit, push, CI watch.
do_bump() {
  local lib_version="$1" binary_version="$2" module today
  [ "$DRY_RUN" = true ] || ensure_go_bin
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
      dry_echo "changelog_add_bullet $repo_root/CHANGELOG.md Added \"\`$module\` cut to $binary_version, ahead of the \`$module/v$binary_version\` tag.\""
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

  commit_and_push "bump" \
    "release(mcp,cli): require freshbooks v$lib_version and cut $binary_version" \
    mcp/go.mod mcp/go.sum mcp/CHANGELOG.md cli/go.mod cli/go.sum cli/CHANGELOG.md CHANGELOG.md
  watch_head_ci "bump-ci-watch" "bump"
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
  require_semver "$lib_version"

  cut_lib "freshbooks" "$lib_version" "freshbooks/v$lib_version"
  verify_after_cut "freshbooks/v$lib_version"

  if [ -z "$binary_version" ]; then
    binary_version=$(propose_binary_version)
    announce_binary_version "$binary_version"
  fi
  require_semver "$binary_version"
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
  # correct on every run, resumed or not. R10: say so in the OUTPUT, not
  # only in this comment -- every precedent ship commit also retargeted
  # GOAL.md, and the operator had no signal that it was still owed.
  step_note "all-ship stages README.md only -- write the docs/progress.md ledger row and retarget GOAL.md by hand, then amend or follow up"

  commit_and_push "all-ship" "docs: ship v$lib_version" README.md
  watch_head_ci "all-ship-ci-watch" "ship"
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
