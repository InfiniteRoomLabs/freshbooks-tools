# Work order: Phase 7 implementer (live-conformance pass)

Dispatch: `Agent(subagent_type: "general-purpose", model: "opus", name: "phase-7-impl")`. Attended phase: the lead holds the credentials path and the sandbox decision; the implementer never sees a token value.

Stage 1 (lead, 2026-09-02): `fnox exec` resolves `FRESHBOOKS_CLIENT_ID`/`FRESHBOOKS_CLIENT_SECRET` (64 chars each). No stored token existed (`~/.config/freshbooks/` absent), so the live smoke failed at `no stored token`; the lead ran the released `freshbooks auth login` (cli/v0.1.0) to obtain one -- that login IS end-to-end fact S1 below. The CLI stores credentials per context under `$XDG_CONFIG_HOME/freshbooks/credentials/<context>.json`; the lib's live suite reads `FRESHBOOKS_ACCESS_TOKEN` or the lib token file, so the bridge for the suite is `FRESHBOOKS_ACCESS_TOKEN="$(freshbooks auth token)"` in the same shell, never echoed.

---

You are implementing **Phase 7 (live conformance)** of `freshbooks-tools` in `<repo root>` on branch `phase-7/live` (checked out, clean; this plan is its first commit). Everything API-facing in this repo is docs-confirmed only; this phase turns each docs-only or INFERRED fact into a CONFIRMED one backed by a capture on disk, or corrects the code.

## Read first (pointers)

1. `CLAUDE.md`; `docs/progress.md` (current state, backlog item 7); the spec `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` sections 3 and 5.1 -- every `> **STATE AS OF**` callout there is the fact list's source (lines ~43-80 and ~150-165).
2. `freshbooks/live_test.go` (the existing read-only smoke; extend it, keep the `live` tag + `FRESHBOOKS_LIVE=1` gate), `freshbooks/testdata/seed/` (the Phase 1 captures: the format to mirror -- verbatim body, synthetic IDs), `scripts/redaction-check.sh`.
3. How to make a live call: `FRESHBOOKS_LIVE=1 FRESHBOOKS_ACCESS_TOKEN="$(freshbooks auth token)" fnox exec -- mise exec -- go test -tags live -count=1 ./freshbooks/...` from `<repo root>`. `freshbooks auth token` (the released CLI, on PATH or `/tmp/relbin/freshbooks`) prints the current access token and refreshes through the store when needed; never write it to a file, never `echo` it, never paste it into a report. Scope ids come from `freshbooks identity me -o json`.

## Facts (each row: what to observe, how, read or write)

Read-only rows run against whichever account the lead authorized. Write rows run ONLY if the lead has confirmed a sandbox account in `docs/phases/7/reports/lead-sandbox.md`; otherwise mark them DEFERRED in the report with the reason, do not attempt them.

| Id | Fact | Probe | R/W |
|---|---|---|---|
| B | Business-family filter encoding is bare `field=value` | `TimeEntries.List` with `updated_since`; capture query + response | R |
| C | Business-family sort direction: docs say `-field`, lib sends `field_desc` | `Projects.List` with `Sort("updated_at", desc)`; then the same with a raw `sort=-updated_at` via `Client.Do`; compare ordering and any 4xx | R |
| D | `/events/` callbacks list uses the accounting envelope | `Callbacks.List`; capture | R |
| E | `/payments/` gateways answer flat (business family) | `Gateways.Get`; capture | R |
| F | Ledger family is flat `{"data": ...}`; taxonomy endpoints `Types`/`SubTypes` shape | `LedgerAccounts.List`, `.Types`, `.SubTypes`; capture | R |
| O | `PageMeta` drops `meta.sort` on Projects list | `Projects.List`; inspect raw `meta` via `Client.Do` into `json.RawMessage` | R |
| P | `StaffService.List` discards sibling business fields | `Staff.List` raw vs decoded; list the dropped keys | R |
| Q | `DateTime` zoneless `YYYY-MM-DDTHH:MM:SS` producers | `Projects.List`, `TimeEntries.List`: which fields carry which of the four formats | R |
| J1 | `Expenses.Vendors` returns a bare string array | capture | R |
| S1 | Real `auth login` through the self-signed loopback (done by the lead; record the transcript shape, redacted) | -- | R |
| S2 | CLI end to end: `freshbooks identity me`, `freshbooks invoices list --per-page 2 -o json` | capture stdout with IDs replaced | R |
| S3 | MCP end to end: `freshbooks-mcp serve --transport http` on loopback, `tools/call identity_whoami` with the bearer | capture the JSON-RPC exchange, token redacted | R |
| G | Invoice delete verb: PUT (lib) vs DELETE-with-body (docs) | create a draft invoice, delete it both ways, capture both responses, clean up | W |
| H | `Expenses.Delete` sends `vis_state: 1` (Postman says 0) | create + delete an expense; verify `vis_state` after | W |
| I | `Expenses/Create Custom Expense Category` works at all (docs say unsupported) | attempt create; capture success or the exact error | W |
| J2 | `Contacts.Update` and `Expenses.CreateRecurring` response envelopes | create a client with a secondary contact, update it; create a recurring expense; capture; clean up | W |
| K | Payment-options body (`entity_id`/`entity_type`) and the quoted-`entity_id` echo | `Invoices.EnablePaymentOptions` on the draft invoice; capture | W |
| L | Webhook verify/resend bodies carry `callback_id` (Postman) vs not (docs) | `Callbacks.Register` to a throwaway URL, `ResendVerification`, capture; delete | W |
| M | Checkout-link create/update response shape | `Payments.CreateCheckoutLink`; capture; delete | W |
| R | Quoted-ID writes (string ids accepted where ints are documented) | one update with a quoted id; capture | W |
| A | PKCE enforcement (wrong verifier rejected?) | DEFERRED unless the lead runs a second consent; note only | -- |
| N | Tokenization shapes (`paid.freshbooks.com`) | DEFERRED: needs a test card and a connected gateway; note only | -- |

## Deliverables

- For every row that ran: a verbatim capture under `freshbooks/testdata/seed/<resource>/<name>.json` with real account/business ids, names, emails, addresses, and amounts replaced by synthetic values (keep shapes and key sets exact; the redaction check must pass; grep the capture for the real ids you saw in `identity me` before staging). Synthesize every integer and uuid in a capture, not only the top-level ids: system-assigned row ids on nested members (`jea_id`, `jesa_id`, and the like) are real values too, and the redaction check cannot see them because it only knows the configured term list. One `TestLive<Fact>` per row in `freshbooks/live_test.go` (or a sibling `live_<resource>_test.go`, same tag) that makes the call and asserts the shape the capture shows.
- Spec callouts: each confirmed fact's callout gets `CONFIRMED (live, 2026-09-02)` in place; each corrected fact gets a new `> **STATE AS OF 2026-09-02 (Phase 7, live)**` line with the observed truth. Same commit as the code change.
- Lib fixes where the wire disagreed, each its own commit: `fix(freshbooks): <fact>`; unit fixtures re-seeded from the capture (synthetic ids) where the old fixture was struct-derived. Candidates: C (`Sort()` for the business family), O (`PageMeta.Sort`), P (`Staff` fields), G (delete verb), J2/K shapes.
- `docs/progress.md` backlog items 1, 4, 5, and 7 resolved or re-cut; `freshbooks/CHANGELOG.md` `[Unreleased]` entries for every lib change.
- Coverage floors hold; `mise run check` green; checkpoint commits per resource family; `scripts/redaction-check.sh` before each commit.

## Reporting

`docs/phases/7/reports/implementer.md` (commit it), `SendMessage` to `team-lead` (full report in `message`), and the same as your final text: the fact table with CONFIRMED / CORRECTED / DEFERRED per row and the capture path, the lib changes, the gate tail, coverage, `git log --oneline main..phase-7/live`, `git status --porcelain`.
