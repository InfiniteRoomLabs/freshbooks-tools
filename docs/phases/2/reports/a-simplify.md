# Phase 2 batch a -- simplification lane

Branch `phase-2/a`, worktree `.worktrees/a`, diff `git diff main...phase-2/a` (23 files, +3136/-51).
Scope reviewed: `freshbooks/{invoices,invoice_profiles,items,payments,retainers}.go` + their `_test.go` + `freshbooks/testdata/*`.
**Propose only.** No file was modified, nothing committed, no `mise`/test/build run.

## Verdict

The batch is in good shape. It reuses the Phase 1 seams correctly -- `client.do`, `RequestOption`, `Page[T]`/`PageMeta`, `newTestClient`/`serveFixture` -- and never hand-rolls HTTP. The tests are table-ish, tagged, and share the existing fixtures helper rather than growing their own. There is no over-abstraction to tear out.

What there IS: five hand-copied instances of the same three list-plumbing shapes. That matters less for the ~55 lines it costs here and more because **batches b, c, and d are writing the same five shapes right now**, and the remaining ~30 resource files will copy them again. Proposals 1-3 are worth applying as a cross-batch convergence item at merge time, not as a batch-a-only cleanup.

Findings 1-3 are APPLY-RECOMMENDED, 4 is a dead field, 5-8 OPTIONAL, 9-13 DO-NOT-APPLY (considered and rejected, or out of this lane).

---

## APPLY-RECOMMENDED

### 1. Five byte-identical `opts()` bodies -> one free function

`freshbooks/invoices.go:186-201`, `invoice_profiles.go:308-323`, `items.go:56-71`, `payments.go:237-252`, `retainers.go:56-71`.

All five are the same 16 lines modulo the receiver type (invoices additionally copies a `Sort` field that is never read -- see finding 4).

Before (x5):

```go
func (o *ItemListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	var opts []RequestOption
	if len(o.Search) > 0 {
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

After -- one helper in `page.go` beside `Page`/`PageMeta`:

```go
// listOpts is the option plumbing every resource's list-options struct
// shares: a filter, a page number, and a page size, each omitted when unset.
func listOpts(search Search, page, perPage int) []RequestOption {
	var opts []RequestOption
	if len(search) > 0 {
		opts = append(opts, search)
	}
	if page > 0 {
		opts = append(opts, PageNumber(page))
	}
	if perPage > 0 {
		opts = append(opts, PerPage(perPage))
	}
	return opts
}
```

and each resource keeps its own struct and method, now four lines:

```go
func (o *ItemListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}
```

Behaviour-preserving: the option values, their order of appending, and the nil-receiver short-circuit are unchanged; `newRequestOptions` folds them the same way. **The exported API is untouched** -- the five `*ListOptions` structs keep their own names and their own fields, so `&ItemListOptions{Search: ...}` composite literals still compile. (I deliberately did NOT propose embedding a shared `ListOptions` struct: that would break every composite literal, which is an API change and out of bounds for this lane. See finding 11.)

Net: 80 lines -> 28. Risk: **very low** (mechanical; the existing per-resource list tests cover the query encoding).

### 2. The `Page[T]` construction literal, repeated five times -> `newPage`

`invoices.go:212`, `invoice_profiles.go:334`, `items.go:87`, `payments.go:263`, `retainers.go:85`.

Before (x4, accounting family):

```go
return &Page[Invoice]{Items: resp.Invoices, Page: resp.Page, Pages: resp.Pages, PerPage: resp.PerPage, Total: resp.Total}, nil
```

plus the retainers variant that reads from `resp.Meta` instead of the embedded `PageMeta`.

After -- in `page.go`:

```go
// newPage assembles a Page from a decoded list response's items and its
// pagination block, whichever family the block came from.
func newPage[T any](items []T, m PageMeta) *Page[T] {
	return &Page[T]{Items: items, Page: m.Page, Pages: m.Pages, PerPage: m.PerPage, Total: m.Total}
}
```

Call sites become `return newPage(resp.Invoices, resp.PageMeta), nil` and, for retainers, `return newPage(resp.Retainers, resp.Meta), nil` -- which also makes the accounting/business pagination difference read as one visible argument instead of a diff you have to spot inside a 130-character literal.

Behaviour-preserving: same five fields, same values, same order of evaluation. Risk: **very low**.

### 3. The accounting soft-delete body, repeated four times -> one transport helper

`invoices.go:315-317`, `invoice_profiles.go:425-427`, `items.go:168-170`, `payments.go:337-339`.

Each is a one-line `PUT` whose body is the same `vis_state` map wrapped in a different resource key:

```go
func (s *ItemsService) Delete(ctx context.Context, acct AccountID, id int64) error {
	return s.client.do(ctx, http.MethodPut, itemPath(acct, id), FamilyAccounting, itemWriteEnvelope{Item: map[string]int{"vis_state": int(VisStateDeleted)}}, nil)
}
```

After -- in `transport.go`:

```go
// softDelete performs the accounting family's delete, which is not a DELETE:
// a PUT that sets vis_state on the resource, wrapped in that resource's own
// body key. key is the singular JSON key, e.g. "invoice".
func (c *Client) softDelete(ctx context.Context, path, key string) error {
	body := map[string]any{key: map[string]int{"vis_state": int(VisStateDeleted)}}
	return c.do(ctx, http.MethodPut, path, FamilyAccounting, body, nil)
}
```

Call sites: `return s.client.softDelete(ctx, itemPath(acct, id), "item")`.

Behaviour-preserving on the wire: `encoding/json` emits `{"item":{"vis_state":1}}` for both the single-field struct and the single-key map -- byte-identical, and the four existing delete tests already prove it, because each one decodes the request body into `map[string]any` and asserts `gotBody["<key>"]["vis_state"] == 1` (`invoices_test.go:224`, `items_test.go:186`, and the two siblings). Those tests pass unchanged; that is exactly the check that would fail if the encoding drifted.

Also worth stating in one place: this helper is where the "FreshBooks has no hard delete for accounting resources" fact belongs, instead of being restated in four doc comments.

Risk: **low**. Note it does NOT cover `Retainers.Delete`/`Undelete` (business family, `active` bool, returns the retainer) -- correctly left alone, see finding 12.

---

## Dead code

### 4. `InvoiceListOptions.Sort` is written but never read -- delete the field

`freshbooks/invoices.go:181` declares `Sort string`; `invoices.go:220` faithfully copies it in `All`; **`opts()` at 186-201 never converts it to an option.** Setting `InvoiceListOptions{Sort: "date"}` silently does nothing. It is the only one of the five list-option structs with the field, so it is not even a consistency argument.

Two ways out, and the choice is the lead's, not mine:

- **This lane's proposal (behaviour-preserving): delete the field** and the `o.Sort` copy at line 220. Nothing reads it, no test sets it, no wire bytes change. Callers who want sorting already have the exported `Sort(field, dir)` `RequestOption` and can pass it through the `extra ...RequestOption` variadic that every `List` already accepts -- which is arguably the better spelling anyway, since `Sort` needs a direction and the struct field has nowhere to put one.
- **Wiring it up instead** (`opts = append(opts, Sort(o.Sort, SortAsc))`) would add a query parameter that is not sent today. That is a behaviour change and therefore **not** a simplification -- flagging it to the code-review lane rather than proposing it here.

Risk of the deletion: **very low**, but it is an exported field removal, so it wants to land in the same fix commit as the rest and be named in the CHANGELOG entry (which currently does not mention `Sort`).

---

## OPTIONAL

### 5. `(*Client).fetchRaw` lives in `invoices.go`

`invoices.go:442-467`. It is a `*Client` transport method -- resolve, authorize, build, round-trip -- sitting at the bottom of a resource file because invoices happened to be the first caller. `transport.go` is its home; batch d's `/uploads/` endpoints will want it too and will not think to look here. Pure file move, no signature change. Risk: none.

### 6. The public CHANGELOG documents an unexported helper

`freshbooks/CHANGELOG.md`, second bullet of the batch-a block: "`(*Client).fetchRaw`, an unexported transport helper...". Consumers cannot call it and it is not part of the module's surface. Cut the bullet (or fold the fact into the `InvoicesService.PDF` mention, where it is context rather than an entry). Risk: none.

### 7. Five single-field write-envelope types could be one function

`invoiceCreateEnvelope` (`invoices.go:276`), `itemWriteEnvelope` (`items.go:39`), `paymentWriteEnvelope` (`payments.go:221`), `invoiceProfileWriteEnvelope` (`invoice_profiles.go:292`), `retainerWriteEnvelope` (`retainers.go:37`) are all `struct { X any \`json:"x"\` }`. One `func writeBody(key string, v any) map[string]any { return map[string]any{key: v} }` replaces all five (same wire bytes, same argument as finding 3).

I am tagging this OPTIONAL rather than recommended: it deletes five declarations, but `itemWriteEnvelope{Item: req}` names the key at the call site through the type system, and `writeBody("item", req)` names it with a string literal that nothing checks. That is a real, if small, loss. **If finding 3 is applied, the marginal value here drops further** (the delete sites, the most repetitive ones, are already gone). My inclination: apply 3, skip 7.

### 8. `InvoicesService.InvoicePresentationDefaults` stutters

`invoices.go:423`. Reads as `client.Invoices.InvoicePresentationDefaults(...)`. `PresentationDefaults` says the same thing. Flagged because the work order's "look for" list names inconsistent naming -- but it collides with this lane's hard constraint against exported-signature changes. Nothing outside the module calls it, and the module is unreleased, so the cost is a CHANGELOG line. **Lead's call**, and a legitimate "leave it" if the answer is that batch b/c/d are naming against the same pattern.

---

## DO-NOT-APPLY (considered and rejected -- do not re-derive)

### 9. Collapsing the five `All` wrappers

`invoices.go:216`, `invoice_profiles.go:338`, `items.go:91`, `payments.go:267`, `retainers.go` (none -- retainers has no `All`, correctly, since its List is unpaginated). The four that exist are structurally identical: rebuild the options struct with `Page` set, delegate to `List`. They cannot share an implementation without a common interface over the option types, and the only clean route to that is embedding a shared struct -- which breaks composite literals (finding 11). A generic helper parameterized on a constructor function would cost more lines than the four wrappers do. **Leave them.**

### 10. Table-driving the tests further

`invoices_test.go` (401 lines, 11 test funcs) is the largest, and its per-method `t.Run` blocks look repetitive at a glance. They are not: each asserts a different path, a different body shape, and a different fixture, and the shared parts are already factored into `newTestClient`/`serveFixture` from `client_test.go`. Forcing them into one table would replace readable assertions with a struct of optional fields. The one real duplicate, `boolPtr` at `invoice_profiles_test.go:211`, has exactly one caller (`:102`) and no sibling definition elsewhere in the package -- nothing to merge. **Leave them.** (If batches b/c/d each define their own `boolPtr`/`strPtr`, the merge will need one shared set; worth watching at gate time, not fixable from inside batch a.)

### 11. Embedding a shared `ListOptions` struct in the five option types

The obvious version of finding 1 -- `type ItemListOptions struct { ListOptions }` -- breaks every `&ItemListOptions{Search: ...}` composite literal in the package and in every future caller. That is an exported API change, explicitly out of bounds. The free-function version in finding 1 gets the same deduplication with zero API impact. **Rejected in favour of 1.**

### 12. Merging `Retainers.Delete` and `Retainers.Undelete`

`retainers.go:161-180`. They differ only in a bool. A shared `setActive(ctx, businessID, id, active bool)` would save about six lines, but both are exported, both carry distinct inventory keys, and the doc comment on `Delete` (explaining that FreshBooks has no hard delete here and that this mirrors `vis_state`) is the kind of API evidence the work order says to keep. Not worth the indirection. **Leave them.**

### 13. Out of this lane -- forwarded, not proposed

These are behaviour or design questions, so this lane must not touch them. Listing them so the lead has them in one place and the other lanes are not the only path:

- **`CheckoutLink` create/update return the request on a nil response.** `payments.go:392-394` and `409-411`: `if resp.CheckoutLink == nil { return link, nil }` hands the caller back their own request object as if it were the server's answer, so a caller cannot distinguish "server echoed nothing" from "server confirmed". The doc comment at `payments.go:355-361` is honest that the response shape is uncaptured, but the fallback is a behaviour choice, not a simplification. **-> code-review lane.**
- **`RetainerCreateRequest`/`RetainerUpdateRequest` use value fields with `omitempty` where the work order asks for pointers on write structs** (`retainers.go:100-111`, `129-140`). `Fee: 0`, `ExcessRate: 0`, and `BudgetedTime: 0` cannot be sent; `Active bool` (no `omitempty`) is always sent, so an update always writes it. Also `RetainerCreateRequest.ClientUserID` is `string` while `Retainer.ClientUserID` is `int64`. All three are wire-behaviour questions. **-> code-review lane.**
- **`Invoices.ShareLink` folds a query parameter into the path string** (`invoices.go:391`). I checked `transport.go:106-124`: `resolve` parses the path with `url.Parse` and merges `ref.Query()` with the option-derived values, so this is correct today and correct if options are ever added to the method. No action; recording the verification so nobody re-opens it.
- **`BusinessID` formatted with `%s`** (`retainers.go:183`, `:187`). Correct -- `types.go:24` gives `BusinessID` a `String()` method. No action; recorded for the same reason.

---

## Summary

| # | Proposal | Tag | Risk |
|---|---|---|---|
| 1 | `listOpts` free function replaces 5 identical `opts()` bodies | APPLY-RECOMMENDED | very low |
| 2 | `newPage[T]` replaces 5 copies of the `Page[T]` literal | APPLY-RECOMMENDED | very low |
| 3 | `(*Client).softDelete` replaces 4 copies of the `vis_state` PUT | APPLY-RECOMMENDED | low |
| 4 | Delete the dead `InvoiceListOptions.Sort` field | APPLY-RECOMMENDED | very low |
| 5 | Move `fetchRaw` from `invoices.go` to `transport.go` | OPTIONAL | none |
| 6 | Drop the unexported `fetchRaw` bullet from the CHANGELOG | OPTIONAL | none |
| 7 | One `writeBody` function replaces 5 write-envelope types | OPTIONAL | low |
| 8 | Rename `InvoicePresentationDefaults` -> `PresentationDefaults` | OPTIONAL | API rename |
| 9-12 | `All` wrappers, test tables, struct embedding, retainer Delete/Undelete | DO-NOT-APPLY | -- |
| 13 | Four items forwarded to the code-review lane | DO-NOT-APPLY (out of lane) | -- |

If only one thing is applied, apply 1 -- and apply it as a shared-seam decision across batches a-d before they merge, so the remaining ~30 resource files inherit the four-line version instead of the sixteen-line one.
