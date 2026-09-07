# Account backup

`scripts/freshbooks-account-backup.sh` is a worked example of driving the `freshbooks` CLI from a shell script. It reads everything the CLI can list for one context and writes it to disk as JSON, plus one PDF per invoice. It never writes to FreshBooks.

## Run it

```bash
# credentials and scope come from the CLI's normal chain: a context set with
# `freshbooks config set-context`, or FRESHBOOKS_* environment variables
export FRESHBOOKS_CLIENT_ID=<your client id>
export FRESHBOOKS_CLIENT_SECRET=<your client secret>
scripts/freshbooks-account-backup.sh --out ~/backups/freshbooks
```

The script needs `freshbooks` (logged in: `freshbooks auth status`), `jq`, and `usage` (its interpreter, like the other scripts in `scripts/`; `mise install` provides it). `FRESHBOOKS_BIN` overrides which `freshbooks` binary runs, which the selftest uses to substitute a fake.

| Flag | Meaning |
|---|---|
| `-o, --out <dir>` | Backup root. A UTC-timestamped directory (`20260907T164500Z`) is created inside it. Default `./freshbooks-backup`. |
| `--no-pdfs` | Skip the one-request-per-invoice PDF pass. |

## What lands on disk

```
<out>/<timestamp>/
  backup.log            the script's own progress lines plus every stderr line the CLI produced, in run order
  manifest.json         cli version, ok flag, per-resource count and status (ok, partial, failed), pdf tallies
  <resource>.json       one JSON array per collection (24 files)
  identity.json         `identity me`: the business memberships the token can see
  gateways.json         `gateways get`: configured payment gateway connections
  systems.json          `systems get`: business-level settings
  pdfs/invoices/<id>.pdf
```

The 24 collections are every `<resource> list` command the CLI has: bill-vendors, bills, callbacks, clients, credit-notes, estimates, expense-categories, expenses, invoice-profiles, invoices, items, journal-entry-accounts, ledger-accounts, other-income, payments, projects, retainers, service-rates, services, staff, tasks, taxes, team-members, time-entries. Where a command offers `--all` the script uses it; where it does not, the script fetches page 1 and walks `--page 2..N` itself, or keeps the array as is for the unpaginated lists. Every collection file is a flat JSON array either way (an empty collection, which the CLI prints as `null`, is written as `[]`), so `jq length` is the item count and `.[]` always iterates.

One command, `retainers list`, prints pages but registers no `--page` flag. If an account has more retainers than one page holds, the script keeps page 1, records the resource as `partial`, and exits 1. A collection whose page 1 promises a `total` the walk did not deliver is recorded as `partial` too.

`time-entries list-with-totals` is the same wire request as `time-entries list` and is not repeated.

## Failure handling

A resource that fails is logged to `backup.log`, leaves no file, and is marked `"status": "failed"` in the manifest; a failed page fails its whole collection rather than silently shortening it. The run continues. The exit status is 1 if anything failed or was partial and 0 otherwise, so a cron job or a `systemd` timer can tell; if the manifest itself cannot be written the exit status is still 1.

Each run creates its own timestamped directory and refuses to start if that directory already exists (a second run within the same second, or a name someone pre-created): nothing is ever overwritten.

## Handling the output

- The output directory is created with mode 0700 (`umask 077`), and the CLI writes PDFs 0600. A backup is the whole business's books, including client contact details and every invoice line: treat the directory like the credentials file.
- Nothing prunes old runs. An unattended schedule accumulates a full copy of the books per run; retention is yours.
- Keep the backup root out of git working trees, synced cloud folders, and shared or temporary directories. If a copy leaves the machine, encrypt it first.
- `backup.log` holds the CLI's stderr verbatim. An API error message is server-supplied text and can quote record contents, so the log is as sensitive as the data files.

## What it does not do

- **Restore.** FreshBooks has no bulk import, and every write endpoint takes one record at a time. The JSON is the CLI's own `-o json` shape, so a restore script would feed each file back through `<resource> create --file`, but that is not written and not tested.
- **Reports.** The `reports` commands (profit-loss, balance-sheet, aging, and so on) take date ranges and are not "everything the token can list"; run the ones you want after the backup.
- **Per-record sub-resources** that the CLI exposes only as `get <id>` (an invoice's payment options, a client's contacts) are already embedded in the parent record where FreshBooks returns them, and are not fetched separately.

## Its test

`scripts/backup-selftest.sh` (`mise run backup-selftest`, also part of `mise run check`) runs the script against a fake `freshbooks` that answers from canned JSON: an `--all` collection, a two-page collection walked by hand, an unpaginated array, a collection that fails, a PDF that fails, and the three singletons. It checks the files, the manifest, the 0700 mode, the exit status, `--no-pdfs`, the empty-collection and hostile-id shapes, and that a pre-existing directory at the run's name is refused. No network.
