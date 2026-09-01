# Phase 2 batch c -- simplification lane

Branch: `phase-2/c` (rebased onto main carrying merged batches a + b). Diff: `git diff main...phase-2/c`.
Scope: `freshbooks/{projects,tasks,time_entries,services_svc,service_rates,team_members,staff,systems,settings}.go` + tests + fixtures. Spec sections 3, 5.1. Work order `docs/phases/2/plan-c.md`.
Mode: **propose only** -- no files edited, no commits, no `mise`/test/build runs.

## Headline

Batch c was written against the pre-convergence shape of the library and merged-in main has since converged the shared seams. Every list endpoint, every accounting soft-delete, and every `AccountID`-interpolating path in this batch is hand-rolled against helpers that now exist on main and that all thirteen main-side resources already delegate to. The batch is otherwise clean: the doc comments carry real API evidence (INFERRED/CONFIRMED provenance, the my.freshbooks.com rewrites, the two-services split), the tests follow main's `newTestClient`/`serveFixture` helpers, and there is very little dead weight to cut. Almost all of the value here is convergence, not deletion.

Counts against the seams named in the work order:

| Seam on main | Main-side users | Batch c uses it | Batch c hand-rolls it |
|---|---|---|---|
| `newPage[T](items, PageMeta)` | 13 | 0 | 6 |
| `listOpts(search, page, perPage)` via `XListOptions.opts()` | 13 | 0 | 5 |
| `pageSize` / `defaultPerPage` in `All` | 8 | 0 | 3 |
| `(*Client).softDelete(ctx, path, key)` | 8 | 0 | 2 |
| `pathSegment` + `xPath()` helpers | 10 files | 0 | 12 sites (+2 raw caller strings) |

---

## APPLY-RECOMMENDED

### 1. Six hand-rolled `Page[T]` literals -> `newPage`

`freshbooks/projects.go:108-114`, `freshbooks/tasks.go:81-87`, `freshbooks/time_entries.go:70-76`, `freshbooks/time_entries.go:93-99`, `freshbooks/services_svc.go:54-60`, `freshbooks/team_members.go:59-65`.

Two of the list-response structs need a `PageMeta` first:

`freshbooks/tasks.go:64-70` spells the accounting family's four pagination fields loose instead of embedding `PageMeta`:

```go
// before
type tasksListResponse struct {
	Page    int    `json:"page"`
	Pages   int    `json:"pages"`
	PerPage int    `json:"per_page"`
	Total   int    `json:"total"`
	Tasks   []Task `json:"tasks"`
}

// after
type tasksListResponse struct {
	Tasks []Task `json:"tasks"`
	PageMeta
}
```

`freshbooks/time_entries.go:41-49` declares an anonymous struct with the same four fields in a different order:

```go
// before
type timeEntriesListResponse struct {
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		Pages   int `json:"pages"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
	TimeEntries []TimeEntry `json:"time_entries"`
}

// after
type timeEntriesListResponse struct {
	Meta        PageMeta    `json:"meta"`
	TimeEntries []TimeEntry `json:"time_entries"`
}
```

Then every construction site collapses:

```go
// before (x6)
return &Page[Task]{Items: resp.Tasks, Page: resp.Page, Pages: resp.Pages, PerPage: resp.PerPage, Total: resp.Total}, nil
// after
return newPage(resp.Tasks, resp.PageMeta), nil
```

**Behaviour-preserving:** `PageMeta`'s json tags are byte-identical to the loose/anonymous fields (`page`, `pages`, `per_page`, `total`); `newPage` assigns the same five fields in the same order. Embedding `PageMeta` promotes its fields to the accounting envelope's top level, which is exactly where `tasksListResponse` reads them today (`clients.go:96-99` on main is the same shape). Nothing in the query, URL, body, or error path moves.
**Risk:** very low. Compile-checked; the existing `page.Total`/`len(page.Items)` assertions in the tests cover it.

### 2. Five list methods hand-roll option plumbing instead of an `XListOptions` + `opts()`

`freshbooks/projects.go:102` (`List`), `freshbooks/tasks.go:75`, `freshbooks/time_entries.go:64` and `:86`, `freshbooks/services_svc.go:48`, `freshbooks/team_members.go:53`.

Every one takes bare `opts ...RequestOption`. Main's converged shape, on all thirteen resources, is `(ctx, id, opts *XListOptions, extra ...RequestOption)` with the struct's `opts()` delegating to `listOpts`:

```go
// after -- one per listing resource, e.g. projects.go
// ProjectListOptions filters and paginates List. The business family spells
// filters as bare field=value (spec 5.1's STATE AS OF callout, confirmed by
// this batch).
type ProjectListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *ProjectListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

func (s *ProjectsService) List(ctx context.Context, businessID BusinessID, opts *ProjectListOptions, extra ...RequestOption) (*Page[Project], error) {
	var resp projectsListResponse
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, projectsPath(businessID), FamilyBusiness, nil, &resp, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(resp.Projects, resp.Meta), nil
}
```

**Wire-preserving, signature-changing.** `listOpts` emits exactly `[Search, PageNumber(page), PerPage(perPage)]`, the same three options a caller passes variadically today, folded by the same `newRequestOptions` into the same `url.Values`. The query string, ordering, and family encoding are unchanged for equivalent inputs. What does change is the exported Go signature, which the lane template normally forbids.

I am proposing it anyway, and flagging the tension explicitly for the lead: (a) the work order's PRIORITY CHECK names option plumbing as a target; (b) none of this API has ever been released -- `freshbooks/v0` is unpublished and `mcp/internal/tools/` holds only `doc.go` while `cli/internal/cmd/` holds only `root.go`, so there are **zero** callers outside this branch; (c) the alternative is shipping two permanently divergent list conventions in one package, which costs more to unwind after a tag than it does now. If the lead would rather not touch signatures in a review gate, item 1 still stands on its own and this becomes a tracked follow-up -- but it gets strictly more expensive every batch.

**Cost:** 19 call sites across the five batch-c test files (`tasks_test.go` 3, `services_svc_test.go` 3, `time_entries_test.go` 6, `projects_test.go` 4, `team_members_test.go` 3). Mechanical: `List(ctx, id)` -> `List(ctx, id, nil)`, and `List(ctx, id, Search{"active": "true"})` -> `List(ctx, id, &ProjectListOptions{Search: Search{"active": "true"}})`.
**Risk:** low-medium. Purely mechanical, but it is the largest single diff in this report.

### 3. Three `All` iterators prepend `PageNumber` ahead of the caller's options and skip `pageSize`

`freshbooks/projects.go:118-122`, `freshbooks/tasks.go:91-95`, `freshbooks/team_members.go:69-73`. All three are:

```go
// before
return All(ctx, func(ctx context.Context, page int) (*Page[Project], error) {
	return s.List(ctx, businessID, append([]RequestOption{PageNumber(page)}, opts...)...)
})

// after -- main's shape (clients.go:191-200, expense_categories.go:107-116)
return All(ctx, func(ctx context.Context, page int) (*Page[Project], error) {
	o := ProjectListOptions{Page: page}
	if opts != nil {
		o.Search, o.PerPage = opts.Search, opts.PerPage
	}
	o.PerPage = pageSize(o.PerPage)
	return s.List(ctx, businessID, &o, extra...)
})
```

Two things this fixes, one of which is not cosmetic:

- **`defaultPerPage` is never applied.** Main's eight `All` implementations all call `pageSize(o.PerPage)` so an unspecified page size becomes 100 rather than the server default. Batch c's three walk at whatever the server picks, which means more round trips for the same data. Behaviour-visible (request count and `per_page`), so this one is a deliberate convergence, not a no-op refactor -- call it out in the fix commit.
- **A caller-supplied `PageNumber` silently defeats the iterator.** `newRequestOptions` (`types.go:303-311`) folds last-wins, and `PageNumber(page)` is prepended *before* `opts...`. So `All(ctx, id, PageNumber(3))` refetches page 3 on every iteration instead of walking. Constructing the options struct makes that unrepresentable. This is a correctness finding rather than a simplification -- **the code-review lane owns the verdict**; I raise it only because the delegation is what removes it.

**Risk:** low for the shape change; the `pageSize` and page-pinning behaviour deltas are intentional and want naming in the commit message.

### 4. Two accounting-family vis_state deletes -> `(*Client).softDelete`

`freshbooks/tasks.go:126-130` and `freshbooks/staff.go:135-139`:

```go
// before
body := map[string]map[string]VisState{"task": {"vis_state": VisStateDeleted}}
return s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, nil)

// after
return s.client.softDelete(ctx, taskPath(accountID, taskID), "task")
```

**Behaviour-preserving, byte-for-byte.** `VisState` is `type VisState int` (`types.go:208`) with no `MarshalJSON`, so `{"task":{"vis_state":1}}` is what both spellings encode. `softDelete` (`transport.go:146-149`) issues the same `PUT`, same `FamilyAccounting`, same nil `out`. Eight resources on main already route through it (`invoices.go:353`, `expenses.go:275`, `items.go:182`, ...).
**Risk:** very low. The existing `[happy] PUTs vis_state 1` subtests in `tasks_test.go:147` and `staff_test.go:136` assert the body and will catch any drift.

Per the work order's warning I checked the other three deletes and left them alone -- see DO-NOT-APPLY #10.

### 5. Twelve `AccountID` interpolations with no `pathSegment` validation, and two raw caller strings

`AccountID` is `type AccountID string` (`types.go:16`). Main validates it through `pathSegment` in every `*Path` builder before interpolation (`clients.go:159`, `expense_categories.go:76`, `taxes.go:81`, ten files in total). Batch c interpolates it raw in twelve places:

- `tasks.go:44, 57, 114, 127`
- `services_svc.go:107, 122`
- `staff.go:98, 122, 136`
- `systems.go:44`
- `settings.go:81, 112`

and interpolates two unvalidated caller-supplied strings straight into a path:

- `team_members.go:80` -- `teamMemberUUID string`
- `settings.go:195` -- `clientID string`

`BusinessID` is `int64` (`types.go:21`) so every business-family path in this batch is already safe; nothing to do there, and no `error` return needs adding to those helpers (`retainers.go:137-143` on main is the precedent for the non-erroring business-family builder).

The simplification and the hardening are the same edit -- extract the `*Path` helpers main already has, which also removes the repeated `fmt.Sprintf` template that appears three times in `tasks.go` alone and three times in `staff.go`:

```go
// after -- tasks.go
func tasksPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/projects/tasks", acct), nil
}

func taskPath(acct AccountID, id int64) (string, error) {
	base, err := tasksPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
}
```

**Behaviour-preserving for every valid input;** it adds a typed error for inputs that are currently malformed URLs or traversals. `noTraversal` (`transport.go:163`) already backstops `..` at `resolve`, so this is defence in depth plus the convention, not a hole -- but a `/` or `?` in an `AccountID` is not caught by `noTraversal` and does reshape the request today.
**Cross-lane:** the security lane will almost certainly raise the same two raw-string sites independently. Dedupe at triage.
**Risk:** low. Each method gains the `path, err := ...` two-line prologue main already uses.

---

## OPTIONAL

### 6. `TimeEntriesService.List` and `.Search` are the same body twice

`freshbooks/time_entries.go:64-77` and `:86-100` differ only in path and one prefixed option. After item 1 lands they are 4 lines each, so the shared private helper is close to a wash:

```go
func (s *TimeEntriesService) list(ctx context.Context, path string, opts []RequestOption) (*Page[TimeEntry], error) {
	var resp timeEntriesListResponse
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return newPage(resp.TimeEntries, resp.Meta), nil
}
```

Worth doing only if item 2 lands first (then it also carries the `opts.opts()` fold). Behaviour-preserving; risk very low. Take it or leave it.

### 7. `ProjectServiceRate` duplicates `ServiceRate` plus one field

`freshbooks/service_rates.go:52-57` vs `:10-14`. Embedding would remove three lines but changes the exported struct's shape (field promotion vs direct fields is source-compatible for literals with field names, not for positional literals). Low value, non-zero churn. Leave unless the lead wants it.

### 8. `projects/list.json` is dead

`freshbooks/testdata/projects/list.json` has zero references on `phase-2/c` and zero on `main` (batch c added `list_full.json` alongside it rather than extending it). **This is pre-existing on main** -- it landed in Phase 1's `f287f86` -- so it is outside batch c's diff and outside this gate. Noting it so it does not get re-derived; delete it in a docs/chore pass, not in this batch's fix commit.

---

## DO-NOT-APPLY (considered and rejected)

### 9. A generic `decodeKeyedSlice[T]` for the four single-key slice responses

`serviceRatesListResponse`, `invitationRatesResponse`, `teamMemberRatesResponse`, `abilitiesResponse` are all `struct{ X []T \`json:"key"\` }`. A `map[string][]T` generic would collapse them, but it trades four self-documenting, greppable named types for a stringly-typed key, and main uses named response structs universally. Four instances is under the bar. Rejected.

### 10. Forcing `softDelete` onto the remaining three deletes

Explicitly per the work order, and confirmed by reading each:

- `ProjectsService.Delete` (`projects.go:164-168`) -- business family, real `http.MethodDelete`, `/comments/business/...` root, body `{"project":{"vis_state":1}}`. `softDelete` hardcodes `PUT` + `FamilyAccounting`. Not a fit.
- `TimeEntriesService.Delete` (`time_entries.go:166-169`) -- a genuine `DELETE` with no body. Not a soft delete at all.
- `IdentityService.DeleteBusiness` / `DeleteBusinessSubscription` (`settings.go:64, 80`) -- auth family, real `DELETE`. Not a fit.

### 11. Aliasing `InvitationRate` to `ServiceRate`

`team_members.go:89-93` and `service_rates.go:10-14` are field-identical (`Rate string`, `ServiceID int64`, `BusinessID int64`). They are different endpoints on different resources that FreshBooks is free to diverge; main keeps a distinct type per resource even when shapes coincide. Collapsing them would also make one service's doc comment the other's. Rejected.

### 12. Table-driving the `[sad] a nil request` subtests

Eight files carry a near-identical three-line nil-request subtest. Table-driving across files needs a shared slice of closures with different signatures -- more machinery than the duplication costs, and it would break the one-file-per-resource test locality the batch and main both keep. Rejected.

### 13. Family-switching `Sort()` for the business family

The batch's own spec callout (`docs/superpowers/specs/2026-08-22-...md`, the new STATE AS OF 2026-09-01 block) documents that the business family wants `sort=-field`, not `sort=field_desc`, and deliberately did **not** change `Sort()` because it would flip behaviour for every existing call site including Phase 1's `types_test.go` assertions. That reasoning is correct and the deferral is the right call -- this is a design decision for whoever owns `types.go`/`options.go`, not a simplification. Leaving it alone; the callout is the right artifact.

### 14. Renaming `TimeEntriesService.Search`

The method name collides visually with the package-level `Search` option type it uses in its own body (`time_entries.go:89`). It compiles fine and it is the endpoint's actual name in the collection. Renaming would be a signature change for cosmetic gain. Rejected.

---

## Deliberately left alone

- **Doc comments.** They carry API evidence, not restatement: the INFERRED/CONFIRMED provenance markers, the my.freshbooks.com host-rewrite notes on `ProjectsService.Delete` and `DeleteBusinessSubscription`, the `BillableItem`-vs-`Service` split rationale, the `staffListResponse` double-nesting note, and `teamMembersListResponse`'s "meta is a sibling of response, not nested" observation. Keep every one.
- **`settings.go`'s placement of the Settings/Businesses and Settings/Developer methods on `IdentityService`.** The file's header comment already flags it for this gate. It is a judgement call, not a simplification -- the lead's or the code-review lane's call, and the reasoning given (no pre-declared service fits; `IdentityService` already owns `Membership`) is sound.
- **`Project.Group json.RawMessage`.** Left undecoded on purpose because the Postman examples truncate it. Correct.
- **Test structure.** `newTestClient` + `serveFixture` usage matches main; the `[happy]/[sad]/[edge]` tags are applied consistently; error-path fixtures are reused across files (`auth/error_401` and `accounting/error_429` from earlier phases) rather than duplicated. Nothing to cut.
- **`services_svc.go`'s filename.** Forced by the existing `services.go` service registry. Fine.
- **Missing `All` on `TimeEntriesService` and `ServicesService`** despite both returning a `meta` block. That is a coverage gap, not excess -- code-review lane's call.

## Suggested fix order

If the lead takes items 1-5, apply them in this order so each step compiles on its own:

1. Item 1 (`PageMeta` + `newPage`) -- self-contained, no signature churn.
2. Item 5 (`*Path` helpers + `pathSegment`) -- self-contained, independent of the option work.
3. Item 4 (`softDelete`) -- depends on item 5 for the path helpers.
4. Item 2 (`XListOptions` + `opts()`) -- the signature change; touches 19 test call sites.
5. Item 3 (`All` via the options struct + `pageSize`) -- depends on item 2.

Items 6-8 are opportunistic; 6 only after 2.
