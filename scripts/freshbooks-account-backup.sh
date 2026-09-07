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
# going and exits 1 at the end so a cron job notices. Each run gets a fresh
# 0700 directory and refuses to reuse one: a backup is the whole business's
# books, and the operator owns retention (nothing here prunes old runs).
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
# name|command -- the reads that return one document, not a list. The
# command is two words, split on purpose below; the list is a literal.
singletons=(
  "identity|identity me"
  "gateways|gateways get"
  "systems|systems get"
)

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
dir="$root/$stamp"
umask 077
mkdir -p "$root"
# No -p: a second run in the same second, or a pre-created directory or
# symlink at this name, must fail loudly rather than write into it.
mkdir "$dir"
log="$dir/backup.log"
failed=0
entries=()   # "name<TAB>count<TAB>status"; status is ok, partial, or failed

record() { entries+=("$1	$2	$3"); }
say() { printf 'backup: %s\n' "$*" | tee -a "$log" >&2; }
# The CLI prints `null` for an empty collection (a nil slice); every file
# this script writes is a JSON array, so consumers can rely on `.[]`.
# `jq -e` fails on anything that is not JSON and on empty input (no result
# produced), and every caller checks that: a command that exits 0 with
# junk or nothing on stdout is a failed resource, not an ok one with an
# empty file.
as_array() { jq -e 'if type == "array" then . else [] end'; }

# Writes a JSON array of every item in a collection to $2. Uses --all where
# the command offers it; otherwise fetches page 1 and, for page-shaped
# output, walks --page 2..N by hand. A command that prints pages but has no
# --page flag (retainers) can only yield page 1: that is recorded as
# "partial" when more pages exist. Any failed page fails the collection.
# Returns 0 for ok, 2 for partial, 1 for failed.
fetch_collection() {
  local res="$1" out="$2" help first kind pages p page tmp total
  help="$("$fb" "$res" list --help 2>>"$log" || true)"
  case "$help" in
  *--all*)
    first="$("$fb" "$res" list --all -o json 2>>"$log")" || return 1
    as_array <<<"$first" >"$out" 2>>"$log" || return 1
    return 0
    ;;
  esac
  first="$("$fb" "$res" list -o json 2>>"$log")" || return 1
  kind="$(jq -er type <<<"$first" 2>>"$log")" || return 1
  if [ "$kind" != object ]; then
    as_array <<<"$first" >"$out" 2>>"$log" || return 1
    return 0
  fi
  pages="$(jq -r '.pages // 0' <<<"$first")"
  total="$(jq -r '.total // empty' <<<"$first")"
  tmp="$(mktemp)"
  jq '.items // []' <<<"$first" >"$tmp"
  case "$help" in
  *--page*)
    for ((p = 2; p <= pages; p++)); do
      page="$("$fb" "$res" list --page "$p" -o json 2>>"$log")" || { rm -f "$tmp"; return 1; }
      jq '.items // []' <<<"$page" >>"$tmp" 2>>"$log" || { rm -f "$tmp"; return 1; }
    done
    ;;
  *)
    if [ "$pages" -gt 1 ]; then
      jq -s 'add // []' <"$tmp" >"$out"
      rm -f "$tmp"
      say "$res: prints $pages pages but has no --page flag; only page 1 saved"
      return 2
    fi
    ;;
  esac
  jq -s 'add // []' <"$tmp" >"$out"
  rm -f "$tmp"
  if [ -n "$total" ] && [ "$(jq length "$out")" != "$total" ]; then
    say "$res: page 1 promised $total items, got $(jq length "$out")"
    return 2
  fi
  return 0
}

for res in "${collections[@]}"; do
  out="$dir/$res.json"
  status=0
  fetch_collection "$res" "$out" || status=$?
  case "$status" in
  0)
    n="$(jq length "$out")"
    record "$res" "$n" ok
    say "$res: $n items"
    ;;
  2)
    n="$(jq length "$out")"
    record "$res" "$n" partial
    failed=1
    say "$res: PARTIAL, $n items saved (see backup.log)"
    ;;
  *)
    rm -f "$out"
    record "$res" 0 failed
    failed=1
    say "$res: FAILED (see backup.log)"
    ;;
  esac
done

for entry in "${singletons[@]}"; do
  IFS='|' read -r name cmd <<<"$entry"
  read -r -a argv <<<"$cmd"
  out="$dir/$name.json"
  if "$fb" "${argv[@]}" -o json >"$out" 2>>"$log"; then
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
    # Ids come from the API; the file name is built from them, so only a
    # plain integer is accepted (the CLI would refuse anything else too).
    case "$id" in
    '' | *[!0-9]*)
      pdf_failed=$((pdf_failed + 1))
      failed=1
      say "invoice id '$id': skipped, not an integer"
      continue
      ;;
    esac
    if "$fb" invoices pdf -o "$dir/pdfs/invoices/$id.pdf" -- "$id" 2>>"$log"; then
      pdf_count=$((pdf_count + 1))
    else
      pdf_failed=$((pdf_failed + 1))
      failed=1
      say "invoice $id pdf: FAILED (see backup.log)"
    fi
  done < <(jq -r '.[].id' "$dir/invoices.json" 2>>"$log")
  say "invoice pdfs: $pdf_count written, $pdf_failed failed"
fi

# Written to a temp name and renamed, so a failure here can never leave a
# truncated manifest behind.
jbool() { if "$@"; then echo true; else echo false; fi; }
if printf '%s\n' "${entries[@]}" | jq -R -s \
  --arg created "$stamp" \
  --arg version "$("$fb" version 2>/dev/null || echo unknown)" \
  --argjson skipped "$(jbool [ "$no_pdfs" = true ])" \
  --argjson written "$pdf_count" \
  --argjson pdf_failed "$pdf_failed" \
  --argjson ok "$(jbool [ "$failed" -eq 0 ])" \
  '{created_at: $created, cli_version: $version, ok: $ok,
    invoice_pdfs: {skipped: $skipped, written: $written, failed: $pdf_failed},
    resources: (split("\n") | map(select(length > 0) | split("\t") | {(.[0]): {count: (.[1] | tonumber? // 0), status: .[2]}}) | add)}' \
  >"$dir/manifest.json.tmp" 2>>"$log"; then
  mv "$dir/manifest.json.tmp" "$dir/manifest.json"
else
  rm -f "$dir/manifest.json.tmp"
  say "manifest.json: FAILED to write"
  failed=1
fi

say "done: $dir (ok=$([ "$failed" -eq 0 ] && echo yes || echo no))"
exit "$failed"
