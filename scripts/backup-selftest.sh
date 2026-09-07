#!/usr/bin/env -S usage bash
# Regression test for scripts/freshbooks-account-backup.sh. Runs it against
# a fake `freshbooks` binary (via FRESHBOOKS_BIN) that answers from canned
# JSON shaped like the real CLI's: --all collections (a bare array, `null`
# when empty), a page-walked collection (three pages, and a variant whose
# middle page fails), a page-shaped command with no --page flag (retainers,
# the CLI's one NoPaging list), an unpaginated array, a collection that
# fails outright, the three singletons, and invoices pdf with one hostile
# id. No network, no real CLI. Takes no arguments; wired into
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
# Fake freshbooks CLI for scripts/backup-selftest.sh. Shapes mirror the
# real CLI: --all prints a bare array (null when empty); a paginated list
# without --all prints {items,page,pages,per_page,total}; retainers is
# page-shaped but registers neither --all nor --page; staff is a bare array.
args="$*"
case "$args" in
  version) echo "fake-9.9.9" ;;
  "invoices list --help" | "clients list --help" | "callbacks list --help" | "bills list --help" | "estimates list --help") echo "      --all   walk every page" ;;
  "retainers list --help" | "staff list --help") echo "      --search stringToString" ;;
  *" list --help") echo "      --page int" ;;
  "invoices list --all -o json") echo '[{"id":11,"invoice_number":"A1"},{"id":12,"invoice_number":"A2"},{"id":"../evil"}]' ;;
  "clients list --all -o json") echo 'null' ;;
  "bills list --all -o json") echo 'not json at all' ;;
  "estimates list --all -o json") ;;
  "callbacks list --all -o json") echo '{"error":{"message":"boom"}}' >&2; exit 1 ;;
  "services list -o json") echo '{"items":[{"id":1}],"page":1,"pages":3,"per_page":1,"total":3}' ;;
  "services list --page 2 -o json") echo '{"items":[{"id":2}],"page":2,"pages":3,"per_page":1,"total":3}' ;;
  "services list --page 3 -o json") echo '{"items":[{"id":3}],"page":3,"pages":3,"per_page":1,"total":3}' ;;
  "tasks list -o json") echo '{"items":[{"id":1}],"page":1,"pages":3,"per_page":1,"total":3}' ;;
  "tasks list --page 2 -o json") echo "page 2 boom" >&2; exit 1 ;;
  "tasks list --page 3 -o json") echo '{"items":[{"id":3}],"page":3,"pages":3,"per_page":1,"total":3}' ;;
  "retainers list -o json") echo '{"items":[{"id":1}],"page":1,"pages":2,"per_page":1,"total":2}' ;;
  "retainers list --page "*) echo "Error: unknown flag: --page" >&2; exit 2 ;;
  "staff list -o json") echo '[{"id":1}]' ;;
  *" list -o json") echo '{"items":[{"id":1}],"page":1,"pages":1,"per_page":15,"total":1}' ;;
  "identity me -o json") echo '[{"account_id":"AAA111","role":"owner"}]' ;;
  "gateways get -o json") echo '[]' ;;
  "systems get -o json") echo '{"id":1}' ;;
  "invoices pdf -o "*" -- 11") p="${args#invoices pdf -o }"; printf '%%PDF-fake' >"${p% -- 11}" ;;
  "invoices pdf -o "*" -- 12") echo "pdf boom" >&2; exit 1 ;;
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
[ "$(jq length "$dir/invoices.json")" = 3 ] && pass "--all collection written" || fail "invoices.json" "$(cat "$dir/invoices.json" 2>&1)"
[ "$(cat "$dir/clients.json" | jq -c .)" = "[]" ] && pass "empty --all collection (null) written as []" || fail "clients.json" "$(cat "$dir/clients.json" 2>&1)"
[ "$(jq -c 'map(.id)' "$dir/services.json")" = "[1,2,3]" ] && pass "three-page collection walked and merged" || fail "services.json" "$(cat "$dir/services.json" 2>&1)"
[ ! -e "$dir/tasks.json" ] && [ "$(jq -r '.resources.tasks.status' "$dir/manifest.json")" = failed ] && pass "a failed middle page fails the collection" || fail "tasks" "$(find "$dir" -name 'tasks*'; jq -c .resources.tasks "$dir/manifest.json")"
[ "$(jq -c 'map(.id)' "$dir/retainers.json")" = "[1]" ] && [ "$(jq -r '.resources.retainers.status' "$dir/manifest.json")" = partial ] && pass "page-shaped list without --page keeps page 1 as partial" || fail "retainers" "$(cat "$dir/retainers.json" 2>&1); $(jq -c .resources.retainers "$dir/manifest.json")"
[ "$(jq length "$dir/staff.json")" = 1 ] && pass "unpaginated array kept" || fail "staff.json" "$(cat "$dir/staff.json" 2>&1)"
[ ! -e "$dir/callbacks.json" ] && pass "failed collection leaves no file" || fail "callbacks.json" "exists"
[ ! -e "$dir/bills.json" ] && [ ! -e "$dir/estimates.json" ] && [ "$(jq -r '.resources.bills.status, .resources.estimates.status' "$dir/manifest.json" | sort -u)" = failed ] && pass "exit 0 with junk or empty stdout is a failed resource" || fail "junk stdout" "$(find "$dir" -name 'bills*' -o -name 'estimates*'; jq -c '.resources.bills, .resources.estimates' "$dir/manifest.json" 2>&1)"
[ "$(jq '.resources | length' "$dir/manifest.json")" = 27 ] && [ ! -e "$dir/manifest.json.tmp" ] && pass "manifest records all 27 resources" || fail "manifest" "$(jq '.resources | length' "$dir/manifest.json" 2>&1)"
grep -q boom "$dir/backup.log" && grep -q "page 2 boom" "$dir/backup.log" && pass "stderr captured in backup.log" || fail "backup.log" "no error text"
[ -s "$dir/identity.json" ] && [ -s "$dir/gateways.json" ] && [ -s "$dir/systems.json" ] && pass "singletons written" || fail "singletons" "missing"
[ -s "$dir/pdfs/invoices/11.pdf" ] && [ ! -e "$dir/pdfs/invoices/12.pdf" ] && [ "$(find "$dir/pdfs" -type f | wc -l)" = 1 ] && pass "invoice pdfs: one written, one failed, hostile id skipped" || fail "pdfs" "$(ls -R "$dir/pdfs" 2>&1)"
grep -q "not an integer" "$dir/backup.log" && pass "hostile invoice id logged" || fail "hostile id" "not logged"
m="$dir/manifest.json"
[ "$(jq -r '.ok' "$m")" = false ] && [ "$(jq -r '.resources.callbacks.status' "$m")" = failed ] \
  && [ "$(jq -r '.resources.services.count' "$m")" = 3 ] && [ "$(jq -r '.invoice_pdfs.written' "$m")" = 1 ] \
  && [ "$(jq -r '.invoice_pdfs.failed' "$m")" = 2 ] && [ "$(jq -r '.cli_version' "$m")" = "fake-9.9.9" ] \
  && pass "manifest records outcomes" || fail "manifest" "$(cat "$m" 2>&1)"
[ -n "$(find "$dir" -maxdepth 0 -perm 700)" ] && pass "backup dir is 0700" || fail "mode" "$(ls -ld "$dir")"

# --- a pre-existing directory at this run's name is refused, never reused --
# The run's name is the current second; pre-create it and the next two so
# the probe does not depend on finishing within one second.
pre="$scratch/pre"
for s in 0 1 2; do mkdir -p "$pre/$(date -u -d "+$s seconds" +%Y%m%dT%H%M%SZ 2>/dev/null || date -u +%Y%m%dT%H%M%SZ)"; done
status=0; run_backup --out "$pre" --no-pdfs || status=$?
[ "$status" -ne 0 ] && [ -z "$(find "$pre" -name manifest.json)" ] && pass "an existing directory at the run's name is refused" || fail "mkdir" "exit $status; $(find "$pre" -name manifest.json)"

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
