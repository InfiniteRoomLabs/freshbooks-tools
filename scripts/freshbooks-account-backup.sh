#!/usr/bin/env -S usage bash
# Full read-only backup of everything the freshbooks CLI can list for the
# current context: one JSON array per collection, the three singletons
# (identity me, gateways get, systems get), one PDF per invoice, and a
# manifest.json recording the CLI version and per-resource counts and
# outcomes. Never writes to FreshBooks. See docs/backup.md.
#
# Scope (account/business/business-uuid) and credentials come from the
# CLI's normal chain (flags are not forwarded; set a context or export the
# FRESHBOOKS_* variables), so the usual invocation is:
#
#   scripts/freshbooks-account-backup.sh --out ~/backups/freshbooks
#
# A resource that fails is logged to backup.log and skipped; the run keeps
# going and exits 1 at the end so a cron job notices. Output directories
# are 0700: a backup is the whole business's books.
#USAGE bin "freshbooks-account-backup.sh"
#USAGE about "Back up everything the freshbooks CLI can read for the current context"
#USAGE flag "-o --out <dir>" help="Backup root; a UTC-timestamped directory is created inside it" default="./freshbooks-backup"
#USAGE flag "--no-pdfs" help="Skip downloading one PDF per invoice"

set -euo pipefail

fb="${FRESHBOOKS_BIN:-freshbooks}"
root="${usage_out:-./freshbooks-backup}"
no_pdfs="${usage_no_pdfs:-false}"

# Every `<resource> list` the CLI has. time-entries list-with-totals is the
# same wire call as time-entries list and is deliberately not repeated.
collections=(
  bill-vendors bills callbacks clients credit-notes estimates
  expense-categories expenses invoice-profiles invoices items
  journal-entry-accounts ledger-accounts other-income payments projects
  retainers service-rates services staff tasks taxes team-members
  time-entries
)
# name|command -- the reads that return one document, not a list.
singletons=(
  "identity|identity me"
  "gateways|gateways get"
  "systems|systems get"
)

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
dir="$root/$stamp"
umask 077
mkdir -p "$dir"
log="$dir/backup.log"
failed=0
entries=()   # "name<TAB>count<TAB>status"

record() { entries+=("$1	$2	$3"); }
say() { printf 'backup: %s\n' "$*" | tee -a "$log" >&2; }

# Writes a JSON array of every item in a collection to $2. Uses --all where
# the command offers it; otherwise fetches page 1 and walks the remaining
# pages by hand (page-shaped output), or takes the array as is (unpaginated).
fetch_collection() {
  local res="$1" out="$2" first pages p
  if "$fb" "$res" list --help 2>/dev/null | grep -q -- '--all'; then
    "$fb" "$res" list --all -o json >"$out" 2>>"$log"
    return
  fi
  first="$("$fb" "$res" list -o json 2>>"$log")" || return 1
  if [ "$(jq -r type <<<"$first")" != object ]; then
    printf '%s\n' "$first" >"$out"
    return
  fi
  pages="$(jq -r '.pages // 0' <<<"$first")"
  {
    jq '.items // []' <<<"$first"
    for ((p = 2; p <= pages; p++)); do
      "$fb" "$res" list --page "$p" -o json 2>>"$log" | jq '.items // []'
    done
  } | jq -s 'add // []' >"$out"
}

for res in "${collections[@]}"; do
  out="$dir/$res.json"
  if fetch_collection "$res" "$out"; then
    record "$res" "$(jq 'length' "$out")" ok
    say "$res: $(jq 'length' "$out") items"
  else
    rm -f "$out"
    record "$res" 0 failed
    failed=1
    say "$res: FAILED (see backup.log)"
  fi
done

for entry in "${singletons[@]}"; do
  IFS='|' read -r name cmd <<<"$entry"
  out="$dir/$name.json"
  # shellcheck disable=SC2086  # $cmd is a two-word command, split on purpose
  if "$fb" $cmd -o json >"$out" 2>>"$log"; then
    record "$name" 1 ok
    say "$name: ok"
  else
    rm -f "$out"
    record "$name" 0 failed
    failed=1
    say "$name: FAILED (see backup.log)"
  fi
done

pdf_count=0
pdf_failed=0
if [ "$no_pdfs" != true ] && [ -f "$dir/invoices.json" ]; then
  mkdir -p "$dir/pdfs/invoices"
  while read -r id; do
    if "$fb" invoices pdf "$id" -o "$dir/pdfs/invoices/$id.pdf" 2>>"$log"; then
      pdf_count=$((pdf_count + 1))
    else
      pdf_failed=$((pdf_failed + 1))
      failed=1
      say "invoice $id pdf: FAILED (see backup.log)"
    fi
  done < <(jq -r '.[].id' "$dir/invoices.json")
  say "invoice pdfs: $pdf_count written, $pdf_failed failed"
fi

printf '%s\n' "${entries[@]}" | jq -R -s \
  --arg created "$stamp" \
  --arg version "$("$fb" version 2>/dev/null || echo unknown)" \
  --argjson pdfs "{\"skipped\": $([ "$no_pdfs" = true ] && echo true || echo false), \"written\": $pdf_count, \"failed\": $pdf_failed}" \
  --argjson ok "$([ "$failed" -eq 0 ] && echo true || echo false)" \
  '{created_at: $created, cli_version: $version, ok: $ok, invoice_pdfs: $pdfs,
    resources: (split("\n") | map(select(length > 0) | split("\t") | {(.[0]): {count: (.[1] | tonumber), status: .[2]}}) | add)}' \
  >"$dir/manifest.json"

say "done: $dir (ok=$([ "$failed" -eq 0 ] && echo yes || echo no))"
exit "$failed"
