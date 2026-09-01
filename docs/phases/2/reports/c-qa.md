# Phase 2 batch c -- QA / reality-check lane

Subject: branch `phase-2/c` at `532185f` (`fix(lib-resources-c): apply the review-gate findings`), worktree `.worktrees/c`. Oracle: spec sections 3 and 5.1, `docs/phases/2/plan-c.md` (47 inventory keys), `docs/phases/2/triage-c.md` (F1-F17).

## Verdict: NEEDS WORK

Two BLOCKING findings, six ADVISORY. The gate is green, the tree is clean, coverage clears the floor, parity balances, and the mandatory acceptance probe found **zero decode failures across 30 captured responses**. The blockers are both fidelity defects the gate cannot see: one endpoint implemented against a path the FreshBooks docs contradict and whose only captured evidence is a 404, and one of F16's two named behavior deltas that did not actually land.

## 1. Gate: PASS

```
mise run check   (run once, from the worktree, exit code captured directly -- not through a pipe)
GATE_EXIT=0
```

Tail:

```
== cover: freshbooks ==
coverage-gate: .../freshbooks/coverage.out total = 92.1% (floor 90%)
coverage-gate: PASS
== vuln: freshbooks ==
No vulnerabilities found.
== inventory-check: freshbooks ==
implemented 161, ignored 0, todo 52, uncovered 0, double-covered 0, stale 0, unknown 0
== cover: mcp ==
coverage-gate: .../mcp/coverage.out total = 100.0% (floor 90%)
coverage-gate: PASS
== cover: cli ==
coverage-gate: .../cli/coverage.out total = 100.0% (floor 90%)
coverage-gate: PASS
== actionlint ==
== build ==
build: done, artifacts in .../dist
check.sh: all OK
```

Per-module coverage: freshbooks 92.1%, mcp 100.0%, cli 100.0%. `git status --porcelain` was empty before the gate, empty after it (the `dist/` build output is gitignored), and empty again after the probe was deleted. Only `docs/phases/2/reports/c-qa.md` (this file) is uncommitted, as instructed.

`mise run vuln` covers triage's non-apply of security 4 (govulncheck not installed locally): clean for all three modules.

## 2. MANDATORY acceptance test: PASS (0 decode failures)

Throwaway probe `freshbooks/qa_probe_test.go` (in-package, deleted afterwards; tree verified clean). It parsed `freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json`, pulled every captured response body under `Projects`, `Time Tracking`, `My Team`, and `Settings`, served each verbatim from `newTestClient`'s httptest server, and called the real batch-c service method -- so the transport's envelope unwrapping and every custom unmarshaler (`DateTime`, `Date`, `Money`, `VisState`, `BusinessUUID`) ran on real captured bytes. A second reflection walk compared each unwrapped payload's JSON keys against the declared `json` tags of the receiving type to count silent drops.

```
PROBE SUMMARY: 30 cases, 0 decode failures, 32 dropped fields total
```

Per-model silent drops, and whether each is accounted for:

| Response | Drops | Accounted for? |
|---|---|---|
| Projects/List Projects | `meta.sort` | No -- see ADVISORY 4 |
| Projects/Single Project | `abilities` | Yes -- `Project` doc comment |
| Projects/Update Project | `abilities` | Partly -- doc comment names Get only (ADVISORY 5) |
| Projects/Create Single Project | 0 | -- |
| Projects/Delete Project | `error_type`, `message` | n/a -- captured body is a 404 (BLOCKING 1) |
| Projects/{Service Rates, Update Service Rates, Team Member Rates, Update Team Member Rate, Invitation Rates} | 0 | -- |
| Projects/Tasks/{List, Single, Create, Update, Delete} | `taskid`, `tname`, `tdesc` (x5) | No -- see ADVISORY 3 |
| Time Tracking/List Entries | `meta.total_logged`, `meta.total_unbilled` | Yes -- F8 doc note on `timeEntriesListResponse` |
| Time Tracking/{Create, Update} | 0 | -- |
| My Team/{List Team Members, Single Team Member, Update Staff Rates} | 0 | -- |
| My Team/List Staff | 7 sibling business fields | No -- see ADVISORY 6 |
| My Team/{Single Staff, Update Staff} | `staff.api_token` | Yes -- deliberate, F8 no-credentials rule |
| My Team/Delete Staff | `errors` | n/a -- captured body is a 403 (ADVISORY 2) |
| Settings/Abilities/Abilities | 0 | -- |
| Settings/Businesses/{Add Business, Delete Business - Subscription, Provision FreshBooks Payments} | 0 | -- |
| Settings/Systems/Get System | 0 | -- |

Out-of-probe by design: `Settings/Items and Services/*` item and tax keys (batch a/b), `Settings/{Businesses/Gateway Details, Gateways/List Gateways}` and `Developer/Upload App Logo` (batch d), and `Settings/Developer/Get all applications` -- whose captured "response" is a 119KB copy-pasted Identity Info body, not this endpoint's shape. The implementer identified that mislabel independently and says so in `settings.go`'s `Applications` doc comment; I confirm the mislabel is real.

## 3. Fidelity spot-check: 8 inventory entries against 5 docs pages

Sources: https://www.freshbooks.com/api/project, /api/tasks, /api/time_entries, /api/team-members, /api/staff.

| Entry | Docs say | Code sends | Verdict |
|---|---|---|---|
| Projects/List Projects | `GET /projects/business/<id>/projects` | same (`projects.go:135`) | MATCH |
| Projects/Single Project | `GET /projects/business/<id>/project/<pid>` (singular) | `/projects/.../projects/<pid>` (plural, `projects.go:101`) | DIVERGES -- ADVISORY 5 |
| Projects/Update Project | `PUT /projects/business/<id>/project/<pid>` | same (`projects.go:174`) | MATCH |
| Projects/Delete Project | `DELETE /projects/business/<id>/project/<pid>` | `/comments/business/<id>/project/<pid>` (`projects.go:198`) | DIVERGES -- BLOCKING 1 |
| Projects/Tasks/Single Task | `GET /accounting/account/<a>/projects/tasks/<t>`; response carries `id`, `taskid`, `name`, `tname`, `description`, `tdesc`, `rate{amount,code}`, `billable`, `tax1`, `tax2`, `updated`, `vis_state` | path MATCHES; `Task` (`tasks.go:12`) models 9 of 12 | ADVISORY 3 |
| Time Tracking/List Entries | `GET /timetracking/business/<id>/time_entries`; bare `field=value` filters (`?updated_since=...&include_deleted=1`, `?started_from=...&started_to=...`); meta carries `total_logged`/`total_unbilled` | all three MATCH | MATCH -- confirms the premise task |
| My Team/List Team Members | `GET /auth/api/v1/businesses/<id>/team_members`; `{"response":[...], "meta":{...}}` with meta as a sibling; 20 documented fields | path, envelope, and all 20 fields MATCH (`team_members.go:33`) | MATCH |
| My Team/Single Staff | `GET /accounting/account/<a>/users/staffs/<id>`; 29 fields incl. `api_token` | path MATCHES; `Staff` (`staff.go:80`) models 28, drops `api_token` deliberately | MATCH |

The premise task landed correctly. `/api/parameters` confirms bare `field=value` for the business family; the spec 5.1 callout records it as docs-only with live confirmation pending, exactly as the work order required. The implementer additionally surfaced an unasked-for second finding there (sort direction: business family wants `-field`, `Sort()` emits `field_desc`) -- see ADVISORY 7.

## 4. F1-F17: all seventeen verified landed

| Fix | Evidence |
|---|---|
| F1 | `pathSegment`-guarded `*Path` helpers in `tasks.go:37,44`, `services_svc.go:105,112`, `staff.go:117`, `systems.go:74`, plus inline guards in `settings.go` (`DeleteBusinessSubscription`, `ProvisionPayments`, `UpdateApplication` clientID) and `team_members.go:74` (teamMemberUUID). 14 `[sad] a hostile ... never reaches the network` subtests across 6 test files. |
| F2 | `testdata/settings/business_added.json` now carries `ACM123` / `Example Business LLC` / `Sam Owner` / `owner@example.com`. `scripts/redaction-check.sh` runs clean in the gate. |
| F3 | `System` models 46 fields; the captured `system` object has exactly 46. Probe: 0 drops. Doc comment no longer claims an address it lacks and names the nullable-pointer set. |
| F4 | Applied broadly and correctly -- `staff/single.json` keeps `api_token`, `projects/single.json` keeps the `abilities` sibling, `time_entries/list.json` keeps `total_logged`/`total_unbilled`, `projects/list_full.json` keeps `meta.sort` + `group_id` + `"description": null`, `staff/list.json` keeps all 8 sibling business fields, `systems/get.json` keeps all 46. **One gap:** the tasks fixtures -- ADVISORY 3. |
| F5 | All nine fields are `DateTime`: `Task.Updated`, `BillableItem.Updated`, `Staff.{Updated,SignupDate,LastLogin}`, `System.Date`, `TeamMember.{CreatedAt,UpdatedAt,InvitationDateAccepted}`. |
| F6 | `Project.GroupID int64` present; the `abilities` drop documented on the type and on `Get`. |
| F7 | `Timer.IsRunning` has no `omitempty`, with the stop-a-timer rationale in a comment. |
| F8 | `TimeEntry.{PendingClient,PendingProject,PendingTask}`, `Business.PhoneNumber`, `Business.BusinessClients []json.RawMessage`, `Staff.AccountingSystemID`, `BusinessGroupMember.UnacknowledgedChange` all present. `BusinessGroup` was typed rather than left raw -- better than the fix asked for, and the probe confirms 0 drops through it. `api_token` non-modeling and the `meta.total_logged` note are both written down. |
| F9 | Per-struct pointer rules stated and applied on `Staff`, `TeamMember`, `BusinessAddress`; `System.BusinessUUID` is `*BusinessUUID`. |
| F10 | Trade-off comment on `TeamMembersService.List`; `team_members_test.go:64` `[sad] an auth-shaped error still resolves through FamilyBusiness`. |
| F11 | `time_entries.go` now reads "CONFIRMED against the FreshBooks docs ... 2026-09-01; docs-only, live confirmation pending". |
| F12 | `ApplicationUpdateRequest`'s seven string fields carry no `omitempty`; the doc comment says full-replace and explains why. |
| F13 | `Application.String()` and `ApplicationUpdateRequest.String()` both render `ClientSecret: redacted`; the Phase 3 MCP constraint is written into `Application`'s doc comment. |
| F14 | Destructive-and-irreversible sentences on `ProjectsService.Delete`, `IdentityService.DeleteBusiness`, and `IdentityService.DeleteBusinessSubscription`. |
| F15 | `tasksListResponse` embeds `PageMeta`; `timeEntriesListResponse.Meta` is `PageMeta`; no `Page[T]` composite literals remain in batch-c files -- all six sites go through `newPage`. |
| F16 | Signature change landed on all five services. Delta 1 (pageSize/defaultPerPage in `All`) landed. **Delta 2 did not fully land -- BLOCKING 2.** |
| F17 | `Tasks.Delete` and `Staff.Delete` both call `s.client.softDelete`. |

## 5. Findings

### BLOCKING 1 -- `ProjectsService.Delete` sends a path the FreshBooks docs contradict, and its only evidence is a 404

`freshbooks/projects.go:198-201`

```go
path := fmt.Sprintf("/comments/business/%s/project/%d", businessID, projectID)
body := map[string]map[string]VisState{"project": {"vis_state": VisStateDeleted}}
return s.client.do(ctx, http.MethodDelete, path, FamilyBusiness, body, nil)
```

Expected (https://www.freshbooks.com/api/project): `DELETE /projects/business/<business_id>/project/<project_id>` -- the same public projects root `Get`/`Create`/`Update` already use, with no body.

Observed: the code implements the `/comments/business/...` path the inventory tool rewrote out of the collection's only `my.freshbooks.com` project request. Raw Postman URL:

```
DELETE https://my.freshbooks.com/service/api/comments/business/{{businessId}}/project/{{projectId}}
```

and that request's own captured response is:

```
status = 404 NOT FOUND
{"error_type": "not_found", "message": "The requested resource was not found."}
```

So the single piece of evidence behind this path is a failure. The docs give a public, documented alternative that the batch-c work order and `CLAUDE.md` both say wins ("If the docs disagree with the spec, the docs win"). The doc comment is honest that the path is Postman-only and INFERRED, but it never mentions that the captured example is a 404, and no one checked `/api/project` for a documented delete. Parity is not at risk: `mise run inventory-check` matches on the `// inventory:` comment, not the URL, so switching to the documented path keeps `Projects/Delete Project` covered.

Suggested fix: send `DELETE /projects/business/{businessId}/project/{projectId}` with no body, restate the doc comment (documented endpoint; the collection's internal-host variant 404'd; still not live-confirmed), and reseed `testdata/projects/delete_error_404.json` usage accordingly.

### BLOCKING 2 -- F16 delta 2 did not land: a caller-supplied `PageNumber` still pins `All()` to one page

`freshbooks/projects.go:144-153` and `:136`; same shape at `tasks.go:121` / `:113` and `team_members.go:108` / `:100`.

The fix commit's body claims:

> (2) a caller-supplied PageNumber option can no longer silently pin the iterator to one page forever

That is true only for the new `opts *XListOptions` channel. The `extra ...RequestOption` variadic survives, and `List` appends it **after** the loop's page option:

```go
reqOpts := append(opts.opts(), extra...)   // projects.go:136 -- extra wins, newRequestOptions is last-wins
```

Probe result (throwaway `TestQAProbeAllPagePin`, since deleted), calling `c.Projects.All(ctx, BusinessID(1), nil, PageNumber(3))` against a 3-page server:

```
PROBE All+extra PageNumber(3): pages requested = [3 3 3] (iterations=3)
```

Expected: pages `[1 2 3]`. Observed: page 3 fetched three times -- the original bug, reachable through the channel the signature change added. It terminates (it does not spin forever), but it silently yields the wrong rows. No test covers it; the three `All` tests (`projects_test.go:126`, `tasks_test.go:127,145`, `team_members_test.go:46`) all pass `nil`/`opts` and never exercise `extra`.

Suggested fix: in each `All` closure, put the loop's page last -- `s.List(ctx, id, &o, append(append([]RequestOption{}, extra...), PageNumber(page))...)` -- or strip page options out of `extra` inside `All`. Add one `[corner]` test per `All` asserting `[1 2 3]` with `PageNumber` passed through `extra`. Either way the commit body's delta-2 claim needs to match reality before this merges.

### ADVISORY 3 -- `Task` drops three documented response fields, and the tasks fixtures hide the drop

`freshbooks/tasks.go:12`; `freshbooks/testdata/tasks/{single,list}.json`

/api/tasks documents `taskid` (duplicate of `id`), `tname` (legacy alias for `name`), and `tdesc` (legacy alias for `description`) as response fields, and the captured Postman body carries all three with values identical to their modern twins (`"id": 55226, "taskid": 55226`, `"name"`/`"tname"` both `"Computer Hardware"`). `Task` models none of them and its doc comment does not mention them.

That alone is defensible -- no data is unreachable. The problem is F4: "reseed every fixture this batch authored from the CAPTURED Postman response ... fields the model does not declare STAY in the fixture (decoder ignores them; readers see the drops)." `tasks/single.json` and `tasks/list.json` contain zero occurrences of `taskid`/`tname`/`tdesc`; they mirror the implementation, which is the exact root cause review 4 filed and F4 was written to close. Every other batch-c fixture I checked complies. Fix: restore the three aliases to both tasks fixtures and add one sentence to `Task`'s doc comment naming them as unmodeled legacy duplicates.

### ADVISORY 4 -- `meta.sort` is dropped by the shared `PageMeta`

`freshbooks/page.go:27`. The captured `Projects/List Projects` meta is `{"sort": [], "total": 5, "per_page": 30, "page": 1, "pages": 1}`; `PageMeta` declares only the last four. Pre-existing shared code landed by batches a/b, not batch c's to change unilaterally -- but batch c is the first to hit a captured meta carrying `sort`, and `projects/list_full.json` correctly preserves it, so the drop is visible to a fixture reader and silent to a `Page` consumer. Worth a phase-close decision: model it or document it on `PageMeta`.

### ADVISORY 5 -- `Get` uses the plural project path the docs spell singular, unremarked

`freshbooks/projects.go:101` sends `/projects/business/{id}/projects/{pid}`; /api/project documents `/projects/business/<id>/project/<project_id>`. The Postman collection does use the plural form and its capture returned a real 200 body, so plural evidently works -- this is the collection and the docs disagreeing, which the "inferred vs confirmed" convention says to record. Nothing does. One sentence on `Get` (plural per the captured collection request; docs spell it singular; both appear to resolve) closes it. Separately, `Update Project`'s captured response also carries the `abilities` sibling, but `Project`'s doc comment attributes that only to `Get`.

### ADVISORY 6 -- `StaffService.List` silently discards the whole business record

`freshbooks/staff.go:57`. The auth response for `/auth/api/v1/users/business/{id}` is a full business object; `staffListResponse` declares only `business_group`, so `id`, `name`, `account_id`, `date_format`, `address`, `phone_number`, and `business_clients` are dropped -- and `Business` (`settings.go`) already models exactly those seven. The doc comment explains the staff-vs-business-group naming confusion well but never says the business fields are discarded. Also worth noting: /api/staff documents `GET /accounting/account/<accountid>/users/staffs` as the real list-staff endpoint. It is absent from the Postman collection, so parity does not require it, but the library currently has `Get`/`Update`/`Delete` for accounting Staff records and no way to list them. Flag for the phase-close convergence checklist.

### ADVISORY 7 -- business-family sort direction is wrong on every batch-c list surface (documented, deferred)

Spec 5.1's new callout records it: the business family wants `?sort=-started_at`, `Sort()` emits `started_at_desc`. The implementer correctly declined to change `types.go` inside a resource batch and wrote down the workaround (`Search{"sort": "-started_at"}`). I agree with the call, but the practical effect is that `Sort()` is silently wrong on `Projects.List`, `Tasks.List` (accounting -- unaffected), `TimeEntries.List/Search`, `Services.List`, and `TeamMembers.List`. It needs an owner and a phase, not just a callout.

### ADVISORY 8 -- two captured "success" fixtures are synthesized from failure responses

`My Team/Delete Staff`'s captured body is a 403 (`{"response":{"errors":[{"errno":1003,"message":"Permission Denied."}]}}`), so `testdata/staff/deleted.json` is invented, not captured; same story for `Projects/Delete Project` (404). `staff/error_403.json` and `projects/delete_error_404.json` do preserve the real captured failures, which is the right instinct -- but the invented success fixtures are not labelled as invented. A one-line comment key or a note in the test would keep a future reader from treating them as observed.

## 6. Test quality

Good. 121 distinct `[sad]`/`[edge]`/`[corner]` subtests across the nine batch-c test files; both error families are exercised (13 x 401, 13 x 404, 7 x 422, 2 x 429 with `Retry-After`, plus malformed-JSON and nil-request paths). No `t.Skip` outside `live_test.go:26`'s intended `FRESHBOOKS_LIVE` guard, no committed `-run` filters, no vacuous tests found -- the assertions check decoded values, request paths, and request bodies, not just non-nil. `go test -tags integration` is green for all three modules. The lowest-covered batch-c functions are 80-90% (`projects.go` `All`/`Threads`/`CreateThread`, `tasks.go` `Update`), all missing only error-return branches; nothing looks like coverage padding.

The one test-shaped gap is the `All` + `extra` case in BLOCKING 2.

## 7. Parity

`mise run inventory-check`: `implemented 161, ignored 0, todo 52, uncovered 0, double-covered 0, stale 0, unknown 0`. I independently extracted all `// inventory:` comments from the nine batch-c files: **47 keys across 44 methods**, every one present in `inventory.json`, matching plan-c exactly, with the three `Time Tracking` list variants and the two team-member-rate keys correctly stacked on single methods per the duplicate-key rule. Every method's URL matches its entry's `pathTemplate`, with the two deliberate exceptions the code documents (`Projects/Delete Project` and `Settings/Businesses/Delete Business - Subscription` are `family=internal` in the inventory and are given `FamilyBusiness`/`FamilyAuth` by hand; `Provision FreshBooks Payments` is `family=payments`, folded to `FamilyBusiness` with the spec-5.1 fallback cited). The ignore list is empty -- nothing was suppressed.

## 8. Commands run

```
mise run check                                   # once, exit 0, from the worktree
cd freshbooks && mise exec -- go test -tags integration ./...
cd mcp && mise exec -- go test -tags integration ./...
cd cli && mise exec -- go test -tags integration ./...
mise exec -- go test -run TestQAProbeCapturedResponses -v .    # throwaway probe, deleted
mise exec -- go test -run TestQAProbeAllPagePin -v .           # throwaway probe, deleted
mise exec -- go tool cover -func=coverage.out
git status --porcelain                           # empty before, during, and after
```

Note for whoever reruns the probe: bare `go` on this machine is 1.26.7 while `mise` pins 1.26.6, so `go test` outside `mise exec` fails with a toolchain-version mismatch against the cached build. Use `mise exec -- go ...`.

## 9. What to do

Fix BLOCKING 1 and BLOCKING 2 in one commit, plus ADVISORY 3 (cheap, and it closes the F4 rule properly). ADVISORY 5 and 8 are one-sentence doc-comment additions worth folding in. ADVISORY 4, 6, and 7 are cross-batch and belong on the phase-close convergence checklist, not this commit. Then re-gate and re-run the acceptance probe.

---

# Re-verification -- 2026-09-01, at `839dbf9`

Subject: `839dbf9 fix(lib-resources-c): apply the QA findings`, answering the NEEDS WORK verdict above (G1-G5 mapped to BLOCKING 1, BLOCKING 2, ADVISORY 3, ADVISORY 5, ADVISORY 8; ADVISORY 4, 6, and 7 deferred to phase-close as agreed). Focused re-check: deltas only.

## Verdict: PASS

Both blockers are fixed and empirically confirmed. All three applied advisories landed. Nothing regressed.

## Gate: PASS

```
mise run check    (exit code captured directly from the task, not through a pipe)
GATE_EXIT=0
check.sh: all OK
```

```
coverage-gate: .../freshbooks/coverage.out total = 92.1% (floor 90%)   PASS
coverage-gate: .../mcp/coverage.out        total = 100.0% (floor 90%)  PASS
coverage-gate: .../cli/coverage.out        total = 100.0% (floor 90%)  PASS
== vuln ==              No vulnerabilities found.  (all three modules)
== inventory-check ==   implemented 161, ignored 0, todo 52, uncovered 0, double-covered 0, stale 0, unknown 0
== lint: freshbooks ==  0 issues.
```

Parity is unchanged at 161/0/52/0/0/0/0, so the G1 path change did not disturb the `Projects/Delete Project` key -- as predicted, `inventory-check` keys off the `// inventory:` comment, not the URL.

Process note on the exit code, since it matters for the record: `scripts/check.sh:140-147` runs a dirty-tree check at the end of an `all` run, and it counts *untracked* files. My own `docs/phases/2/reports/c-qa.md` -- the one file the work order permits to be dirty -- therefore trips it and forces exit 1 while every substantive step passes. To get an exit code that means something, I moved the report to `/tmp` for the duration of the run and restored it immediately after; `GATE_EXIT=0` above is from that fully-clean-tree run. Two earlier runs this round exited 1 on that banner alone with all steps green. Worth flagging for the lead: as written, the QA lane cannot produce a green gate and its report at the same time.

## G1 (was BLOCKING 1) -- FIXED

`freshbooks/projects.go:196-214`

```go
path := fmt.Sprintf("/projects/business/%s/project/%d", businessID, projectID)
return s.client.do(ctx, http.MethodDelete, path, FamilyBusiness, nil, nil)
```

Matches the documented endpoint (`DELETE /projects/business/<business_id>/project/<project_id>`, no body) exactly, on the same path root `Get`/`Create`/`Update` use. The `vis_state` body is gone.

`projects_test.go:234-256` asserts both halves of the contract, not just the path:

```go
if gotMethod != http.MethodDelete || gotPath != "/projects/business/8675309/project/2976412" { ... }
if len(gotBody) != 0 { t.Fatalf("body = %q, want no body per the documented endpoint", gotBody) }
```

The doc comment is now honest in the way the finding asked for: it names the documented endpoint and its source, states plainly that the collection's internal-host request "own captured response is itself a 404 ... so it is not usable evidence for what a delete actually does", cites "the docs win" per `CLAUDE.md`, and still says it remains unconfirmed live. `testdata/projects/delete_error_404.json` -- evidence for the abandoned path -- was removed, and the `[sad]` case reuses the shared `projects/error_404.json`. Clean.

## G2 (was BLOCKING 2) -- FIXED, verified empirically

Each `All` closure now drops `Page` from the options struct and appends its own `PageNumber` after `extra`:

```go
pageOpts := append(append([]RequestOption{}, extra...), PageNumber(page))
return s.List(ctx, businessID, &o, pageOpts...)
```

The inner `append` copies into a fresh backing array each iteration, so there is no slice-aliasing hazard across pages. Re-ran my throwaway pin probe (deleted afterwards), now covering all three iterators against a 3-page server with `PageNumber(3)` passed through `extra`:

```
PROBE Projects     All+extra PageNumber(3): pages=[1 2 3] iterations=3
PROBE Tasks        All+extra PageNumber(3): pages=[1 2 3] iterations=3
PROBE TeamMembers  All+extra PageNumber(3): pages=[1 2 3] iterations=3
```

Previously `[3 3 3]`. The prior commit's delta-2 claim is now true, and the fix commit says so rather than restating the old claim. One `[corner]` regression test per `All` was added in-tree (`projects_test.go`, `tasks_test.go`, `team_members_test.go`), so this cannot silently regress.

## G3 (was ADVISORY 3) -- FIXED

All three tasks fixtures carry the legacy aliases again, with values identical to their modern twins:

```
tasks/single.json  keys: billable description id name rate taskid tax1 tax2 tdesc tname updated vis_state
                   taskid==id: True   tname==name: True   tdesc==description: True
tasks/list.json    6 alias lines (2 tasks x 3)
tasks/deleted.json 3 alias lines
```

`deleted.json` was fixed too, which I had not called out -- correct application of the F4 rule rather than the literal minimum. `Task`'s doc comment (`tasks.go:12-17`) now names `taskid`/`tname`/`tdesc` as documented duplicates of ID/Name/Description and says why they are not modeled. The F4 fixture rule now holds across every batch-c fixture.

## G4 (was ADVISORY 8) -- FIXED

Both invented success bodies are labelled at the point of use, which is the right place given JSON fixtures cannot carry comments:

- `projects_test.go:235-239` -- "No captured success example exists for this endpoint on either path (the Postman my.freshbooks.com capture is itself a 404; the docs page carries no example body): the 204 this handler returns is invented, not captured."
- `staff_test.go:166` -- "staff/deleted.json is an invented success body, not a captured ..."

## G5 (was ADVISORY 5) -- FIXED

`ProjectsService.Get`'s doc comment now records the divergence directly: plural path matches the collection's captured request and its real 200; the docs spell it singular; both appear to resolve; recorded "per the inferred-vs-confirmed convention rather than silently picking one." `Project`'s type comment was also corrected from "Get also has a sibling" to "Get and Update both have a sibling ... Both methods drop it", closing the second half of that finding.

## Unrequested extra, checked

The fix commit reports that the three new `[corner]` handlers originally tripped gosec G705 (formatting tainted query input into a response body) and were rewritten to marshal a typed struct instead, with no lint suppression added. Confirmed: `== lint: freshbooks == 0 issues.` and no new `//nolint` appears anywhere in the diff. Good instinct -- a suppression there would have been the easy wrong answer.

## Still open (deferred by agreement, phase-close checklist)

- ADVISORY 4 -- `PageMeta` drops `meta.sort` (shared code from batches a/b).
- ADVISORY 6 -- `StaffService.List` discards the seven sibling business fields; no accounting list-staff endpoint exists in the library.
- ADVISORY 7 -- business-family sort direction: `Sort()` emits `field_desc`, the API wants `-field`. Documented in spec 5.1 with a workaround; needs an owner.

Plus the process note above: the dirty-tree check makes a green gate and an uncommitted QA report mutually exclusive.

## Commands run this round

```
mise run check                                              # exit 0 on a fully clean tree (report stashed, then restored)
mise exec -- go test -run TestQAProbeAllPagePin -v .        # throwaway probe, deleted
git show 839dbf9 -- freshbooks/{projects,tasks,team_members}.go
git status --porcelain                                      # only docs/phases/2/reports/c-qa.md
```

Tree state on exit: clean except this report, as instructed.
