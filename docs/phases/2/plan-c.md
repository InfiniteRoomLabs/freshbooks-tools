# Work order: Phase 2 batch c implementer (Projects + Time Tracking + My Team + Settings)

Dispatch: `Agent(subagent_type: "general-purpose", model: "sonnet", name: "phase-2c-impl")`.

---

You are implementing **Phase 2 batch c (Projects + Time Tracking + My Team + Settings)** of `freshbooks-tools`, a public MIT Go monorepo. Work ONLY inside `<repo root>/.worktrees/c` on branch `phase-2/c` (already created, clean). Do not touch other branches or worktrees. All `git` and `mise` commands run from inside the worktree.

## Read first (pointers, not pasted)

1. The oracle: `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` sections 3 (every `STATE AS OF` callout; the 2026-08-23 ones are live-CONFIRMED ground truth) and 5.1 -- **especially the INFERRED business-family filter-encoding callout in 5.1, which YOUR batch resolves.** Section 2 is locked; do not redesign.
2. Conventions: `CLAUDE.md`, `GOAL.md` stage 2.
3. **FreshBooks docs before coding** (they carry response examples the Postman collection lacks): https://www.freshbooks.com/api/project (note: singular), /api/tasks, /api/time_entries, /api/services, /api/team-members, /api/staff, /api/parameters (query-parameter reference -- your filter-encoding evidence), /api/identity_model, /api/how-to-add-time-entries-for-your-employees.
4. Inventory entries: `freshbooks/internal/inventory/testdata/inventory.json`, folders `Projects`, `Time Tracking`, `My Team`, `Settings`, EXCEPT the exclusions below. Every request you implement gets a `// inventory: <full key>` comment on its method.
5. Exemplar to mirror in shape and style: `freshbooks/identity.go` + `freshbooks/identity_test.go` (service methods, fixtures, stacked inventory comments), `freshbooks/transport.go` (use the existing transport helpers, never raw http).

## Scope: exactly which inventory keys

You own every key under `Projects/`, `Time Tracking/`, `My Team/`, `Settings/` **minus** 14 keys owned by other batches. Total 47 keys.

**Removed from your batch** (do NOT implement, do NOT touch their ignore.list lines):

Owned by batch a (duplicates of `Invoices/Items and Services/*`):
- `Settings/Items and Services/Create Item`, `.../Delete Item`, `.../List Items`, `.../List Items Filtered by SKU`, `.../Single Item`, `.../Update Item` (6)

Owned by batch b (duplicates of the Expenses/Accounting tax endpoints):
- `Settings/Items and Services/List Taxes`, `.../Create Single Tax`, `.../Update Tax`, `.../Single Tax (GET)`, `.../Single Tax (DELETE)` (5)

Owned by batch d (payments/uploads families):
- `Settings/Businesses/Gateway Details`, `Settings/Gateways/List Gateways` (2 -- same GET /payments gateway endpoint)
- `Settings/Developer/Upload App Logo` (1 -- POST /uploads/images)

You still own the 5 service keys in `Settings/Items and Services` (`Create Service`, `Get a Single Service`, `Get a Single Service Rate`, `List Services`, `Single Service`) -- they are business-family services/rates endpoints, natural fits for `ServicesService`/`ServiceRatesService`.

**Duplicate-key rule** (applies inside your batch): when two keys are the same method+URL+operation differing only in example query/body, implement ONE method and stack all their `// inventory:` comments on it, exactly like `freshbooks/identity.go:86-87` (e.g. the three `Time Tracking` list variants -- `List Entries`, `Time Entries Updated Since Precise Time`, `Time Entries for a Given Day` -- are one `List` method with options carrying three stacked comments; `My Team/Update Staff Rates` and `Projects/Update Team Member Rate` share one PUT endpoint). When the same method+URL carries genuinely different operations (delete-via-PUT with `vis_state`), give each its own method with its own key (`Staff.Update` vs `Staff.Delete`, `Tasks.Update` vs `Tasks.Delete`).

## Services you own (files you create)

`ProjectsService` (projects.go), `TasksService` (tasks.go), `TimeEntriesService` (time_entries.go), `ServicesService` (services_svc.go -- `services.go` already exists for the struct declarations; pick a non-colliding filename), `ServiceRatesService` (service_rates.go), `TeamMembersService` (team_members.go), `StaffService` (staff.go), `SystemsService` (systems.go). Settings keys that are auth-family (`Settings/Abilities`, `Settings/Businesses/*` remaining, `Settings/Developer/*` remaining) go on the service the inventory path implies -- if none of the 36 pre-declared services fits (e.g. developer applications), note it in your report and put the method on the closest service rather than declaring a new exported service; flag it for the gate.

Families in your batch: Projects/Time Tracking/Services are business family (`BusinessID` int64, flat + `meta`); My Team is accounting family (`/accounting/account/.../users/staffs`) EXCEPT team-member-rate endpoints (`/comments/business/`); Settings spans auth (`/auth/api/v1/...`), accounting (`Systems`), and business. Read each inventory entry's `pathTemplate` + `family` field and trust it over folder intuition.

## YOUR premise task: resolve the business-family filter-encoding INFERRED callout

Spec 5.1 flags the business family's bare `field=value` filter encoding as INFERRED (docs-only). Your first business-scoped list endpoint (projects or time entries): gather the docs evidence (the /api/parameters and /api/time_entries pages show filter examples), then update the callout in spec 5.1 with a `> **STATE AS OF 2026-09-01**` note recording what the docs confirm and that live confirmation remains pending (this phase is unattended -- no live calls). If the docs contradict bare `field=value`, implement what the docs show and say so in the callout + your report.

## ignore.list

Remove EXACTLY the `//go:inventory-todo` lines for the 47 keys you implement. Touch no other line.

## Deliverables

- One `<resource>.go` + `<resource>_test.go` pair per service above, typed models with `json` tags (pointer fields for optionals on write structs), request/list options per spec 5.1.
- Fixtures under `freshbooks/testdata/<resource>/*.json` seeded from the docs pages' examples with synthetic IDs (never real account IDs, tokens, names, or emails).
- Tests: table-driven `httptest` per method family; sad paths per family (business-family flat errors AND accounting envelope errors -- you span both); `t.Run` tags `[happy] [sad] [edge] [corner]` where they aid triage.
- Coverage >= 90% module-wide inside the worktree (`mise run cover`).
- Doc comments on every exported identifier.
- `freshbooks/CHANGELOG.md` `[Unreleased]` entry in the existing style.
- The spec 5.1 filter-encoding callout updated (see above).
- `mise run check` green on a clean tree (run from the worktree; includes inventory-check -- your parity must balance).
- Conventional commits `feat(freshbooks): ...`, TDD-sized, each green. **Stage and commit in separate Bash calls.** Do NOT push, do NOT merge.

## Gotchas

- Two `my.freshbooks.com`-sourced entries live in your folders (the collection's only delete-project and delete-business-subscription requests); the inventory tool already rewrote them to public `api.freshbooks.com` paths. Implement against the public path, mark the fact INFERRED (Postman-only evidence) in the method's doc comment, and note it in your report.
- Business-family errors are flat (`{"error": ...}` or `{"message": ...}`); accounting errors are enveloped. The transport handles both; your tests still cover the decode per family.
- `My Team ` carries a trailing space in the Postman source; inventory keys are trimmed -- always copy keys verbatim from `inventory.json`.
- Docs are ASCII-only, no hard wraps. Fixture IDs synthetic.
- If the docs disagree with the spec, the docs win: `> **STATE AS OF 2026-09-01**` callout in the same commit + report it.
- Run `scripts/redaction-check.sh` before each commit.

## Reporting (both channels)

When done (check green, committed, `git status --porcelain` empty in the worktree): write the report to `docs/phases/2/reports/c-implementer.md` (commit it in the worktree), send the same report with `SendMessage` to `team-lead` (full report in `message`, not `summary`), AND return it as your final text. Report: files created/changed, test counts, module coverage, the exact `mise run check` tail, `git log --oneline main..phase-2/c`, `git status --porcelain` output, inventory keys covered (count), the filter-encoding resolution, and every spec discrepancy or ambiguity you hit and how you resolved it. If genuinely blocked, report the blocker the same way instead of guessing.
