# Phase 2 batch c -- gate triage

Lead triage of the three read-only lane reports (`c-code-review.md` REQUEST CHANGES 6 blocking / 6 advisory; `c-simplify.md` 5 apply-recommended / 3 optional; `c-security.md` BLOCK, 2 blocking / 3 advisory). Review 1 == security 1 == simplify 5 (one fix). One fix commit: `fix(lib-resources-c): apply the review-gate findings`. QA runs after.

## Fix list (apply ALL, one commit)

- **F1** (review 1 + security 1 + simplify 5): extract `*Path` helpers returning `(string, error)` guarded by `pathSegment`, per the `items.go:185-198` pattern, covering the 12 `AccountID` interpolations (`tasks`, `services_svc`, `staff`, `systems`, `settings`) AND the two raw string ids (`team_members.go` `teamMemberUUID`, `settings.go` `clientID`). Business-family int64 builders stay non-erroring (the `retainers.go` precedent). One `[sad]` hostile-segment test per affected resource asserting error before any HTTP request.
- **F2** (security 2): resynthesize `testdata/settings/business_added.json` -- replace the vendor-sourced identifiers and personal name with the corpus conventions (`ACM123`, `Example Business LLC`, synthetic ints) and update `settings_test.go` assertions.
- **F3** (review 2 + review 12): model the captured `System` fields (Money-shaped ones as `Money`, nullable scalars as pointers) or, where a field is deliberately omitted, name it and why in the doc comment; fix the "currency, address, billing cycle" claim; extend `systems_test.go` to assert the shape meaningfully.
- **F4** (review 4): reseed every fixture this batch authored from the CAPTURED Postman response with only identifiers/names/emails substituted -- fields the model does not declare STAY in the fixture (decoder ignores them; readers see the drops). Named: `systems/get.json`, `projects/list_full.json` (restore `group_id`, keep `"description": null` as null), plus any fixture F3/F8 touches.
- **F5** (review 3): the nine timestamp fields typed `string` become `DateTime` (`Task.Updated`, `BillableItem.Updated`, `Staff.{Updated,SignupDate,LastLogin}`, `System.Date`, `TeamMember.{CreatedAt,UpdatedAt,InvitationDateAccepted}`).
- **F6** (review 5): add `Project.GroupID int64` (`json:"group_id"`). For the `Single Project` sibling `abilities` array: DOCUMENT the drop in `Get`'s doc comment with a pointer to `(*Client).Do` for callers who need it (lead call: no wrapper type this phase; revisit if a real consumer appears).
- **F7** (review 6): `Timer.IsRunning` loses `omitempty` (captured responses always carry it; a timer must be stoppable).
- **F8** (review 7): add the captured-but-dropped fields: `TimeEntry.{PendingClient,PendingProject,PendingTask}`, `Business.{PhoneNumber, BusinessClients or a documented drop, BusinessGroup json.RawMessage if shape is complex}`, `Staff.AccountingSystemID`, `BusinessGroupMember.UnacknowledgedChange`. `Staff.api_token` is deliberately NOT modelled -- state the no-credentials-in-models decision in the doc comment. Add a doc-comment note on `TimeEntries.List` about the captured `meta.total_logged`/`total_unbilled` aggregates being unreachable through `Page` (escape hatch: `Do`).
- **F9** (review 8): per-struct nullable-pointer consistency: `Staff`, `TeamMember`, `BusinessAddress` -- pick pointers for captured-null fields and apply uniformly; `System.BusinessUUID` -> `*BusinessUUID` (its own doc comment says nullable); update the test that locked in `""`.
- **F10** (review 9): keep `FamilyBusiness` on the two team-members methods but extend the comment with the error-shape trade-off AND add a `[sad]` test proving an auth-shaped error still resolves to the right sentinel through `FamilyBusiness`.
- **F11** (review 10): fix the `time_entries.go:54` doc comment: not "CONFIRMED live" -- "CONFIRMED against the FreshBooks docs (docs-only; live confirmation pending)".
- **F12** (review 11): `ApplicationUpdateRequest` is a full-replace PUT per the captured body: drop `omitempty` from its string fields and say "full replace" in the doc comment.
- **F13** (security 3): `Application` and `ApplicationUpdateRequest` get redacting `String()` renderings (`ClientSecret: redacted`), mirroring `auth.Token`, with doc comments; note on `Application` that MCP tool output must strip `client_secret` (Phase 3 inherits the constraint in writing).
- **F14** (security 5): one sentence on `ProjectsService.Delete` and `IdentityService.DeleteBusinessSubscription`/`DeleteBusiness` doc comments: destructive and irreversible; CLI/MCP surfaces must require explicit confirmation and must not expose unattended.
- **F15** (simplify 1): `tasksListResponse` embeds `PageMeta`; `timeEntriesListResponse.Meta` becomes `PageMeta`; the six `Page[T]` literals become `newPage(...)`.
- **F16** (simplify 2 + 3, signature change APPROVED by lead -- zero external callers, one list convention per package): introduce `{Project,Task,TimeEntry,Service,TeamMember}ListOptions` with `opts()` delegating to `listOpts`; `List(ctx, id, opts *XListOptions, extra ...RequestOption)`; the three `All` iterators build the options struct, apply `pageSize`, and thereby fix the caller-`PageNumber`-pins-the-iterator bug (name both behavior deltas in the commit body). simplify 6 (shared time-entries list helper) lands with it.
- **F17** (simplify 4): `Tasks.Delete` and `Staff.Delete` delegate to `(*Client).softDelete`.

## Explicit non-applies (do NOT do)

- simplify 7 (ProjectServiceRate embed), 9-14: agreed DO-NOT-APPLY as filed; do not re-derive.
- simplify 8 (dead `testdata/projects/list.json`): pre-existing on main, outside this diff -- phase-close chore, not this commit.
- security 4 (govulncheck not installed locally): `mise run vuln` covers it in QA/CI.
- review 7's `Page` shape limitation (`total_logged`): doc-comment note only (in F8), no Page redesign.
- Missing `All` on `TimeEntries`/`Services` despite a `meta` block: deferred to the phase-close convergence checklist, not this commit.

## Lane verdict reconciliation

Three-way convergence on the pathSegment gap (F1) -- the third batch in a row; the fix-forward is mechanical and the pattern now covers the whole library. Security's fixture finding (F2) stands as blocking: the rule is explicit even though the values are public in the vendor collection. Review 4's fixture-mirrors-implementation root cause matches batch b's QA finding; F4 applies the same captured-response rule. No lane-vs-lane conflicts.
