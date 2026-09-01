# Work order: Phase 2 batch a implementer (Invoices)

Dispatch: `Agent(subagent_type: "general-purpose", model: "sonnet", name: "phase-2a-impl")`.

---

You are implementing **Phase 2 batch a (Invoices)** of `freshbooks-tools`, a public MIT Go monorepo. Work ONLY inside `<repo root>/.worktrees/a` on branch `phase-2/a` (already created, clean). Do not touch other branches or worktrees. All `git` and `mise` commands run from inside the worktree.

## Read first (pointers, not pasted)

1. The oracle: `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` sections 3 (every `STATE AS OF` callout; the 2026-08-23 ones are live-CONFIRMED ground truth) and 5.1. Section 2 is locked; do not redesign.
2. Conventions: `CLAUDE.md` (toolchain, commits, green rule, parity contract, public-repo hygiene), `GOAL.md` stage 2.
3. **FreshBooks docs before coding** (they carry response examples the Postman collection lacks): https://www.freshbooks.com/api/invoices, /api/invoice_profiles, /api/items, /api/payments, /api/invoice_presentation_attachments, /api/online-payments. There is no docs page for retainers; use the Postman examples and mark retainer facts INFERRED.
4. Inventory entries: `freshbooks/internal/inventory/testdata/inventory.json`, folder `Invoices`, EXCEPT the exclusions below. Every request you implement gets a `// inventory: <full key>` comment on its method.
5. Exemplar to mirror in shape and style: `freshbooks/identity.go` + `freshbooks/identity_test.go` (service methods, fixtures, stacked inventory comments), `freshbooks/transport.go` (how requests are made -- use the existing transport helpers, never raw http).

## Scope: exactly which inventory keys

You own every key under `Invoices/` **plus** 6 keys from `Settings/Items and Services`, **minus** 5 keys owned by batch d. Total 51 keys.

**Added to your batch** (same method+URL duplicates of your `Invoices/Items and Services/*` keys -- implement ONE method per operation and stack every duplicate key as its own `// inventory:` comment line, exactly like `freshbooks/identity.go:86-87` does):

- `Settings/Items and Services/Create Item`
- `Settings/Items and Services/Delete Item`
- `Settings/Items and Services/List Items`
- `Settings/Items and Services/List Items Filtered by SKU`
- `Settings/Items and Services/Single Item`
- `Settings/Items and Services/Update Item`

**Removed from your batch** (do NOT implement, do NOT touch their ignore.list lines -- batch d owns them):

- `Invoices/Other Income/Create Single Other Income`, `.../Delete Single Other Income`, `.../List Other Income`, `.../Update Single Other Income` (4 -- exact duplicates of `Accounting/Other Income/*`)
- `Invoices/Upload Logo/Upload Logo` (1 -- duplicate of the Uploader endpoint; batch d owns all `/uploads/` endpoints)

**Duplicate-key rule** (applies inside your batch too, e.g. `Single Invoice` vs `Single Invoice w/ Logo`, `Share Link` vs `Share PDF`, the three `Create Invoice` variants): when two keys are the same method+URL+operation differing only in example body/query, implement ONE method and stack all their `// inventory:` comments on it. When the same method+URL carries genuinely different operations (FreshBooks deletes and sends via `PUT` with different bodies: `Delete Invoice` = `vis_state`, `Send Invoice by Email` = `email_recipients` action), give each operation its own method with its own key.

## Services you own (files you create)

`InvoicesService` (invoices.go), `InvoiceProfilesService` (invoice_profiles.go), `ItemsService` (items.go), `PaymentsService` (payments.go), `RetainersService` (retainers.go). Method vocabulary per spec 5.1: `List`, `All`, `Get`, `Create`, `Update`, `Delete` + resource verbs (`Send`, `PDF`, `ShareLink`, ...). Wire each service by adding its field initialization ONLY if `client.go` does not already do it -- check first; the fields are pre-declared, and `newClient` wires them centrally. If wiring requires touching `client.go`, add exactly your services' lines and nothing else.

Retainers are `/comments/business/{businessId}/retainer...` -- business family (`BusinessID` int64, flat envelope, `meta` pagination). Everything else in your batch is accounting family (`AccountID` string, enveloped).

## ignore.list

Remove EXACTLY the `//go:inventory-todo` lines for the 51 keys you implement (your Invoices keys minus the 5 exclusions, plus the 6 Settings item keys). Touch no other line, no comments, no reordering.

## Deliverables

- One `<resource>.go` + `<resource>_test.go` pair per service above, typed models with `json` tags (pointer fields for optionals on write structs), request/list options per spec 5.1.
- Fixtures under `freshbooks/testdata/<resource>/*.json` seeded from the docs pages' examples with synthetic IDs (never real account IDs, tokens, names, or emails).
- Tests: table-driven `httptest` per method family; sad paths (error envelope decode, 404, validation) per family; `t.Run` tags `[happy] [sad] [edge] [corner]` where they aid triage.
- Coverage >= 90% module-wide inside the worktree (`mise run cover`).
- Doc comments on every exported identifier.
- `freshbooks/CHANGELOG.md` `[Unreleased]` entry in the existing style.
- `mise run check` green on a clean tree (run from the worktree; it includes inventory-check -- your parity must balance).
- Conventional commits `feat(freshbooks): ...`, TDD-sized, each green. **Stage and commit in separate Bash calls.** Do NOT push, do NOT merge.

## Gotchas

- Accounting URLs take string `account_id`; business URLs take int64 `business_id`. Never convert one to the other.
- Accounting responses are enveloped `{"response":{"result":{...}}}`; the transport un-peels this -- your code never sees the envelope. Business family is flat with `meta`.
- Invoice "delete" is `PUT` with `vis_state: 1`; "send" is `PUT` with an action body. Model them as `Delete`/`Send` methods.
- `search[field]=value` filter encoding for accounting; the business family (retainers) is INFERRED `field=value` -- batch c owns confirming that callout; encode via the existing `Search` option type either way.
- Docs are ASCII-only, no hard wraps. Fixture IDs synthetic.
- If the docs disagree with the spec, the docs win: add a `> **STATE AS OF 2026-09-01**` callout in the affected spec section in the same commit and list it in your report.
- Run `scripts/redaction-check.sh` before each commit.

## Reporting (both channels)

When done (check green, committed, `git status --porcelain` empty in the worktree): write the report to `docs/phases/2/reports/a-implementer.md` (commit it in the worktree), send the same report with `SendMessage` to `team-lead` (full report in `message`, not `summary`), AND return it as your final text. Report: files created/changed, test counts, module coverage, the exact `mise run check` tail, `git log --oneline main..phase-2/a`, `git status --porcelain` output, inventory keys covered (count), and every spec discrepancy or ambiguity you hit and how you resolved it. If genuinely blocked, report the blocker the same way instead of guessing.
