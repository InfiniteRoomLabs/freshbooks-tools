# Phase 2 batch d -- simplification lane

Branch `phase-2/d` (rebased onto main with a+b+c merged), diff `git diff main...phase-2/d`.
Scope: ledger_accounts, journal_entries, other_income, reports, callbacks, attachments, images,
gateways, payment_options, the transport additions, tests, fixtures.
**Propose only.** No files changed, no commits, no `mise run check` / test / build runs.

Headline: the batch is well built -- doc comments carry real API evidence, fixtures are lean, no
speculative types. The cuts below are mostly *convergence*: batch d was authored before b/c landed
the shared seams, so it hand-rolls things main already has. Net effect of applying everything
tagged APPLY-RECOMMENDED: roughly **-110 lines** in the resource files and transport, plus
**+~50 lines** of `pathSegment` guards that main's seam requires (net ~-60).

---

## APPLY-RECOMMENDED

### 1. `reports.go` -- collapse the 12 repeated accounting-report path/decode preambles

`freshbooks/reports.go:66,141,184,229,260,297,375,403,444,487,561,618`

The literal `"/accounting/account/" + string(acct) + "/reports/accounting/"` is spelled out 12
times, each followed by an identical `do(... FamilyAccounting, nil, &resp)`.

Before (x12):
```go
path := reportPath("/accounting/account/"+string(acct)+"/reports/accounting/trial_balance", opts.values())
var resp struct{ TrialBalance TrialBalanceReport `json:"trial_balance"` }
if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
	return nil, err
}
return &resp.TrialBalance, nil
```

After -- one private helper, each method keeps its own exported signature, option struct and
envelope key (parity + docs, per the work order):
```go
// get fetches one accounting report for acct into out.
func (s *ReportsService) get(ctx context.Context, acct AccountID, name string, q url.Values, out any) error {
	path, err := reportPath(acct, name, q)   // see #6 for the (string, error) form
	if err != nil {
		return err
	}
	return s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, out)
}

// ...
var resp struct{ TrialBalance TrialBalanceReport `json:"trial_balance"` }
if err := s.get(ctx, acct, "trial_balance", opts.values(), &resp); err != nil {
	return nil, err
}
return &resp.TrialBalance, nil
```
`reportPath` already no-ops on an empty `url.Values`, so the no-option reports pass `nil`.

Behaviour-preserving: same method, same path string, same family, same destination pointer, same
error. Purely moves shared prefix + call into one place. Risk: **low**. Saves ~30 lines and gives
#6 a single site to add the `pathSegment` guard.

### 2. `reports.go` -- one `setNonEmpty` for the four "if != \"\" { q.Set }" chains

`freshbooks/reports.go:160-175, 204-222, 508-526, 579-594`

`ItemSalesOptions.values` (L320) already uses the compact map form; four siblings did not get the
same treatment.

Before (`TrialBalanceOptions.values`, 16 lines; same shape at 160, 204, 508):
```go
q := url.Values{}
if o == nil { return q }
if o.StartDate != "" { q.Set("start_date", o.StartDate) }
if o.EndDate != "" { q.Set("end_date", o.EndDate) }
if o.CurrencyCode != "" { q.Set("currency_code", o.CurrencyCode) }
return q
```
After:
```go
// setNonEmpty adds every pair with a non-empty value to q.
func setNonEmpty(q url.Values, pairs map[string]string) {
	for k, v := range pairs {
		if v != "" {
			q.Set(k, v)
		}
	}
}

func (o *TrialBalanceOptions) values() url.Values {
	q := url.Values{}
	if o == nil {
		return q
	}
	setNonEmpty(q, map[string]string{
		"start_date": o.StartDate, "end_date": o.EndDate, "currency_code": o.CurrencyCode,
	})
	return q
}
```
`SalesTaxSummaryOptions` keeps its `CashBased` branch after the call; `BalanceSheetOptions` keeps
its `dates[]` loop.

Behaviour-preserving: `url.Values.Encode` sorts keys, so map iteration order is not observable, and
the exact same key/value set is written. Risk: **low**. Saves ~30 lines. It also folds
`ItemSalesOptions`' inline loop into the same helper.

### 3. `reports.go:528` -- `boolQuery` is `strconv.FormatBool`

```go
func boolQuery(b bool) string { if b { return "true" }; return "false" }   // delete
q.Set("cash_based", strconv.FormatBool(*o.CashBased))                     // call site
```
Identical output. Risk: **none**. -6 lines, one fewer package-local name.

### 4. `transport.go:88-94, 160-179` -- delete `doRaw` and `sendRaw`

Two single-caller adapters over already-shared machinery, added in the rebase merge.

- `doRaw` (L92) is one line: `return c.fetchRaw(ctx, method, path, fam, "")`. Its only caller is
  `Reports.DownloadCSV` (reports.go:248). Two doc comments now describe the same "raw body, no
  envelope" behaviour (L88-91 vs L181-186). Delete `doRaw`; call
  `s.client.fetchRaw(ctx, http.MethodGet, path, FamilyAccounting, "")` from `DownloadCSV`, and keep
  the merged explanation on `fetchRaw`.
- `sendRaw` (L175) exists only to widen `build func(string)` into `attemptLoop`'s
  `func(context.Context, string)` -- and it discards the ctx it is handed. Its only caller is
  `send`.

After:
```go
// send runs build through the retry loop and decodes the winning response
// into out. do, doOnHost, and doMultipart all funnel through it; only how
// the request body is built differs between them.
func (c *Client) send(ctx context.Context, fam Family, out any, build func(context.Context, string) (*http.Request, error)) error {
	raw, err := c.attemptLoop(ctx, fam, build)
	if err != nil {
		return err
	}
	return decodeBody(raw, fam, out)
}
```
and the three closures at L62, L83, L115 take `(ctx context.Context, authorization string)` and pass
that ctx to `c.newRequest` instead of closing over the outer one.

Behaviour-preserving: `attemptLoop` passes the same `ctx` it was given down to `newReq`, so the
closures receive the value they already closed over. Risk: **low** (mechanical; the existing
`transport_upload_test.go` retry/replay cases cover the path). -18 lines, one layer of indirection
gone, and after this the transport has exactly three entry shapes (`send` for decoded, `fetchRaw`
for raw, `attemptLoop` underneath) rather than five.

Also stale after this: `newRequest`'s comment at L382 cites "doRaw's GET" as the empty-contentType
case -- `doRaw` passes a nil payload, so that clause never applied even today. Retarget it to
`fetchRaw`.

### 5. `other_income.go:127-130` and `callbacks.go:97-100` -- use `newPage`

Both List methods hand-assemble the `Page[T]`; main converged on `newPage(items, meta)` (page.go:70)
and every batch b/c list uses it.
```go
- return &Page[OtherIncome]{
-	Items: resp.OtherIncome, Page: resp.Page, Pages: resp.Pages,
-	PerPage: resp.PerPage, Total: resp.Total,
- }, nil
+ return newPage(resp.OtherIncome, resp.PageMeta), nil
```
The response structs already embed `PageMeta`, so `resp.PageMeta` resolves. Field-for-field
identical. Risk: **none**. -6 lines, and these two stop drifting if `Page` ever gains a field.

### 6. Missing `pathSegment` guards on every caller-supplied path segment

This is the biggest convergence gap: main's accounting resources all route through a
`pathSegment`-guarded `(string, error)` builder (`clients.go:159`, `bills.go:136`,
`expenses.go:153`, ...). Batch d interpolates caller strings straight into paths at:

| file:line | unguarded segment |
|---|---|
| `reports.go:67,142,185,230,247,261,298,376,404,445,488,562,619` | `acct` (and `downloadToken` at 247) |
| `ledger_accounts.go:111,124,138,153,201` | `biz BusinessUUID`, `accountUUID`, sub-type `id` |
| `other_income.go:90` | `acct` |
| `callbacks.go:61` | `acct` |
| `gateways.go:65` | `acct` |
| `payment_options.go:151,172` | `acct` |
| `attachments.go:31`, `images.go:29` | `acct` |

`noTraversal` in `resolve` backstops `..` only -- a segment carrying `/`, `?`, or `#` still
reaches `url.Parse` and silently reshapes the request. Once the CLI and MCP server land, these
values arrive from flags and model-authored tool inputs. Convert each to main's shape:
```go
func reportPath(acct AccountID, name string, q url.Values) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	path := "/accounting/account/" + string(acct) + "/reports/accounting/" + name
	if len(q) == 0 {
		return path, nil
	}
	return path + "?" + q.Encode(), nil
}
```
Same for `otherIncomePath`, `callbacksPath`, and new `ledgerAccountsPath`/`ledgerAccountPath`
builders. `Reports.DownloadCSV` should guard `downloadToken` too (it is a server-issued token, but
it still comes back through the caller).

Note this is *additive* -- it introduces an error return on paths that previously could not fail,
so it is a behaviour change on malformed input (by design, matching main). Tag it a convergence fix
rather than a pure simplification; the security lane is likely flagging the same lines. Risk:
**low**, but it needs the four `if err != nil` call-site updates per builder. Ordering: land #1
first, then this touches one site in `reports.go` instead of twelve.

### 7. `familyForPath(path)` at nine call sites -> the constant it provably returns

`ledger_accounts.go:113,128,140,155,172,189,205`, `gateways.go:69`, `payment_options.go:157,181`

Every other resource file in the package passes a `Family` constant. These re-derive it at runtime
from a string-prefix switch:
```go
- if err := s.client.do(ctx, http.MethodGet, path, familyForPath(path), nil, &resp); err != nil {
+ if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
```
`familyForPath` returns `FamilyBusiness` for `/accounting/businesses/`, `/accounting/ledger_accounts/`
and `/payments/` (client.go:209-238), so the substitution is exact today -- and it stops these
methods from silently changing envelope handling if someone reorders the switch later. The
classification itself stays covered by `TestFamilyForPath`. Risk: **low**.

### 8. `reports.go:629-640` -- `TimeEntryAbility` duplicates `Ability`

`Ability{Name string; Value bool}` already exists on main (`projects.go`, `inventory: Settings/
Abilities/Abilities`) with the identical shape and identical JSON tags. Drop `TimeEntryAbility` and
type `TimeEntryAbilities.Abilities` as `[]Ability`. Wire-identical; removes one exported type from
the public surface before it ships. Risk: **low** (nothing outside this file references it).

---

## OPTIONAL

### 9. `transport.go:104,149-153` -- `doMultipart`'s `fields map[string]string` has no production caller

All three real callers (`attachments.go:40`, `images.go:34,50`) pass `nil`; only
`transport_upload_test.go:53` exercises it. Dropping the parameter and the `mw.WriteField` loop is
-8 lines of code plus one test subtest. Counter-argument for keeping it: multipart plain fields are
a real shape in the FreshBooks uploader family and a future `/uploads/` endpoint will likely need
one, and the cost is genuinely small. Lead's call; I lean **keep**, and flag it only so nobody has
to re-derive that it is currently dead.

### 10. `other_income.go:158` -- `Delete` vs the shared `client.softDelete`

`OtherIncomeService.Delete` re-implements the soft-delete-by-`vis_state` pattern through its own
`Update`, while `transport.go:261` has `softDelete(ctx, path, key)` for exactly this. But
`softDelete` returns only `error`, and this `Delete` returns the updated `*OtherIncome` -- routing
through it would drop the returned record. Two ways out, neither free:
- change `Delete` to `error` and match `softDelete` (loses information the API already sends back);
- leave as is.

The current code is short and clearly commented. I lean **leave as is**; flagging only because "why
doesn't this use `softDelete`" is a question a reviewer will ask twice.

### 11. `other_income.go:119` / `callbacks.go:88` -- `List(ctx, acct, opts ...RequestOption)` predates the b/c `List` shape

Batch b/c converged on `List(ctx, id, opts *XListOptions, extra ...RequestOption)` with an `opts()`
method delegating to `listOpts`, plus an `All` iterator (see `projects.go:128-166` on main). These
two Lists take a bare variadic and have no `All`. Converging them means adding
`OtherIncomeListOptions` / `CallbackListOptions` and an `All` per resource -- an **exported
signature change**, so it is not a behaviour-preserving simplification and I am not proposing it
under that banner. It is a real consistency gap the lead asked me to surface: two of the package's
list endpoints will look different from the other twenty. Cheap now (pre-1.0, unreleased),
expensive after `freshbooks/v0.1.0`. Risk of doing it: **medium** (new code, new tests).
`journal_entries.go:132,177` and `ledger_accounts.go:123` return bare slices with no pagination at
all, which is correct -- those endpoints return no `meta`.

### 12. `reports_test.go:81-122` -- the three "returns raw JSON" subtests table-drive cleanly

`BankReconciliationSummary`, `ClientAccountStatement` and `ExpenseDetails` each get a near-identical
subtest whose only real assertion is `len(raw) != 0`. A small table over
`{fixture, call func(*Client) (json.RawMessage, error)}` collapses ~30 lines to ~15. Low value --
the current version is readable and the query-string assertion in the first subtest is genuinely
specific to it. Take it only if the QA lane is already touching the file.

---

## DO-NOT-APPLY (considered and rejected -- do not re-derive)

13. **Route the report query parameters through `RequestOption` instead of `reportPath`.**
    `requestOptions` (`types.go:250-337`) models `include[]`, `search`, `sort`, `page`, `per_page`
    only. Reports need arbitrary bare params (`dates[]`, `sales_type`, `bank_accountid`), which
    would mean adding a public raw-parameter option -- new exported API surface to avoid a
    3-line helper. `reportPath` is the right size. Also confirmed the round trip is safe:
    `resolve` re-parses the query out of the path and re-encodes it, and `dates%5B%5D` survives.

14. **Merge `TrialBalanceOptions` / `SalesTaxSummaryOptions` / `ClientAccountStatementOptions` into
    one `DateRangeOptions`.** They are structural subsets of each other, but the work order (and
    docs/parity) require each report to keep its own exported option struct. After #2 each is four
    lines anyway.

15. **A generic `report[T]` that decodes the envelope key by name.** Tempting (would collapse all
    12 methods to one line each), but the envelope key does not always match the path segment
    (`profitloss_entity` -> `"profitloss"`, `taxsummary` -> `"taxsummary"`, `account_statement`),
    the return types split between `*T` and `json.RawMessage`, and reproducing "missing key decodes
    to the zero value, not an error" through a `map[string]json.RawMessage` is exactly the kind of
    subtle decode-path change this lane is told not to make. #1 gets most of the win for none of
    the risk.

16. **Name the three anonymous nested structs in `JournalEntryDetailEntry`
    (`journal_entries.go:76-102`).** It would add three exported types to the public surface to
    remove no duplication -- `JournalEntryAccountSub` overlaps but is not identical (it carries
    `AccountType`, `CurrencyCode`, `Balance`, `Custom`; the nested one does not). Inline anonymous
    structs are correct here.

17. **`Callback.UnmarshalJSON` (`callbacks.go:31-44`).** The alias-plus-`ID`-fallback dance looks
    like ceremony but is the standard Go idiom for accepting two spellings of one field, and the
    docs/Postman disagreement it handles is documented directly above it. Leave it. Its `[edge]`
    test at `callbacks_test.go:49` is the right coverage.

18. **Fixtures.** 34 new files, all small and each one the evidence behind a specific decode
    assertion; `ledger_accounts/sub_type.json` (3 lines) and `types.json` (3 lines) are the smallest
    honest fixtures for those endpoints. No sprawl, nothing to merge.

19. **Doc comments.** Long, but they carry API evidence (INFERRED vs CONFIRMED markers, the
    my.freshbooks.com rewrite note at `journal_entries.go:107`, the docs-vs-Postman disagreement at
    `reports.go:251`) -- exactly the kind the work order says to keep. The only comment I would cut
    is the stale `newRequest` clause in #4.

---

## Summary

| # | Tag | Where | Net lines |
|---|---|---|---|
| 1 | APPLY-RECOMMENDED | `reports.go` report get helper | -30 |
| 2 | APPLY-RECOMMENDED | `reports.go` `setNonEmpty` | -30 |
| 3 | APPLY-RECOMMENDED | `reports.go` `strconv.FormatBool` | -6 |
| 4 | APPLY-RECOMMENDED | `transport.go` drop `doRaw`/`sendRaw` | -18 |
| 5 | APPLY-RECOMMENDED | `other_income.go`, `callbacks.go` `newPage` | -6 |
| 6 | APPLY-RECOMMENDED | `pathSegment` guards (convergence + security) | +50 |
| 7 | APPLY-RECOMMENDED | `familyForPath` -> constant, 9 sites | 0 |
| 8 | APPLY-RECOMMENDED | drop `TimeEntryAbility` | -6 |
| 9-12 | OPTIONAL | dead `fields` param, `softDelete`, `List` shape, test table | -- |
| 13-19 | DO-NOT-APPLY | see above | -- |

Suggested order if all are applied: **3, 5, 8** (trivial, no call-site churn) -> **2** -> **1** ->
**6** (lands on the one path builder #1 created) -> **7** -> **4** (transport, isolated).
