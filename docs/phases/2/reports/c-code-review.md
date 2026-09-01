# Phase 2 batch c -- code-review lane report

Branch `phase-2/c` @ `47fdab4`, worktree `.worktrees/c`, diff `git diff main...phase-2/c` (7 commits, 58 files, +3595/-50).
Scope: spec sections 3 + 5.1, `docs/phases/2/plan-c.md` (47 keys), exemplars = merged batches a+b on `main`.
Read-only: no edits, no commits, no `mise`/test/build runs.

## Verdict: REQUEST CHANGES

Six blocking findings. Five are the same wire-fidelity classes the previous two gates hit; the sixth (`pathSegment`) is a trust-boundary regression against a convention every merged batch-a/b file follows. The batch is otherwise well-built: family routing matches the inventory classifier, the duplicate-key stacking is right, `ignore.list` is surgical, the spec callout is honest, and the redaction check is clean.

---

## BLOCKING

### 1. No caller-supplied path segment is validated -- `pathSegment` is never called in batch c

Every merged accounting-family file validates the `AccountID` before interpolating it (`items.go:186`, `clients.go:160`, `expenses.go:154`, `taxes.go:82`, `invoices.go:514`, `payments.go:321,324,333,344` -- 18 call sites across batches a+b). Batch c has **zero**.

Unvalidated `AccountID` interpolations:
- `freshbooks/tasks.go:44,57,77,114,127`
- `freshbooks/services_svc.go:107,122`
- `freshbooks/staff.go:98,122,136`
- `freshbooks/systems.go:44`
- `freshbooks/settings.go:81,112`

Unvalidated caller-supplied **string** segments -- the case the work order called out explicitly:
- `freshbooks/team_members.go:80` -- `teamMemberUUID string` concatenated straight into `/auth/api/v1/businesses/<id>/team_members/` + uuid.
- `freshbooks/settings.go:195` -- `clientID string` concatenated into `/auth/api/v1/partners/applications/` + clientID.

Why it matters, in `pathSegment`'s own words (`transport.go:199-210`): these values "come from callers today, and once the CLI and MCP server exist will also arrive from flags, config files, and model-authored tool inputs", and "escaping it at the call site does not help: `resolve` re-decodes the path through `url.Parse` and rebuilds it from the decoded form". A UUID or client id carrying `/`, `?`, `#`, or `..` silently retargets the request -- e.g. `Get(ctx, biz, "../../users/me")` reads a different resource, and `"x?foo=bar"` injects a query parameter. `AccountID` is the same hazard with an established precedent.

Fix: add the `pathSegment` guard to each of the 15 sites, following the `itemsPath`/`itemPath` helper pattern in `items.go:185-198` (a `<resource>Path(...) (string, error)` helper per file keeps it to one guard per path shape). The two string-id methods need the guard on the id argument too, not just on the numeric parents.

### 2. `System` drops ~28 captured response fields, including the whole address its own doc comment advertises

`freshbooks/systems.go:12-31` models 18 fields. The captured `Settings/Systems/Get System` response in `freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json` carries 46. Missing, non-exhaustively: `street`, `street2`, `province`, `mob_phone`, `info_email`, `systemid`, `test_system`, `modern_system`, `masterlock_billing`, `auto_bill`, `payment_amount` (Money), `gst_amount` (Money), `payment_frequency`, `timezoneid`, `business_type`, `vat_name`, `vat_number`, `num_clients`, `num_staff`, `referralid`, `referring_url`, `landing_url`, `heard_about_us_via`, `salutation`, `size_limit`, `split_token`, `discountid`, `dst`, `ip`, `migrated_to_smux_at`.

The doc comment at `systems.go:9-11` says the record carries "currency, **address**, billing cycle" -- but `street`, `street2`, and `province` are exactly the address fields that were dropped; `city` and `code` alone are not an address. This is a documented-behaviour mismatch, not just an omission.

Fix: model the captured fields (the two `Money`-shaped ones as `Money`, the nullable scalars as pointers), or -- if a subset is deliberate -- say so explicitly in the doc comment and name what is omitted and why. Silence reads as "this is the whole record".

### 3. Timestamps typed `string` where `DateTime` parses them, against the merged convention

Every merged batch-a/b model types the accounting `updated` field as `DateTime`: `items.go:34`, `taxes.go:33`, `expenses.go:87`, `clients.go:82`. Batch c reverts to `string` in eight places, and in every case the captured wire value is a layout `DateTime` already accepts:

| site | field | captured value | layout |
|---|---|---|---|
| `tasks.go:20` | `Task.Updated` | `"2019-04-18 09:14:53"` | `DateTimeLayout` |
| `services_svc.go:78` | `BillableItem.Updated` | (INFERRED from `Task`) | `DateTimeLayout` |
| `staff.go:85` | `Staff.Updated` | `"2019-04-24 14:37:58"` | `DateTimeLayout` |
| `staff.go:84` | `Staff.SignupDate` | `"2019-04-18 09:14:53"` | `DateTimeLayout` |
| `staff.go:83` | `Staff.LastLogin` | `null` | (nullable timestamp) |
| `systems.go:30` | `System.Date` | `"2019-04-18 09:14:53"` | `DateTimeLayout` |
| `team_members.go:32` | `TeamMember.CreatedAt` | `"2023-01-24T17:17:56Z"` | `RFC3339Layout` |
| `team_members.go:33` | `TeamMember.UpdatedAt` | `"2023-03-01T09:45:42Z"` | `RFC3339Layout` |
| `team_members.go:31` | `TeamMember.InvitationDateAccepted` | `null` | (nullable timestamp) |

Callers now have to re-parse strings the library already knows how to parse, and cannot compare or sort these without knowing which of four layouts to expect. Note the irony: this batch *extended* `dateTimeLayouts` (`types.go:165`) to cover a fourth format, then typed nine timestamp fields as `string`.

Fix: `DateTime` for all of them (the nullable ones as `DateTime` -- `UnmarshalJSON` already maps `null` to the zero value -- or `*DateTime` if the null/epoch distinction matters).

### 4. Fixtures mirror the implementation instead of the captured responses

This is the root cause of findings 2 and 5 surviving a green test suite. `freshbooks/testdata/systems/get.json` contains exactly the 18 fields `System` models and not one of the other 28 -- so `TestSystemsGet` cannot fail on the omission. `freshbooks/testdata/projects/list_full.json` drops `group_id` (present on all five projects in the captured `List Projects` response) and rewrites `"description": null` to `""`.

The work-order template names this smell directly: "fixtures that mirror the implementation instead of the docs". Fixtures are supposed to be the captured response with synthetic identifiers substituted -- nothing else removed. When a fixture is trimmed to the model, the model can never be found wrong.

Fix: reseed each fixture from the captured Postman response body, substituting only identifiers, names, and emails. Any field left out of the model should still be present in the fixture (Go's decoder ignores it) so the next reader can see what is being dropped.

### 5. `Project` drops `group_id`; `Get` drops the `abilities` sibling

- `freshbooks/projects.go:13-39`: the captured `Projects/List Projects` response carries `group_id` (int) on every project; `Single Project` and `Create Single Project` carry the expanded `group` object instead. The model has `Group json.RawMessage` (`projects.go:38`) but no `GroupID`, so `List` silently discards the only handle a caller has on the project's team -- and `List` is the endpoint most likely to be used for it.
- `freshbooks/projects.go:41-43`: `projectResponse` decodes `{"project": ...}` only. The captured `Single Project` response has a 17-entry `abilities` array as a **sibling** of `project` -- the per-project permission set, distinct from the business-wide `ProjectsService.Abilities` endpoint (`projects.go:187`), which returns a different 9-entry list. `Get` throws it away.

Fix: add `GroupID int64 \`json:"group_id"\`` to `Project`; give `Get` a return that surfaces the sibling abilities (a second return value, or a `ProjectWithAbilities` wrapper), or document the drop explicitly.

### 6. `Timer.IsRunning` carries `omitempty` on a write path -- a timer can never be stopped

`freshbooks/time_entries.go:12`:

```go
IsRunning bool `json:"is_running,omitempty"`
```

`Timer` is not read-only: it is a field on `TimeEntryUpdateRequest` (`time_entries.go:143`, `Timer *Timer json:"timer,omitempty"`). With `omitempty` on a non-pointer bool, `is_running: false` marshals to nothing, so `Update(..., &TimeEntryUpdateRequest{Timer: &Timer{ID: "t1", IsRunning: false}})` sends `{"timer":{"id":"t1"}}` and the running timer stays running. This is the exact toggle-bool class from the last two gates.

Fix: drop `omitempty` (the captured response always carries the field), or make it `*bool`.

---

## ADVISORY

### 7. Other captured response fields silently absent from read models

- `TimeEntry` (`time_entries.go:17-36`) drops `pending_client`, `pending_project`, `pending_task` -- present on every entry in the captured `List Entries`, `Create`, and `Update` responses.
- `Page[TimeEntry]` cannot carry the captured `meta.total_logged` / `meta.total_unbilled`. That is a `Page` shape constraint, not a batch-c bug, but the aggregate is the main reason to call this endpoint; worth a doc-comment note pointing callers at `Do` if they need it.
- `Business` (`settings.go:30-36`) drops `business_group` (with its full member list), `phone_number`, and `business_clients` from the captured `Add Business` response.
- `Staff` (`staff.go:57-86`) drops `accounting_systemid` -- the handle that correlates a staff record with `System.AccountID`. (`api_token` is also dropped; if that is a deliberate no-credentials-in-models decision, say so in the doc comment -- the security lane will want it stated rather than inferred.)
- `BusinessGroupMember` (`staff.go:13-24`) drops `unacknowledged_change`.

### 8. Nullable fields typed as non-pointer `string`, inconsistently within the same struct

`Staff` types `Rate *string` (`staff.go:70`) but `CurrencyCode` (`:69`), `Note` (`:71`), `HomePhone` (`:73`), and `LastLogin` (`:83`) as plain `string` -- and the captured `Single Staff` response returns **all five** as `null`. Same in `TeamMember` (`team_members.go:16-31`: `middle_name`, `job_title`, `street_1`, `city`, `province`, `country`, `postal_code`, `phone_number` are all `null` in both captured responses) and `BusinessAddress` (`settings.go:19-26`: `street`, `city`, `province`, `postal_code` all `null`).

`null` decodes to `""` without error, so nothing breaks -- but the caller cannot distinguish "unset" from "empty", and the struct contradicts itself about which nullables deserve a pointer. Pick one rule and apply it per struct.

Special case worth fixing regardless: `System.BusinessUUID string` (`systems.go:15`) directly contradicts its own doc comment two lines up -- "businessUuid is nullable in the payload; not every account has one wired up" -- and the captured response does return `null`. `systems_test.go:30-32` even asserts the `""` behaviour, locking in the mismatch.

### 9. `/auth/api/v1/businesses/...` is passed `FamilyBusiness`, which the library's own classifier contradicts

`team_members.go:56,81` pass `FamilyBusiness` for paths under `/auth/`. `familyForPath` (`client.go:205-208`) classifies every `/auth/` path as `FamilyAuth`, and it is live on a real code path (`transport.go:39`, `(*Client).Do`). So the same URL gets two different families depending on whether the caller uses `TeamMembers.List` or `Do`.

The choice is defensible -- the captured `List Team Members` body is `{"response": [...], "meta": {...}}`, and `unwrap` under `FamilyAuth` (`transport.go:360-362`) would return only `env.Response`, losing `meta` and therefore pagination. But `Family` also selects the **error** shape (`errors.go:131`), so these two methods now decode auth-family errors through the flat business path. The comment at `team_members.go:36-40` explains the envelope reason but not the error consequence.

Fix: either teach `unwrap` to keep `meta` for the auth family (the general fix, since any auth-family list will hit this), or extend the comment to state the error-decoding trade-off and add a `[sad]` test proving an auth-shaped error still resolves to the right sentinel through `FamilyBusiness`.

### 10. `time_entries.go:54` claims "CONFIRMED live" for a docs-only finding

The doc comment reads "CONFIRMED live against https://www.freshbooks.com/api/parameters, 2026-09-01". A docs page is not a live call, the implementer report says "Confirmed via docs, not live", and the spec callout it points at correctly says "docs-only; this phase made no live calls". This is a public doc comment in a library whose whole INFERRED/CONFIRMED discipline depends on the word meaning one thing.

Fix: drop "live" -- "CONFIRMED against the FreshBooks docs (docs-only; live confirmation pending)".

### 11. `ApplicationUpdateRequest` cannot clear an optional field

`settings.go:180-183`: `Description`, `WebsiteURL`, `SettingsURL`, `LogoPublicID` are non-pointer `string` with `omitempty` on what the captured Postman body shows to be a full-replace PUT (all seven fields sent every time). A caller who wants to blank a description cannot: `""` marshals to nothing. Either drop `omitempty` (matching the captured request, which always sends all seven) or make them `*string`.

### 12. `systems_test.go` asserts 3 of 18 modeled fields

`systems_test.go:27-32` checks `ID`, `CurrencyCode`, `Country`, and the `BusinessUUID` null behaviour. Fifteen other modeled fields -- including every one whose JSON tag could be wrong -- are unasserted, so a typo'd tag on `date_format`, `billing_status`, or `accountid` passes. Compare `staff_test.go` / `team_members_test.go`, which assert far more of their shape. (Related: the `[sad]` case reuses the shared `accounting/error_404` fixture rather than a systems-specific one, which is fine, but means this endpoint has no fixture-backed error of its own.)

---

## Checked and clean

Stating these explicitly so the next gate does not re-derive them:

- **`omitempty` on struct-typed write fields (class 1): clean.** Batch c has no non-pointer struct field on any write struct; `Money`, `Date`, `DateTime`, and `Timer` are all reached through pointers (`tasks.go:32`, `services_svc.go:93`, `projects.go:60`, `time_entries.go:143,112`). The merged `omitzero` convention (`bills.go`, `credit_notes.go`, `estimates.go`) is not violated because the situation never arises.
- **Non-pointer fields on partial-update structs (class 3): clean.** `ProjectUpdateRequest` (`projects.go:126-133`), `TaskUpdateRequest` (`tasks.go:99-104`), `TimeEntryUpdateRequest` (`time_entries.go:136-144`), and `StaffUpdateRequest` (`staff.go:107-112`) are fully pointer-typed. `ApplicationUpdateRequest` is the one exception and it is a full-replace PUT (advisory 11).
- **Family routing matches the inventory classifier** for every key except the two in advisory 9 -- which also match it (`My Team/List Team Members` and `Single Team Member` are classified `business` in `inventory.json`, contrary to path intuition). `Invite` -> `FamilyAuth`, `Staff.List` -> `FamilyAuth`, `ProvisionPayments` -> the documented `FamilyBusiness` fallback for `payments`: all correct.
- **URL templates match `pathTemplate` for all 47 keys**, including the two `my.freshbooks.com`-sourced rewrites (`projects.go:165`, `settings.go:81`), both flagged INFERRED in their doc comments as instructed.
- **`Projects/Delete Project` sends a request body on a DELETE** -- matches the captured Postman request exactly (`{"project":{"vis_state":1}}`); not an invention.
- **Duplicate-key stacking** is correct: 3 stacked comments on `TimeEntries.List` (`time_entries.go:60-62`), 2 on `TeamMembers.UpdateRate` (`team_members.go:143-144`); `Staff.Update`/`Delete` and `Tasks.Update`/`Delete` correctly split.
- **`ignore.list` diff removes exactly the 47 in-scope lines**, no others; the three batch-d keys and the batch-a/b duplicates are untouched.
- **Redaction clean**: no `api.freshbooks@gmail.com`, `Postman Sandbox`, `wkMd2g`, or the real account/business/identity IDs (`1966214`, `1882548`, `37256`) in any new fixture.
- **Spec callout** (`section 5.1`) is honest about docs-vs-live and correctly surfaces the `-field` sort discrepancy as a deferred, out-of-scope design decision rather than silently changing shared code. Good judgement; the deferral is the right call.
- **No `t.Skip`, no committed `-run` filters, no `time.Now()`/`rand` in the new tests.** 103 `t.Run` subtests across nine new files; 15 of them assert the outgoing request body.
- **CHANGELOG** entry is in the existing `[Unreleased] / Added` style and accurate.
- Placement of `Settings/Businesses/*` and `Settings/Developer/*` on `IdentityService` in a new `settings.go` (implementer ambiguity 5) is a reasonable call given the constraint and is documented at `settings.go:9-15`. Same for `Abilities` on `ProjectsService` by path root (ambiguity 6). No objection; a dedicated `BusinessesService` only becomes worth it if that surface grows.
