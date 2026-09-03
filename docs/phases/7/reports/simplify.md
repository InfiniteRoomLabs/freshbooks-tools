# Phase 7 simplification lane -- `phase-7/live` (2026-09-03)

Scope: `git diff main...phase-7/live` (13 commits `9c76dfc..ad1c1b4`). Propose-only; nothing run, nothing modified. Paths are relative to `<repo root>`.

Verdict up front: the branch is lean. The lib fixes are the minimal shape corrections and the evidence comments earn their length. Three small cuts are worth taking in the fix commit (S1-S3, all test-side or an unexported type); three more are optional; four were considered and rejected because they would change behaviour or break a repo convention. Nothing here touches JSON tags, `omitempty`, exported API, CLI output, or generated docs.

## APPLY-RECOMMENDED

### S1 -- one live-test setup helper instead of a four-line preamble in eight tests

`freshbooks/live_conformance_test.go:27-45` (`liveScope`, `liveCtx`) and the preamble repeated at `:52-55`, `:78-81`, `:128-131`, `:202-205`, `:241-244`, `:288-291`, `:324-327`, `:354-357`.

Before (x8):

```go
c := liveClient(t)
ctx, cancel := liveCtx(t)
defer cancel()
m := liveScope(t, c, ctx)
```

After:

```go
// liveSetup builds the client, a 60s context tied to the test's lifetime,
// and the first membership every fact needs (account id, business id,
// business uuid).
func liveSetup(t *testing.T) (*freshbooks.Client, context.Context, freshbooks.Membership) {
	t.Helper()
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)
	ms, err := c.Identity.Me(ctx)
	if err != nil {
		t.Fatalf("Identity.Me: %v", err)
	}
	if len(ms) == 0 {
		t.Fatal("the authorized account has no business memberships")
	}
	return c, ctx, ms[0]
}

// in each test:
c, ctx, m := liveSetup(t)
```

Why behaviour-preserving: same calls in the same order, same 60s timeout, same skip/fatal paths; `t.Cleanup(cancel)` fires where `defer cancel()` did. Test-only file under the `live` tag. Bonus: removes the `liveScope(t, c, ctx)` signature, which has `ctx` third (the one ctx-not-first call in the package). About 30 lines net. Leave `TestLiveIdentity` in `live_test.go` alone (not in the diff).

Risk: none. `t.Context()` needs Go 1.24+; the modules are `go 1.26`.

### S2 -- inline `expenseVendorsEnvelope`; its doc duplicates the method doc

`freshbooks/expenses.go:327-335` (type) and `:337-348` (method doc). The named type is used once, and its comment ("PAGINATED list of one-key objects, not the bare string array Phase 2 inferred, CONFIRMED live 2026-09-02") says exactly what the `Vendors` doc comment two lines below says. The pre-branch `Vendors`, and `Types`/`SubTypes`/`SubType` in `ledger_accounts.go`, all decode into an inline anonymous struct.

Before:

```go
type expenseVendorsEnvelope struct {
	PageMeta
	Vendors []struct {
		Vendor string `json:"vendor"`
	} `json:"vendors"`
}
...
	for page := 1; ; page++ {
		var env expenseVendorsEnvelope
```

After:

```go
	for page := 1; ; page++ {
		var env struct {
			PageMeta
			Vendors []struct {
				Vendor string `json:"vendor"`
			} `json:"vendors"`
		}
```

Keep the method doc as is: it carries the page-size-15 observation and the "returning the first 15 would be a trap" reasoning, which live nowhere else in code. Optionally drop its parenthetical "(CONFIRMED live 2026-09-02; Phase 2 had inferred ... does not decode)" -- that history is already in `freshbooks/CHANGELOG.md` and the spec callout -- leaving one CONFIRMED marker on the method.

Why behaviour-preserving: unexported type, identical tags and embedding; the decoder sees the same shape. About 8 lines net.

Risk: none.

### S3 -- drop the pass-through closure around `serveFixture`

`freshbooks/gateways_test.go:50-52`:

```go
c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	serveFixture(t, http.StatusOK, "gateways", "get_stripe_unified")(w, r)
}))
```

After:

```go
c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "gateways", "get_stripe_unified"))
```

`serveFixture` already returns an `http.HandlerFunc`; the closure form is only needed when the test also captures `r.URL.Path` (the `[happy]` case above it does; this one does not). The ledger taxonomy tests already use the direct form.

Why behaviour-preserving: same handler, same fixture, same status. Risk: none.

## OPTIONAL

### S4 -- `slices.ContainsFunc` in the `Types` unit test

`freshbooks/ledger_accounts_test.go:161-168`:

```go
var names []string
for _, ty := range got {
	names = append(names, ty.Name)
}
if !slices.Contains(names, "income") {
	t.Fatalf("names = %v, want the live taxonomy including income", names)
}
```

After:

```go
if !slices.ContainsFunc(got, func(ty LedgerAccountType) bool { return ty.Name == "income" }) {
	t.Fatalf("got = %+v, want the live taxonomy including income", got)
}
```

`slices` is already imported. The failure message prints the structs instead of the bare names; same information. Four lines. Take it or leave it.

### S5 -- one `clock` helper for the duplicated nil-clock default

`cli/internal/auth/status.go:52-55` (`Status`) and `:113-116` (`Token`) now both carry:

```go
nowFn := now
if nowFn == nil {
	nowFn = time.Now
}
```

A package-private helper removes the second copy:

```go
// clock returns now, or time.Now when the caller passed nil.
func clock(now func() time.Time) func() time.Time {
	if now == nil {
		return time.Now
	}
	return now
}

// Status:  info.Valid = tok.Valid(clock(now)(), fbauth.DefaultExpirySkew)
// Token:   if !refresh && tok.Valid(clock(now)(), fbauth.DefaultExpirySkew) {
```

Behaviour identical (`cmp.Or` cannot do this: funcs are not comparable). Net about -4 lines; only worth it if the fix commit is already in `status.go`. The `now func() time.Time` signature on `Token` is the right call -- it matches `Status`, not `runCallbackServer`'s `time.Time`, because both need "now" at call time, not at construction.

### S6 -- a saved-token helper for the four new `TestToken` subtests

`cli/internal/auth/status_test.go:249-337`. Each new subtest does `store := fbauth.NewMemoryStore()` + `store.Save(ctx, &fbauth.Token{... Expiry: ...})` + `if err != nil { t.Fatal(err) }`. A `savedStore(t *testing.T, tok *fbauth.Token) fbauth.TokenStore` helper cuts three lines from each of the four (and from the pre-existing subtests if someone wants, but do not retrofit in this phase). Marginal; the tests read fine as they are.

## DO-NOT-APPLY (considered, rejected)

### S7 -- rewriting `Vendors` over `All[T]`

Looked at because the lead asked for it. It does not shrink and it changes edge behaviour. `All` needs `fetch func(ctx, page) (*Page[T], error)`, so the closure that decodes the envelope and flattens `[]struct{Vendor}` to `[]string` via `newPage` is the loop body that exists now, and the `for v, err := range All(ctx, fetch)` collector replaces the `for _, v := range env.Vendors` append: same line count, one more indirection. Two behaviour differences: `All` checks `ctx.Err()` before every page (a cancelled context returns bare `context.Canceled` instead of the transport's wrapped error), and its stop rule is `len(items)==0 || (Pages > 0 && page >= Pages)`, so a `pages: 0` response with items would request page 2 where the current `env.Pages <= page` stops. Neither is reachable against the real API; both are still behaviour changes, so not a simplification. `listOpts`/`pathSegment`/`softDelete` do not apply: `Vendors` has no options struct, `expenseVendorsPath` already validates the id, nothing is deleted.

### S8 -- de-duplicating captures that are byte-identical to fixtures

`freshbooks/testdata/seed/gateways/get.json` is byte-identical to `freshbooks/testdata/gateways/get_stripe_unified.json`. That is the established convention, not an accident: six Phase 1 pairs on `main` are already identical (`seed/users_me.json` == `auth/users_me.json`, `seed/accounting_clients_list.json` == `accounting/clients_list.json`, `seed/projects_list.json` == `projects/list.json`, and the three error captures). `seed/` is the verbatim-capture ledger the spec callouts cite by path; `testdata/<resource>/` is what `serveFixture` reads and gets trimmed freely. Symlinking or pointing one at the other would couple a unit fixture's future edits to a capture that is supposed to stay verbatim. Likewise `seed/time_entries/list.json` and `list_bracket_filter_ignored.json` have identical bodies, and that identity IS the fact B evidence (the bracket spelling produced the unfiltered response); both stay, both named for their probe.

### S9 -- collapsing the two "no refresh token" error messages in `Token`

`cli/internal/auth/status.go:120-125`. One message would be shorter; the distinct "has expired and no refresh token is stored" text is CLI output and the new `[sad]` test asserts "expired" in it.

### S10 -- trimming the evidence comments

`staffListResponse` (seventeen-key list and the reason `List` does not return the business record), `Page.Sort` / `PageMeta.Sort` (echo, not validation), `StripeUnifiedConnection` (why `Stripe` stays alongside), `Vendors` (page size 15). Each records a live observation and a decision taken on it. The spec callouts and changelog entries repeat them because the process requires those mirrors; the code copy is the one a reader of the type sees. Keep.

## Out of lane, flagged for the code-review lane

- `freshbooks/expenses.go:355` -- `var vendors []string` is nil when the account has no vendors, and neither branch of the loop allocates. The pre-branch decode of `"vendors": []` produced an empty non-nil slice. `mcp/internal/tools/tools_expenses.go:90` and `cli/internal/cmd/commands_expenses.go:90` marshal the return value directly, so the empty case now encodes as `null` where it used to be `[]`. `vendors := []string{}` restores `[]`. Not a simplification, so not tagged; noting it because S2 edits the same lines.

## Summary for triage

| Id | Tag | File | Net |
|---|---|---|---|
| S1 | APPLY-RECOMMENDED | `freshbooks/live_conformance_test.go` | ~-30 lines, fixes ctx argument order |
| S2 | APPLY-RECOMMENDED | `freshbooks/expenses.go` | ~-8 lines, removes a duplicated CONFIRMED note |
| S3 | APPLY-RECOMMENDED | `freshbooks/gateways_test.go` | -2 lines |
| S4 | OPTIONAL | `freshbooks/ledger_accounts_test.go` | -4 lines |
| S5 | OPTIONAL | `cli/internal/auth/status.go` | -4 lines |
| S6 | OPTIONAL | `cli/internal/auth/status_test.go` | ~-12 lines |
| S7-S10 | DO-NOT-APPLY | -- | behaviour or convention changes |
