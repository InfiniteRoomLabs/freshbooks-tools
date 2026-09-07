#!/usr/bin/env -S usage bash
# Regression test for scripts/freshbooks-account-backup.sh. Runs it against
# a fake `freshbooks` binary (via FRESHBOOKS_BIN) that answers from canned
# JSON: one --all collection, one page-walked collection (two pages), one
# unpaginated array, one collection that fails, the three singletons, and
# invoices pdf. No network, no real CLI. Takes no arguments; wired into
# `mise run check` beside the other selftests.
#USAGE bin "backup-selftest.sh"
#USAGE about "Regression test for scripts/freshbooks-account-backup.sh; takes no arguments"

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
failures=0
pass() { printf 'backup-selftest: PASS %s\n' "$1"; }
fail() { printf 'backup-selftest: FAIL %s -- %s\n' "$1" "$2" >&2; failures=$((failures + 1)); }

cat >"$scratch/freshbooks" <<'FAKE'
#!/usr/bin/env bash
# Fake freshbooks CLI for scripts/backup-selftest.sh.
args="$*"
case "$args" in
  version) echo "fake-9.9.9" ;;
  "invoices list --help") echo "      --all   walk every page" ;;
  *" list --help") echo "      --page int" ;;
  "invoices list --all -o json") echo '[{"id":11,"invoice_number":"A1"},{"id":12,"invoice_number":"A2"}]' ;;
  "retainers list -o json") echo '{"items":[{"id":1}],"page":1,"pages":2,"per_page":1,"total":2}' ;;
  "retainers list --page 2 -o json") echo '{"items":[{"id":2}],"page":2,"pages":2,"per_page":1,"total":2}' ;;
  "callbacks list -o json") echo '{"error":{"message":"boom"}}' >&2; exit 1 ;;
  *" list -o json") echo '[{"id":1}]' ;;
  "identity me -o json") echo '[{"account_id":"AAA111","role":"owner"}]' ;;
  "gateways get -o json") echo '[]' ;;
  "systems get -o json") echo '{"id":1}' ;;
  "invoices pdf 11 -o "*) printf '%%PDF-fake' >"${args##* }" ;;
  "invoices pdf 12 -o "*) echo "pdf boom" >&2; exit 1 ;;
  *) echo "fake freshbooks: unhandled: $args" >&2; exit 2 ;;
esac
FAKE
chmod +x "$scratch/freshbooks"

run_backup() {
  FRESHBOOKS_BIN="$scratch/freshbooks" "$repo_root/scripts/freshbooks-account-backup.sh" "$@" 2>"$scratch/stderr"
}

# --- full run: one failed collection and one failed pdf -> exit 1 -----------
status=0
run_backup --out "$scratch/out" || status=$?
dir="$(find "$scratch/out" -mindepth 1 -maxdepth 1 -type d | head -1)"
[ "$status" -eq 1 ] && pass "exit 1 when a resource fails" || fail "exit status" "got $status"
[ "$(jq length "$dir/invoices.json")" = 2 ] && pass "--all collection written" || fail "invoices.json" "$(cat "$dir/invoices.json" 2>&1)"
[ "$(jq -c 'map(.id)' "$dir/retainers.json")" = "[1,2]" ] && pass "page-walked collection merged" || fail "retainers.json" "$(cat "$dir/retainers.json" 2>&1)"
[ "$(jq length "$dir/staff.json")" = 1 ] && pass "unpaginated array kept" || fail "staff.json" "$(cat "$dir/staff.json" 2>&1)"
[ ! -e "$dir/callbacks.json" ] && pass "failed collection leaves no file" || fail "callbacks.json" "exists"
grep -q boom "$dir/backup.log" && pass "stderr captured in backup.log" || fail "backup.log" "no error text"
[ -s "$dir/identity.json" ] && [ -s "$dir/gateways.json" ] && [ -s "$dir/systems.json" ] && pass "singletons written" || fail "singletons" "missing"
[ -s "$dir/pdfs/invoices/11.pdf" ] && [ ! -e "$dir/pdfs/invoices/12.pdf" ] && pass "invoice pdfs: one written, one failed" || fail "pdfs" "$(ls -R "$dir/pdfs" 2>&1)"
m="$dir/manifest.json"
[ "$(jq -r '.ok' "$m")" = false ] && [ "$(jq -r '.resources.callbacks.status' "$m")" = failed ] \
  && [ "$(jq -r '.resources.retainers.count' "$m")" = 2 ] && [ "$(jq -r '.invoice_pdfs.written' "$m")" = 1 ] \
  && [ "$(jq -r '.cli_version' "$m")" = "fake-9.9.9" ] \
  && pass "manifest records outcomes" || fail "manifest" "$(cat "$m" 2>&1)"
[ "$(stat -c %a "$dir")" = 700 ] && pass "backup dir is 0700" || fail "mode" "$(stat -c %a "$dir")"

# --- --no-pdfs skips the pdf pass entirely --------------------------------
run_backup --out "$scratch/out2" --no-pdfs || true
dir2="$(find "$scratch/out2" -mindepth 1 -maxdepth 1 -type d | head -1)"
[ ! -e "$dir2/pdfs" ] && [ "$(jq -r '.invoice_pdfs.skipped' "$dir2/manifest.json")" = true ] \
  && pass "--no-pdfs skips pdfs" || fail "--no-pdfs" "$(ls "$dir2")"

if [ "$failures" -ne 0 ]; then
  printf 'backup-selftest: %d failure(s)\n' "$failures" >&2
  exit 1
fi
echo "backup-selftest: all probes passed"
