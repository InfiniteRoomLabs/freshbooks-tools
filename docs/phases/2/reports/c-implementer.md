# Phase 2 batch c implementer report (Projects + Time Tracking + My Team + Settings)

Branch `phase-2/c`, worktree `.worktrees/c`. Not pushed, not merged.

## Files created / changed

New service files (+ paired `_test.go`, + `freshbooks/testdata/<resource>/*.json`):

- `freshbooks/projects.go` -- `ProjectsService`: `Create`, `Get`, `List`, `All`, `Update`, `Delete`, `Abilities`, `Threads`, `CreateThread`, `AddThreadComment`.
- `freshbooks/tasks.go` -- `TasksService`: `Create`, `Get`, `List`, `All`, `Update`, `Delete`.
- `freshbooks/time_entries.go` -- `TimeEntriesService`: `List`, `Search`, `Create`, `Update`, `Delete`.
- `freshbooks/services_svc.go` -- `ServicesService`: `Get`, `List` (business family), `Create`, `GetBillableItem` (accounting family).
- `freshbooks/service_rates.go` -- `ServiceRatesService`: `Get`, `List`, `UpdateProjectRate`.
- `freshbooks/team_members.go` -- `TeamMembersService`: `List`, `All`, `Get`, `InvitationRates`, `Rates`, `UpdateRate`, `Invite`.
- `freshbooks/staff.go` -- `StaffService`: `List`, `Get`, `Update`, `Delete`.
- `freshbooks/systems.go` -- `SystemsService`: `Get`.
- `freshbooks/settings.go` -- new methods on the existing `IdentityService` (see "Settings/Businesses and Settings/Developer" below): `AddBusiness`, `DeleteBusiness`, `DeleteBusinessSubscription`, `ProvisionPayments`, `CreateApplication`, `Applications`, `UpdateApplication`.

Other changes:

- `freshbooks/types.go` + `types_test.go` -- `DateTime` now accepts a fourth wire format, the zoneless `"YYYY-MM-DDTHH:MM:SS"` timestamp Projects' `created_at`/`updated_at` use (none of the three documented layouts match it; this batch's Projects/List Projects test would not otherwise pass). Additive only -- `MarshalJSON` behavior and the three existing layouts are unchanged.
- `freshbooks/internal/inventory/testdata/ignore.list` -- removed exactly the 47 `//go:inventory-todo` lines this batch implements (verified via `mise run inventory-check`: `implemented 51, ignored 0, todo 162, uncovered 0, double-covered 0, stale 0, unknown 0`). No other line touched.
- `freshbooks/CHANGELOG.md` -- `[Unreleased] / Added` entry for this batch.
- `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` section 5.1 -- filter-encoding callout resolved + a new sort-encoding discrepancy callout (see below).

## Test counts / coverage

- Module-wide (`freshbooks` package): 98.4% (`go test -race -coverprofile`).
- `mise run cover` (whole-module gate, floor 90%): **PASS at 95.7%** total across `freshbooks` + `freshbooks/auth` + `freshbooks/internal/inventory`.
- This batch added 12 new `_test.go` files with ~70 top-level `func Test*` covering every method, `[happy]`/`[sad]`/`[edge]` tagged; sad paths cover both accounting-envelope errors (`errors_test` style enveloped `{"response":{"errors":[...]}}`) and business-flat errors (`{"error": ...}`/`{"message": ...}`) per family, per the CLAUDE.md gotcha.
- Module-wide test suite (incl. Phase 1): 128 top-level `Test*` functions, 341 subtests, all green, `-race` clean.

## `mise run check` tail (post-commit, clean tree)

```
== inventory-check: freshbooks ==
implemented 51, ignored 0, todo 162, uncovered 0, double-covered 0, stale 0, unknown 0
...
== cover: mcp ==   PASS (100.0%)
== cover: cli ==   PASS (100.0%)
== build ==
build: done, artifacts in .../dist
check.sh: all OK
```

No dirty-tree banner -- ran after the final commit, `git status --porcelain` empty.

## `git log --oneline main..phase-2/c`

```
78ee14a feat(freshbooks): add Settings/Businesses and Developer methods, finish batch c
128cb1e feat(freshbooks): add TeamMembersService, StaffService, SystemsService
e20ecde feat(freshbooks): add ServicesService and ServiceRatesService
0a06fce feat(freshbooks): add TimeEntriesService, resolve filter-encoding callout
8e9f92e feat(freshbooks): add ProjectsService and TasksService
94ba0d9 feat(freshbooks): accept a zoneless DateTime wire format
```

## `git status --porcelain`

Empty (clean tree, all 6 commits landed).

## Inventory keys covered

47/47, matching the work order's exact list (Projects 19, Time Tracking 7, My Team 7, Settings 14 after the stage-1 duplicate-ownership re-cut). Duplicate-key stacking applied per the work order's rule:

- `TimeEntriesService.List` carries 3 stacked comments (`List Entries`, `Time Entries Updated Since Precise Time`, `Time Entries for a Given Day`).
- `TeamMembersService.UpdateRate` carries 2 (`My Team/Update Staff Rates`, `Projects/Update Team Member Rate`).
- `Staff.Update` / `Staff.Delete` and `Tasks.Update` / `Tasks.Delete` are separate methods per their separate keys, per the work order (same PUT endpoint, different `vis_state` semantics).

## Filter-encoding resolution (the premise task)

Confirmed via docs, not live (this phase is unattended). `https://www.freshbooks.com/api/parameters` states plainly that accounting endpoints filter via `search[key]=value` and business/project/time-tracking endpoints filter via bare `field=value`. `TimeEntriesService.List` is the first business-scoped list endpoint implemented and its test asserts `updated_since`/`include_deleted` land as bare query keys, never `search[updated_since]`. Spec 5.1 updated with a `STATE AS OF 2026-09-01` callout recording this and that live confirmation is still pending.

**Bonus finding, not fixed:** the same docs page shows the business family sorts via a leading `-field` prefix for descending (`?sort=-started_at`), not the `_desc` suffix `Sort()` emits for both families today -- corroborated by the Postman example for `Time Tracking/Time Entries For Employee on Specific Project` (`sort=-started_at`). I did **not** change `Sort()`/`requestOptions.values()` in `types.go`/`options.go`: doing so would change behavior for every existing business-family call site, including Phase 1's own `types_test.go` assertions, and is a design decision for whoever next owns those shared files, not scoped to this batch. Documented as a second `STATE AS OF` note in the same spec callout and in `TimeEntriesService.Search`'s doc comment. A caller needing the real `-field` form today can pass it directly via `Search{"sort": "-started_at"}`.

## Spec discrepancies and ambiguities hit, and how I resolved them

1. **Zoneless DateTime format** (see above) -- extended `dateTimeLayouts`, additive only.
2. **`Settings/Items and Services` conflates two different resources under one folder name.** `Get a Single Service` / `List Services` hit the business-family `/comments/business/.../service(s)` endpoint (light record: id, business_id, name, billable, vis_state). `Create Service` / `Single Service` hit the accounting-family `/accounting/account/.../billable_items/...` endpoint instead of the docs page's `/comments/business/.../service` -- confirmed from the Postman `pathTemplate`/`family` fields, not assumed. Modeled as two types (`Service`, `BillableItem`) on the one `ServicesService`, with the split explained in both types' doc comments. `BillableItem`'s response shape is INFERRED -- neither Postman example carries a response body; I extrapolated from the sibling `Task` shape (tasks are billable items under the hood: the Tasks docs page requires `user:billable_items:*` scopes) and the `Create` request body's own fields.
3. **`Projects/Delete Project`** -- the collection's only my.freshbooks.com-sourced project-delete request, and its rewritten path (`/comments/business/.../project/{id}`) doesn't match the `Get`/`Update`/`List` path root (`/projects/business/...`). Implemented against the rewritten path per the work order's explicit instruction, flagged INFERRED in the method doc comment. Its 404 example also carries a different flat-error shape (`{"error_type", "message"}` rather than the usual `{"error": ...}`) -- covered by a dedicated fixture/test; `decodeError`'s `message` fallback still decodes it correctly (`errors.Is(err, ErrNotFound)` holds).
4. **`Settings/Businesses/Delete Business - Subscription`** -- same my.freshbooks.com-rewrite situation, this time landing on an `/auth/`-prefixed path (`FamilyAuth`). Implemented, flagged INFERRED.
5. **No pre-declared service fits `Settings/Businesses/*` or `Settings/Developer/*`.** Per the work order, these landed on the closest existing service (`IdentityService`, since it already models `Membership`/business identifiers) rather than declaring a new one -- in a **new file** `settings.go` so Phase 1's `identity.go` is untouched. Flagging this placement for the review gate as instructed; a future phase may want a dedicated `BusinessesService`/`DeveloperService` if the surface grows.
6. **`Settings/Abilities/Abilities`** -- its path (`/projects/business/{id}/abilities`) is rooted at Projects, so it landed on `ProjectsService` rather than `IdentityService`, on the "closest service by path" principle rather than by folder name.
7. **Three Developer-application endpoints and three Projects-discussion endpoints have no genuine Postman response examples** (`Get all applications`' example is a mislabeled copy-paste of an unrelated Identity Info response). For the discussion endpoints (`Threads`, `CreateThread`, `AddThreadComment`) I return the decoded body undecoded (`map[string]any` / `[]map[string]any`) rather than invent a typed shape with zero evidence. For the Developer-application endpoints I did model a typed `Application` struct (evidence: the `Create`/`Modify` request bodies, which strongly imply the readable/writable field set), but flagged INFERRED throughout and in the CHANGELOG.
8. **`Settings/Businesses/Provision FreshBooks Payments`** is a `payments`-family endpoint (per the inventory classifier) that remains in my scope (unlike the two `payments`-family keys the work order moved to batch d). I implemented it against the current `FamilyBusiness` fallback for the still-unverified `payments` envelope (spec 5.1's existing callout, owned by batch d) -- its Postman example returns `201` with a `null` body, so there's no envelope to get wrong either way; no spec change needed here since batch d owns that callout's resolution.
9. **`My Team/List Staff`** doesn't hit the accounting Staff resource at all -- it's `/auth/api/v1/users/business/{id}`, returning a business-group-members list, while `Single`/`Update`/`Delete Staff` hit the accounting `/users/staffs/{id}` resource. Modeled as two types (`BusinessGroupMember` for `List`, `Staff` for the rest) with the split explained in the doc comment, matching FreshBooks' own naming conflation rather than fighting it.
10. **`My Team ` folder name carries a trailing space in the Postman source** -- copied every inventory key verbatim from `inventory.json`/`ignore.list` rather than hand-typing, so this never became a mismatch.

No blockers. All 47 keys implemented, tested, and green.
