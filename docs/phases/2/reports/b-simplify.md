# Phase 2 batch b -- simplification lane

Branch `phase-2/b` (rebased onto main, which carries batch a). Diff: `git diff main...phase-2/b`.
Scope: clients, contacts, credit_notes, estimates, expenses, expense_categories, taxes, bills, bill_payments, bill_vendors + tests + fixtures.
PROPOSE ONLY -- nothing edited, nothing committed, no `mise`/test/build run.

Headline: batch b was authored before batch a's gate landed `listOpts`, `newPage[T]`, and `(*Client).softDelete` on main. Three mechanical delegations remove ~95 lines of hand-rolled duplication with zero wire-format change. Everything else is small.

---

## APPLY-RECOMMENDED

### 1. Delegate the eight `requestOptions()` bodies to `listOpts` (and rename to `opts()`)

Sites: `bills.go:109`, `bill_vendors.go:96`, `credit_notes.go:118`, `expenses.go:124`, `expense_categories.go:68`, `taxes.go:71` (no `Include`); `estimates.go:172`, `clients.go:134` (with `Include`).

All eight are the identical 16-line body. Before (`taxes.go:71`):

```go
func (o *TaxListOptions) requestOptions() []RequestOption {
	if o == nil {
		return nil
	}
	var opts []RequestOption
	if o.Search != nil {
		opts = append(opts, o.Search)
	}
	if o.Page > 0 {
		opts = append(opts, PageNumber(o.Page))
	}
	if o.PerPage > 0 {
		opts = append(opts, PerPage(o.PerPage))
	}
	return opts
}
```

After:

```go
func (o *TaxListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}
```

And for the two with `Include` (`estimates.go:172`, `clients.go:134`):

```go
func (o *EstimateListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	opts := listOpts(o.Search, o.Page, o.PerPage)
	if len(o.Include) > 0 {
		opts = append(opts, Include(o.Include...))
	}
	return opts
}
```

Behaviour-preserving, and the one place it looks like it might not be is worth spelling out: b guards with `o.Search != nil`, `listOpts` guards with `len(search) > 0`. The only case they differ on is a non-nil empty `Search{}`. `Search` is `map[string]string` (`types.go:261`) and `Search.apply` (`types.go:263`) copies zero entries for an empty map, so `requestOptions.values` (`types.go:315`) iterates nothing and emits no query key either way. Identical wire output. Everything else -- append order, `PageNumber`/`PerPage` construction, the nil-receiver early return -- is copied verbatim.

The rename is part of the same edit, for two reasons: main's five existing list-options methods (`invoices.go:213`, `items.go:59`, `payments.go:64`, `retainers.go:63`, `invoice_profiles.go:127`) are all named `opts()`, and `requestOptions` is already the name of an unexported *struct type* in `types.go` -- a method returning `[]RequestOption` named after an unrelated type is a reader trap. Unexported method, callers are all in-file (`opts.requestOptions()...` at each `List`), so the rename is mechanical.

Risk: **low**. ~82 lines removed. Existing List/All tests (query-string assertions in `taxes_test.go:31`, `clients_test.go`, etc.) cover the result.

### 2. Replace the eight `Page[T]{...}` literals with `newPage`

Sites: `bills.go:142`, `bill_vendors.go:129`, `credit_notes.go:151`, `expenses.go:157`, `estimates.go:208`, `expense_categories.go:101`, `taxes.go:106`, `clients.go:170`.

Before (`clients.go:170`):

```go
return &Page[Customer]{Items: env.Clients, Page: env.Page, Pages: env.Pages, PerPage: env.PerPage, Total: env.Total}, nil
```

After:

```go
return newPage(env.Clients, env.PageMeta), nil
```

Every one of b's list envelopes already embeds `PageMeta` (`clients.go:96`, `taxes.go:40`, `bills.go:72`, `bill_vendors.go:62`, `credit_notes.go:89`, `estimates.go:114`, `expenses.go:79`, `expense_categories.go:49`), so `env.PageMeta` is available directly and the promoted `env.Page`/`env.Pages`/... the literal reads today are the same four fields. `newPage` (`page.go:57`) assigns exactly those four plus `Items`. Field-for-field identical struct.

Behaviour-preserving by construction. The value is drift resistance: a hand-copied five-field literal repeated eight times is where a future `PageMeta` field silently fails to propagate.

Risk: **low**. Net line count is flat; the win is that the copy exists once.

### 3. Delegate four soft-delete bodies to `(*Client).softDelete`

Sites: `bill_vendors.go:186`, `credit_notes.go:213`, `expenses.go:238`, `estimates.go:283`.

Before (`credit_notes.go:212`):

```go
func (s *CreditNotesService) Delete(ctx context.Context, acct AccountID, id int64) error {
	body := map[string]any{"credit_note": map[string]any{"vis_state": VisStateDeleted}}
	return s.client.do(ctx, http.MethodPut, creditNotePath(acct, id), FamilyAccounting, body, nil)
}
```

After:

```go
func (s *CreditNotesService) Delete(ctx context.Context, acct AccountID, id int64) error {
	return s.client.softDelete(ctx, creditNotePath(acct, id), "credit_note")
}
```

`softDelete` (`transport.go:146`) sends `map[string]any{key: map[string]int{"vis_state": int(VisStateDeleted)}}` via `PUT` on `FamilyAccounting` with `out == nil`. b sends `map[string]any{key: map[string]any{"vis_state": VisStateDeleted}}` -- `VisState` is an integer type, so `encoding/json` emits the same `1` for both; the map value type differs in Go and not on the wire. Same method, same family, same nil out. Wire-identical.

Keep every explanatory comment as-is. The `expenses.go:226-234` comment (Postman says vis_state 0, docs and every sibling say 1, treated as a Postman authoring mistake) and the `estimates.go:273-280` comment (docs say DELETE, Postman concretely sends PUT) are API evidence, not restatement -- they are exactly the kind of comment the work order says to keep. They stay above the one-line body.

The existing tests keep covering this: `credit_notes_test.go:146`, `estimates_test.go:164`, `expenses_test.go:167`, and `bill_vendors_test.go:112` all decode the request body and assert the method and `vis_state`, so they verify the delegation preserves the payload rather than merely compiling.

Risk: **low**. 4 sites, 2 lines to 1 each.

---

## OPTIONAL

### 4. Name the `perPage := 100` default once

Eight `All` methods (`bills.go:147`, `bill_vendors.go:134`, `credit_notes.go:156`, `expenses.go:162`, `estimates.go:213`, `expense_categories.go:106`, `taxes.go:111`, `clients.go:175`) each open with a bare `perPage := 100`. The number is undocumented and repeated eight times; if the API's max page size turns out to be different, that is eight edits and one of them gets missed.

Add next to `listOpts` in `page.go`:

```go
// defaultPerPage is the page size All uses when the caller did not pick
// one: large enough to keep the round-trip count down, small enough to
// stay inside the accounting family's page-size ceiling.
const defaultPerPage = 100

// pageSize resolves a caller's page size against the default.
func pageSize(perPage int) int {
	if perPage > 0 {
		return perPage
	}
	return defaultPerPage
}
```

and at each site collapse

```go
	perPage := 100
	var search Search
	if opts != nil {
		if opts.PerPage > 0 {
			perPage = opts.PerPage
		}
		search = opts.Search
	}
```

to

```go
	var search Search
	var perPage int
	if opts != nil {
		search, perPage = opts.Search, opts.PerPage
	}
	perPage = pageSize(perPage)
```

Behaviour-preserving: same resolved value for nil opts, zero `PerPage`, and positive `PerPage`. Marked optional rather than recommended because the saving is small (about 8 lines) and it touches all eight `All` bodies -- reasonable to fold in with items 1-3 if the implementer is in those files anyway, reasonable to skip.

Risk: **low**.

### 5. A shared request-capture helper for the delete tests (defer -- not b-only)

`bills_test.go:126`, `credit_notes_test.go:129`, `estimates_test.go:147`, `expenses_test.go:155` each repeat:

```go
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			...
		}))
```

A `captureRequest(t, next http.Handler) (http.HandlerFunc, *string, *map[string]any)` in `client_test.go` next to `newTestClient`/`serveFixture` would collapse it.

Flagged OPTIONAL and recommended **deferred**, not applied in this gate: the same idiom already exists on main in `invoices_test.go:229`, `items_test.go:191`, `payments_test.go:181`, and `invoice_profiles_test.go:173`. Doing it b-only creates two idioms where there is currently one; doing it properly means editing four main-side test files inside a batch-b fix commit, which is scope the batch does not own. Better as a one-line package-wide sweep in a later phase.

Risk: **low** if done package-wide, **medium** (as inconsistency, not breakage) if done b-only.

---

## DO-NOT-APPLY

Considered and rejected -- recorded so the lead does not re-derive them.

### 6. The duplicated `boolPtr`/`strPtr` test helper -- checked, does not exist

The priority check asked for this specifically. `grep -rn "func boolPtr\|func strPtr\|func intPtr\|func ptr\[" freshbooks/*_test.go` returns exactly one hit across the whole package: `invoice_profiles_test.go:223`, which is on main (batch a), not in b's diff. Batch b adds no pointer helper and no shadowing redefinition. Nothing to collapse. Clean.

### 7. Do not route `BillsService.Delete` through `softDelete`

`bills.go:179` has its own `visStatePut(ctx, acct, id, state) (*Bill, error)`, used by `Archive` (vis_state 2, returns the updated bill -- `bills.go:193`) and `Delete` (vis_state 1, discards it -- `bills.go:201`). Two reasons to leave it:

- `softDelete` hard-codes `VisStateDeleted`, so `Archive` cannot use it. Converting only `Delete` would leave `visStatePut` alive for one caller and split one concept across two mechanisms in the same file.
- `Delete` currently passes `&env` as `out`, so a malformed response body surfaces as a decode error; `softDelete` passes `nil`, which discards it. That is a small observable behaviour change, and the work order says behaviour-preserving only.

Leave `bills.go` as written. It is already the DRY shape for its own file.

### 8. Do not collapse the eight `All` bodies into one generic helper

Tempting -- they are eight near-identical bodies. But each closes over a *different* concrete options struct (`BillListOptions`, `TaxListOptions`, ... two of which carry `Include`), so a generic helper needs either a second type parameter plus a constructor callback per resource, or an interface every options struct implements. Both are longer and harder to read than the duplication they replace. `All` (`page.go:63`) already carries the part that actually generalises -- the pagination loop. Item 4 removes the only genuinely shared constant. Stop there.

### 9. b's `All` page-size default and missing `extra ...RequestOption` -- behaviour/API, not simplification

Two divergences from main's exemplar, both out of this lane:

- Main's `InvoicesService.All` (`invoices.go:238`) and `ItemsService.All` (`items.go`) pass `opts.PerPage` straight through with no default; b's eight `All` methods default it to 100, which puts `per_page=100` on the wire where main puts nothing.
- Main's `List`/`All` take a trailing `extra ...RequestOption`; b's do not.

Reconciling either changes the wire query or the exported signature. Flagging for the lead as a phase-level consistency call (b's defaulting is arguably the better behaviour -- fewer round trips), not proposing an edit.

### 10. b's path builders do not validate the account segment -- routed to the code-review and security lanes

Every batch-a path builder returns `(string, error)` and calls `pathSegment` first (`invoices.go`: `invoicesPath`/`invoicePath`). All eighteen of b's builders return a bare `string` with no validation: `clients.go:154,158`, `contacts.go:45`, `credit_notes.go:135,139`, `estimates.go:192,196`, `expenses.go:141,145`, `expense_categories.go:85,89`, `taxes.go:88,92`, `bills.go:126,130`, `bill_payments.go:60,64`, `bill_vendors.go:113,117`.

`noTraversal` in `resolve` (`transport.go:167`) is a backstop for `.`/`..` but does not catch a `/`, `?`, or `#` in an `AccountID`. Noting it here only so it is not lost between lanes -- it is an *addition* of validation and a security question, the opposite of a simplification, so this lane proposes nothing. Code-review and security own it.

---

## Deliberately left alone

- **Doc comments on the resource types.** `clients.go:19-27` (why `Customer` rather than `Client`), `contacts.go:9-14` (why `ContactsService` exists next to `RemoveAllSecondaryContacts`), the p_/s_ address-prefix note at `clients.go:57`, `taxes.go:177` (taxes really do use a hard DELETE), `expense_categories.go` on the docs contradiction, `estimates.go:119` on the INFERRED presentation block. Every one carries API evidence a reader cannot recover from the code. Keep.
- **The anonymous `body := struct { X *XRequest \`json:"x"\` }{req}` literals** (17 sites across b). A generic `writeBody[T](key string, v T)` would need `map[string]any` and lose the compile-time key/field pairing; main uses named envelope types (`invoiceCreateEnvelope`) in some places and anonymous structs in others, so there is no single exemplar to converge on. Not worth churning.
- **The `if req == nil` guards** on every Create/Update. Trust-boundary input validation on an exported API -- explicitly out of bounds for this lane.
- **Fixtures.** Checked the plausible duplicate pairs: `bills_archive`/`bills_delete` differ on vis_state 2 vs 1, `bill_vendors_create`/`bill_vendors_delete` on vis_state and updated_at, `taxes_create`/`taxes_get`/`taxes_update`, `expense_categories_create`/`_get`, `credit_notes_create`/`_update`, `bill_payments_create`/`_update` all differ on the fields their tests assert. No sprawl; each fixture earns its file.
- **Test harness.** b's tests use main's `newTestClient` and `serveFixture` (`client_test.go:31,51`) throughout -- no reinvented harness, no per-file server boilerplate.
- **`[happy]/[sad]/[edge]` tagging.** Present and consistent across b's test files.

---

## Suggested fix order

Items 1-3 are independent and touch mostly disjoint lines; 4 overlaps item 1's files. Apply 1, 2, 3, then 4 if the lead wants it, in one fix commit. Items 5 and 9 are follow-ups for a later phase; item 10 belongs to the other lanes' triage.
