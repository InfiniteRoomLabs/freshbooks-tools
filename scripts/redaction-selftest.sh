#!/usr/bin/env -S usage bash
# Regression test for scripts/redaction-check.sh (Phase 8 security A1: the
# --range mode's term scan silently scanned nothing for a whole phase, and
# nothing would have caught it). Every probe runs in a throwaway git repo
# under mktemp -d and is asserted in BOTH modes -- staged index and
# --range -- so the two can never disagree again.
#
# It is safe without the private term list: the term-scan probes run
# against a stub resolver (a fake $HOME plus a `uv` shim on PATH) whose
# terms are nonsense strings, so the control is armed everywhere including
# CI. When the real resolver IS present, two extra probes plant one short
# and one long term drawn from it; their values are never printed.
#
# Takes no arguments. Wired into `mise run check` once per gate.
#USAGE bin "redaction-selftest.sh"
#USAGE about "Regression test for scripts/redaction-check.sh; takes no arguments"

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
resolver="$HOME/projects/infinite-room-labs/agent-ops/scripts/resolve-redaction-terms.py"

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

# Stub environment: a fake HOME carrying a resolver at the path
# redaction-check.sh looks for, and a `uv` shim that prints the stub term
# list instead of running it. Both nonsense strings, one below the 8-char
# short-term threshold (word-boundary path) and one above it (fixed-string
# substring path).
stub_short="zqxjvb"
stub_long="zzredactionselftestneedle"
stub_home="$scratch/home"
stub_bin="$scratch/bin"
mkdir -p "$stub_home/projects/infinite-room-labs/agent-ops/scripts" "$stub_bin"
: >"$stub_home/projects/infinite-room-labs/agent-ops/scripts/resolve-redaction-terms.py"
cat >"$stub_bin/uv" <<EOF
#!/bin/sh
printf '%s\n%s\n' "$stub_short" "$stub_long"
EOF
chmod +x "$stub_bin/uv"

# `usage` is this script's interpreter and is normally reached through a
# mise shim, which resolves the tool out of the real $HOME -- so under the
# stub $HOME the shim errors out. Put the resolved binary itself on the
# stub PATH instead.
stub_usage="$(mise which usage 2>/dev/null || command -v usage)"
ln -s "$stub_usage" "$stub_bin/usage"

# Preflight: the stub `uv` must be the binary redaction-check.sh actually
# resolves, or every stub probe would silently take the "no redaction
# terms configured" no-op path. BASH_ENV is dropped for exactly this
# reason -- it re-activates mise's shims and re-prepends them to PATH.
resolved_uv=$(env -u BASH_ENV HOME="$stub_home" PATH="$stub_bin:$PATH" bash -c 'command -v uv')
if [ "$resolved_uv" != "$stub_bin/uv" ]; then
  echo "redaction-selftest: the stub uv shim is shadowed by $resolved_uv" >&2
  exit 1
fi

mkdir -p "$scratch/nohooks"

case_n=0
failures=0

pass_msg() { printf 'redaction-selftest: PASS %s\n' "$1"; }
fail_msg() {
  printf 'redaction-selftest: FAIL %s -- %s\n' "$1" "$2" >&2
  failures=$((failures + 1))
}

# new_repo makes a throwaway repo with a `base` commit and a copy of the
# script under test, and echoes its path.
new_repo() {
  local dir="$scratch/case$1"
  mkdir -p "$dir"
  git -C "$dir" init -q -b base
  git -C "$dir" config user.email "selftest@example.com"
  git -C "$dir" config user.name "redaction selftest"
  git -C "$dir" config commit.gpgsign false
  git -C "$dir" config core.hooksPath "$scratch/nohooks"
  git -C "$dir" commit -q --allow-empty -m base
  # Tag the baseline: the probe commit below advances the `base` branch
  # itself, so `base..HEAD` would be empty.
  git -C "$dir" tag baseline
  cp "$repo_root/scripts/redaction-check.sh" "$dir/redaction-check.sh"
  chmod +x "$dir/redaction-check.sh"
  printf '%s' "$dir"
}

# probe <name> <relpath> <payload> <want-exit> <needle> <stub|real>
#
# Plants payload on line 3 of relpath, then asserts the wanted exit status
# and that the needle appears in the output, in staged mode and in range
# mode. The payload is never echoed.
probe() {
  local name="$1" rel="$2" payload="$3" want="$4" needle="$5" env_kind="$6"
  local dir mode out status ok=1
  case_n=$((case_n + 1))
  dir="$(new_repo "$case_n")"
  mkdir -p "$dir/$(dirname "$rel")"
  {
    printf '{\n'
    printf '  "unrelated": "value",\n'
    printf '  "planted": "%s"\n' "$payload"
    printf '}\n'
  } >"$dir/$rel"
  git -C "$dir" add "$rel"
  for mode in staged range; do
    local -a args=()
    if [ "$mode" = range ]; then
      git -C "$dir" commit -q -m probe
      args=(--range baseline..HEAD)
    fi
    set +e
    if [ "$env_kind" = stub ]; then
      out=$(cd "$dir" && env -u BASH_ENV HOME="$stub_home" PATH="$stub_bin:$PATH" ./redaction-check.sh "${args[@]}" 2>&1)
    else
      out=$(cd "$dir" && ./redaction-check.sh "${args[@]}" 2>&1)
    fi
    status=$?
    set -e
    if [ "$status" -ne "$want" ]; then
      fail_msg "$name [$mode]" "exit $status, want $want"
      ok=0
    elif ! printf '%s' "$out" | grep -qF -- "$needle"; then
      fail_msg "$name [$mode]" "output does not contain the expected marker"
      ok=0
    fi
  done
  [ "$ok" -eq 1 ] && pass_msg "$name"
  return 0
}

# Preflight: the script itself must run to completion under the stub
# environment, so a broken stub can never be mistaken for a passing probe.
smoke_dir="$(new_repo smoke)"
if ! smoke_out=$(cd "$smoke_dir" && env -u BASH_ENV HOME="$stub_home" PATH="$stub_bin:$PATH" ./redaction-check.sh 2>&1); then
  echo "redaction-selftest: the stub environment cannot run redaction-check.sh: $smoke_out" >&2
  exit 1
fi
if ! printf '%s' "$smoke_out" | grep -qF -- "redaction-check: clean"; then
  echo "redaction-selftest: stub smoke run did not report clean: $smoke_out" >&2
  exit 1
fi

seed_json="freshbooks/testdata/seed/x.json"

# -- the digit sweep ------------------------------------------------------
probe "7-digit integer fails, naming file:line" \
  "$seed_json" "1825574" 1 \
  "redaction-check: unallowlisted 6+-digit number 1825574 in $seed_json:3" stub

# The sweep covers every fixture, not just the raw captures (Phase 8
# security A5 / code review R5): D2 re-seeded these paths from seed/.
probe "the sweep reaches re-seeded fixtures" \
  "freshbooks/testdata/accounting/expenses_list.json" "1825574" 1 \
  "redaction-check: unallowlisted 6+-digit number 1825574 in freshbooks/testdata/accounting/expenses_list.json:3" stub

# -- UUID handling (Phase 8 security A3) ----------------------------------
probe "a real id wearing a synthetic uuid tail fails" \
  "$seed_json" "12345678-0000-4000-8000-000000000001" 1 \
  "redaction-check: unallowlisted 6+-digit number 12345678 in $seed_json:3" stub

probe "an entirely decimal uuid-shaped token fails" \
  "$seed_json" "18255740-1234-5678-9012-123456789012" 1 \
  "redaction-check: unallowlisted 6+-digit number 18255740 in $seed_json:3" stub

probe "the synthetic uuid convention passes" \
  "$seed_json" "00000000-0000-4000-8000-000000000123" 0 \
  "redaction-check: clean" stub

probe "a genuine hex uuid passes" \
  "$seed_json" "9f8e7d6c-1a2b-4c3d-8e9f-0a1b2c3d4e5f" 0 \
  "redaction-check: clean" stub

# -- the allowlist --------------------------------------------------------
probe "a 700NN synthetic id passes (below the sweep threshold)" \
  "$seed_json" "70023" 0 "redaction-check: clean" stub

probe "an allowlisted filler number passes" \
  "$seed_json" "8675309" 0 "redaction-check: clean" stub

probe "an all-zero run passes" \
  "$seed_json" "000000000" 0 "redaction-check: clean" stub

# -- the term scan (the control A1 found inert in --range mode) -----------
probe "a short stub term fails in both modes" \
  "notes.txt" "$stub_short" 1 "redaction-check: possible leak in notes.txt" stub

probe "a long stub term fails in both modes, mid-identifier" \
  "notes.txt" "id_${stub_long}_x" 1 "redaction-check: possible leak in notes.txt" stub

# -- an unusable range fails loudly (Phase 8 security A2) -----------------
case_n=$((case_n + 1))
bad_dir="$(new_repo "$case_n")"
set +e
bad_out=$(cd "$bad_dir" && env -u BASH_ENV HOME="$stub_home" PATH="$stub_bin:$PATH" ./redaction-check.sh --range no-such-ref..HEAD 2>&1)
bad_status=$?
set -e
if [ "$bad_status" -ne 2 ]; then
  fail_msg "an unusable range exits 2" "exit $bad_status, want 2"
elif ! printf '%s' "$bad_out" | grep -qF -- "redaction-check: unusable range"; then
  fail_msg "an unusable range exits 2" "output does not contain the expected marker"
else
  pass_msg "an unusable range exits 2"
fi

# -- the private term list, when this machine has it ----------------------
short_term=""
long_term=""
if [ -f "$resolver" ] && command -v uv >/dev/null 2>&1; then
  terms_raw=$(cd "$(dirname "$resolver")" && uv run "$(basename "$resolver")")
  while IFS= read -r term; do
    search="${term%%==>*}"
    [ -z "$search" ] && continue
    if [ "${#search}" -lt 8 ] && [ -z "$short_term" ]; then
      short_term="$search"
    fi
    if [ "${#search}" -ge 8 ] && [ -z "$long_term" ]; then
      long_term="$search"
    fi
  done <<<"$terms_raw"
fi

if [ -n "$short_term" ]; then
  probe "a short private term fails in both modes" \
    "notes.txt" "$short_term" 1 "redaction-check: possible leak in notes.txt" real
else
  echo "redaction-selftest: NOTICE no short private term available -- skipping that probe"
fi

if [ -n "$long_term" ]; then
  probe "a long private term fails in both modes" \
    "notes.txt" "$long_term" 1 "redaction-check: possible leak in notes.txt" real
else
  echo "redaction-selftest: NOTICE no long private term available -- skipping that probe"
fi

if [ "$failures" -ne 0 ]; then
  echo "redaction-selftest: $failures assertion(s) failed" >&2
  exit 1
fi

echo "redaction-selftest: OK"
