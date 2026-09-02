# Phase 3 review gate -- simplification lane

Branch `phase-3/mcp` (`git diff main...phase-3/mcp`, 77a86be..0cc509b). Scope: `mcp/` (`internal/tools`, `internal/config`, `internal/server`, `cmd/freshbooks-mcp`) and `docs/mcp.md`. **Propose only** -- nothing in this report was applied, no gate was run.

Every item below is behaviour-preserving by construction: no tool name, input-schema shape, field name, description string, output JSON, error content, annotation, transport semantic, env var, exit code, or exported signature changes. Where that claim rests on a library detail I verified it in the module cache rather than assuming it (see "Evidence" at the end).

Headline: the registry design (`Spec`/`newSpec`/`shapes.go`) is genuinely good -- 168 tools with one line of per-tool logic each is the right shape, and most of what looks like duplication across the 33 `tools_*.go` files is per-resource *data* (descriptions, inventory keys) that must stay distinct. The real cuts are five contained ones (#1-#5), one large-but-mechanical one the lead should decide on explicitly (#7), and a pile of micros.

## A cheap way to verify any of these

None of these refactors is pinned by a golden-schema test -- the round-trip test synthesizes *from* `InputSchema`, so it moves with the schema instead of anchoring it. Before/after diff of the manifest is the direct check, and it covers every proposal here in one command:

```
go run ./mcp/cmd/freshbooks-mcp tools > /tmp/manifest-before.json
# apply changes
go run ./mcp/cmd/freshbooks-mcp tools > /tmp/manifest-after.json
diff /tmp/manifest-before.json /tmp/manifest-after.json   # must be empty
```

Worth handing to QA regardless of which items land.

---

## APPLY-RECOMMENDED

### 1. `tools_reports.go:9-72` -- twelve identical wrapper structs collapse to one generic

Twelve report inputs differ only in the Go type of one field. Field name, JSON tag, and jsonschema description are byte-identical across all twelve.

Before (x12):
```go
type reportsAccountsAgingIn struct {
	AcctScope
	Options freshbooks.AccountsAgingOptions `json:"options,omitempty" jsonschema:"report filter options"`
}
type reportsBalanceSheetIn struct {
	AcctScope
	Options freshbooks.BalanceSheetOptions `json:"options,omitempty" jsonschema:"report filter options"`
}
// ... 10 more
```

After:
```go
// reportIn is the input shape every Reports.<Name>(ctx, accountID, *Options)
// tool shares: an account scope plus that report's own options struct.
type reportIn[O any] struct {
	AcctScope
	Options O `json:"options,omitempty" jsonschema:"report filter options"`
}
```
and at each call site `in reportsAccountsAgingIn` becomes `in reportIn[freshbooks.AccountsAgingOptions]` (lines 80, 87, 94, 101, 119, 126, 133, 140, 147, 154, 161, 168). `reportsDownloadInvoiceDetailsCSVIn` (line 29) stays as-is -- it carries a `download_token`, not options.

Behaviour-preserving because `jsonschema.ForType` inlines struct fields by reflection and never emits a Go type name into the schema (no `$defs`, no `$ref`); the type's name is used only as a cycle-detection key, and each instantiation is a distinct named type. Same fields, same order, same tags -> same schema bytes.

Net: -50 lines, one file, no cross-file churn. **Risk: low.**

### 2. `config.go:239-266,271-298` -- one flag table instead of two tables plus inline literals

`flagDefs` declares an `env` field that **has no reader**: `Load` re-types every env var name as a string literal at the call site (lines 294-298), and the usage strings live in a *second* map keyed by flag name. Three places to edit to add a flag; two of them can silently drift.

Before:
```go
var flagDefs = []struct{ name, env, def string }{
	{"transport", "FRESHBOOKS_MCP_TRANSPORT", "stdio"},
	...
}
var flagUsage = map[string]string{"transport": "transport to serve on: stdio or http", ...}

func stringFlag(cmd *cobra.Command, name, env string) string { ... }

Transport: stringFlag(cmd, "transport", "FRESHBOOKS_MCP_TRANSPORT"),
```

After:
```go
var flagDefs = []struct{ name, env, def, usage string }{
	{"transport", "FRESHBOOKS_MCP_TRANSPORT", "stdio", "transport to serve on: stdio or http"},
	...
}

var envForFlag = func() map[string]string {
	m := make(map[string]string, len(flagDefs))
	for _, f := range flagDefs {
		m[f.name] = f.env
	}
	return m
}()

// stringFlag resolves one flag: flag > its FRESHBOOKS_MCP_* env twin > default.
func stringFlag(cmd *cobra.Command, name string) string { /* env := envForFlag[name] */ }

Transport: stringFlag(cmd, "transport"),
```

Behaviour-preserving: identical flag names, defaults, usage strings, env names, and the same flag > env > default order including the "empty env var is unset" rule (config.go:279-285), which is untouched. Covered by `TestLoadPrecedence`. **Risk: low.**

### 3. `tools_retainers.go:102-107` -- reimplements `listIn.search()`

The retainers list closure hand-rolls the nil-if-empty `Search` conversion that `shapes.go:27-32` already owns. `retainersListIn` legitimately cannot embed `listIn` (documented at lines 10-13: `RetainerListOptions` has no `Page`/`PerPage`) but it can share the conversion.

Before:
```go
var search freshbooks.Search
if len(in.Search) > 0 {
	search = freshbooks.Search(in.Search)
}
return c.Retainers.List(ctx, scope.BusinessID, &freshbooks.RetainerListOptions{Search: search})
```

After (`shapes.go`):
```go
// searchOf converts a wire filter map to the lib's Search type, nil when
// empty so an omitted filter never sends an empty search[] parameter.
func searchOf(m map[string]string) freshbooks.Search {
	if len(m) == 0 {
		return nil
	}
	return freshbooks.Search(m)
}

func (l listIn) search() freshbooks.Search { return searchOf(l.Search) }
```
and in `tools_retainers.go`:
```go
return c.Retainers.List(ctx, scope.BusinessID, &freshbooks.RetainerListOptions{Search: searchOf(in.Search)})
```

Struct definitions untouched (retainers keeps its own field and its own description), so no schema change; the conversion is provably the same expression. **Risk: low.**

### 4. `tools_identity.go:17-23` -- two single-embed wrapper types that six other tools do without

```go
type identityDeleteBusinessIn struct{ BizScope }
type identityDeleteBusinessSubscriptionIn struct{ AcctScope }
```
Both wrap exactly one embedded scope struct and add nothing. Elsewhere in the same package the bare scope type is already used directly as `In` -- `gateways_get` (`AcctScope`), `invoices_invoice_presentation_defaults` (`AcctScope`), `staff_list` / `projects_abilities` / `service_rates_list` / `team_members_rates` (`BizScope`), `ledger_accounts_list` (`UUIDScope`). Delete both types and use `in BizScope` / `in AcctScope` at lines 95 and 102.

Behaviour-preserving: `jsonschema-go` walks `reflect.VisibleFields` and skips anonymous fields, promoting their members, so `struct{ BizScope }` and `BizScope` produce the same object schema with the same single `business_id` property and the same property order. **Risk: low.**

### 5. `registry.go:114-150` -- 33 hand-rolled appends -> one stdlib call

```go
func buildRegistry() []Spec {
	return slices.Concat(
		attachmentsSpecs, billPaymentsSpecs, billsSpecs, billVendorsSpecs,
		callbacksSpecs, clientsSpecs, contactsSpecs, creditNotesSpecs,
		... // same 33 names, same order
	)
}
```
`slices.Concat` (Go 1.22+, module is on 1.26) preserves order exactly, so `All`'s contents and index order are unchanged -- which matters because `TestRoundTrip` and `TestParityKeyCoverage` iterate `All`. -20 lines. **Risk: low.**

### 6. `roundtrip_test.go:199-226` and `unit_test.go:216-246` -- three copies of the same session wiring

`redaction_test.go:272-309` already has `newTestSession(t, upstream, logger)`, which does exactly what `TestRoundTrip` open-codes (client + `redirectTransport` + `mcp.NewServer` + `Register` + in-memory transports + both sessions closed) and what `TestMissingScopeIsError` open-codes with a different default scope.

Add one parameter:
```go
func newTestSession(t *testing.T, upstream *httptest.Server, defaults tools.Scope, logger *slog.Logger) *mcp.ClientSession
```
(inside the package it is just `Scope`), then:
- `TestRoundTrip`: lines 201-226 -> `clientSession := newTestSession(t, upstream, testScope, nil)`
- `TestMissingScopeIsError`: lines 218-246 -> `clientSession := newTestSession(t, upstream, Scope{}, nil)`
- `redaction_test.go` call sites pass `testScope`.

Test-only; identical wiring, and `t.Cleanup` closes the sessions at the same point `defer` did (both are top-level test functions). -55 lines. **Risk: low.**

---

## OPTIONAL

### 7. 45 per-resource input structs across 20 files are byte-identical to seven shared shapes

The largest single duplication in the diff. These families have identical fields, identical tags, and identical descriptions -- the only thing distinguishing them is the type name:

| Shape | Count | Types |
|---|---|---|
| `{AcctScope; listIn}` | 15 | bills, billVendors, callbacks, creditNotes, expenseCategories, expenses, invoiceProfiles, invoices, items, journalEntriesDetails, journalEntryAccounts, otherIncome, payments, tasks, taxes |
| `{AcctScope; idIn}` | 13 | bills, billVendors, clients, creditNotes, estimates, expenses, invoiceProfiles, invoices, items, payments, servicesGetBillableItem, tasks, taxes |
| `{AcctScope; idIn; includeIn}` | 7 | clients, estimates, expenseCategories, expenses, invoiceProfiles, invoices, taxes |
| `{BizScope; listIn}` | 4 | projects, services, teamMembers, timeEntries |
| `{AcctScope; listIn; includeIn}` | 2 | clients, estimates |
| `{AcctScope; uploadIn}` | 2 | attachments, images |
| `{BizScope; ServiceID int64 "the service id"}` | 2 | services, serviceRates |

Collapsing them into `acctListIn`, `acctIDIn`, `acctIDIncludeIn`, `bizListIn`, `acctListIncludeIn`, `acctUploadIn`, `bizServiceIn` in `shapes.go` removes ~45 declarations (~190 lines) and 20 files' worth of near-identical boilerplate. Schemas are unchanged for the same reflection reason as #1 and #4.

**Why OPTIONAL rather than recommended.** Two honest costs:

1. Call sites lose a naming cue: `in invoicesIDIn` becomes `in acctIDIn`. The tool name on the line above still says which resource it is, so this is small, but it is real.
2. This phase already shows the shared shape being outgrown: contacts, projects, staff, otherIncome, timeEntries, callbacks and payments all hand-rolled their own `ID` field instead of embedding `idIn`, precisely because `idIn`'s generic "the resource id" description was wrong for them. Some of the 45 will want to diverge later, and un-sharing is a local edit -- cheap, but churn.

Plus the blast radius: ~20 files touched right before a merge gate, for a line-count win rather than a behaviour or clarity win.

If the lead wants a middle path: take the two biggest families only (`{AcctScope; listIn}` x15 and `{AcctScope; idIn}` x13), which are 28 of the 45 and the least likely to diverge (a list filter and a bare id are the most generic shapes there are). **Risk: low per-edit, medium in aggregate churn** -- verify with the manifest diff above.

### 8. `sort` -> `slices` in three files

`registry.go:105` (`sort.Slice` -> `slices.SortFunc` with `cmp.Compare` on names), `unit_test.go:149` (`sort.SliceIsSorted` -> `slices.IsSortedFunc`), `parity_test.go:171,195` (`sort.Strings` -> `slices.Sort`) and `:203` (`reflect.DeepEqual` on `[]string` -> `slices.Equal`, which also drops the `reflect` import if nothing else needs it -- it does, `TestParityAgainstClient` uses `reflect.TypeFor`). All are exact-equivalent modern stdlib forms; tool names are unique so sort stability is not observable. **Risk: low.** Cosmetic; skip if the lead prefers a smaller diff.

### 9. `shapes.go:67-70` -- `ok()` has one caller

`ok()` is called only from `void()` (plus its own doc comment and a test *name*). Inline it:
```go
func void(err error) (any, error) {
	if err != nil {
		return nil, err
	}
	// A successful call still carries content rather than an empty result.
	return map[string]bool{"ok": true}, nil
}
```
`TestVoid` asserts on the returned `map[string]bool`, not on `ok` as a symbol, so it keeps passing unchanged. **Risk: low.** Marginal -- only worth doing if #3 is touching `shapes.go` anyway.

### 10. `server_test.go:21-29` -- `fakeFreshBooksUpstream` could own its own cleanup

Six of seven call sites write `upstream, _ := fakeFreshBooksUpstream(t)` followed by `defer upstream.Close()`, and five discard the `*bearerLog` entirely. Adding `t.Cleanup(srv.Close)` inside the helper removes seven `defer` lines. The three `TestHTTPHandler*`/`TestStatelessProperty` cases additionally repeat `srv := New(testConfig(upstream.URL), "test"); ts := httptest.NewServer(srv.HTTPHandler()); defer ts.Close()`, which a `newHTTPTestServer(t) (*httptest.Server, *bearerLog)` helper folds into one line. Test-only, same lifetimes (all call sites are top-level test functions). **Risk: low.**

### 11. `redaction_test.go:339-346` -- linear spec lookup, no break

```go
var spec *Spec
for i := range All {
	if All[i].Name == name { spec = &All[i] }   // scans all 168 every time
}
```
Names are unique (asserted by `TestParityAgainstToolsMD`), so this can be a small `specByName(t, name) *Spec` helper with a `break`, or `slices.IndexFunc`. Behaviour identical. **Risk: low.** Micro.

### 12. `tools_attachments.go:16` -- a stray `// inventory:` comment that nothing reads

```go
// attachmentsSpecs are the tools wrapping *freshbooks.AttachmentsService.
//
// inventory: Uploader/Upload Expense Receipt      <-- this line
var attachmentsSpecs = []Spec{
```
It duplicates `Keys` three lines below, and it is the **only** `// inventory:` comment in the whole `mcp` module -- the other 32 `tools_*.go` files carry none. `scripts/check.sh:61-69` runs `inventory-check` only against the `freshbooks` module (`if [ "$module" != "freshbooks" ]` -> skip), so nothing parses it and it cannot drift into a check failure; it can only drift out of sync with `Keys` silently.

**Flagged, not recommended**: my brief says `// inventory:` comments are mandatory and to leave them. That rule is about the lib's parity contract, and this comment is not part of it -- but the call is the lead's, not mine. **Risk: low** either way.

---

## DO-NOT-APPLY (considered and rejected -- do not re-derive)

1. **Collapse the 20 `{AcctScope; Body freshbooks.XRequest}` create inputs into a generic `acctBodyIn[B any]`.** Tempting (it is the largest count of all), but each carries a distinct description -- "the client fields to create", "the payment fields to create", "the journal entry fields to create". A generic would force one shared string, changing 20 tools' input schemas. Same reasoning kills the 15 `{AcctScope; idIn; Body}` update inputs and the 5 `{Body}`-only ones.

2. **Unify the hand-rolled `ID int64` fields onto `idIn`.** contacts ("the secondary contact id"), projects ("the project id"), timeEntries ("the time entry id"), staff ("the staff member's id"), otherIncome ("the other-income record's id"), callbacks ("the callback id") each wrote their own so the model sees a specific description. Embedding `idIn` would rewrite all of them to "the resource id" -- a schema change, and a worse one. Worth noting as a *deliberate* inconsistency, not a defect: `idIn` is used where the generic wording is fine and skipped where it is not.

3. **A generic helper for the ~20 repeated `&freshbooks.XListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage}` literals.** Go cannot set fields on a type parameter, and the option structs are not uniform (`ClientListOptions` and `EstimateListOptions` add `Include`), so this needs either reflection (slower, panics at runtime instead of compile time, and defeats the point) or a `~struct{...}` core-type constraint that would be less readable than the literal it replaces and still would not cover the `Include` variants. The literal is one line and reads fine.

4. **Turn `newSpec`'s per-tool `Call` closures into a data table.** The closure *is* the per-tool logic -- one line naming one lib method with its own argument list. There is nothing left to factor without inventing a mini-interpreter.

5. **Merge the 33 `tools_*.go` files into fewer files.** File-per-resource mirrors `freshbooks/<resource>.go` and keeps a resource's inputs next to its specs. Fewer, bigger files would be harder to navigate, not simpler.

6. **Make `Spec.add` a method instead of a closure field.** It cannot be: `add` is where the generic `In` type parameter survives after `newSpec` erases it into a non-generic `Spec`. Current design is correct.

7. **Remove `stringFlag`'s `if f == nil` guard (config.go:272-274).** It is over-defensive against a caller that skipped `AddFlags`, but it costs two lines and turns a nil-pointer panic into an empty string that `Validate` then rejects with a clear message. Keep.

8. **Remove `errorText`'s marshal-error fallback (errors.go:50-53).** `fbErrorContent` is four scalars and a string; `json.Marshal` cannot fail on it. But the fallback is two lines, the error must be handled anyway to satisfy the linter, and it degrades to `err.Error()` rather than silently emitting `""`. Keep.

9. **`hintRO`/`hintI` using plain `bool` while `hintD`/`hintW` use `boolPtr` (registry.go:155-160).** Not an inconsistency: `mcp.ToolAnnotations` declares `ReadOnlyHint bool` and `IdempotentHint bool` but `DestructiveHint *bool` and `OpenWorldHint *bool` (go-sdk protocol.go:1974-1991). The code mirrors the SDK exactly. Leave, including `boolPtr`.

10. **Table-drive `run_test.go`.** Five subtests with five different assertion shapes (exit code, JSON manifest length, stderr substring, env setup). A table would need a per-row closure, i.e. the same code with extra scaffolding.

11. **Trim `docs/mcp.md`.** 119 lines covering install, both transports, both client setups, a working `curl` pair, two config tables, and the two structural security constraints. Every paragraph carries information that is not in the code. Nothing to cut; the prose is already un-padded.

12. **`SilenceErrors: false` in root.go:27.** Explicitly writing the zero value next to `SilenceUsage: true` documents that the pairing is deliberate (usage suppressed, errors not) -- which `run_test.go`'s stderr assertions depend on. Keep.

## Left alone, briefly

- The `Spec`/`newSpec`/`Register`/`Manifest` core and the `schemaFor` package-init story (`schema.go:32-43`) -- precomputing schemas once per tool at init, shared across every `*mcp.Server`, is the right call and the comment explains why.
- All doc comments carrying evidence: `schema.go:10-25` (the three `typeOverrides` and *why* jsonschema-go rejects the alternatives), `tools_retainers.go:24-36` (the `json.Number` schema-vs-decoder trap), `tools_contacts.go:32-35` and `tools_gateways.go:14-15` (double spaces copied verbatim from Postman), `tools_services.go:19-22` and `tools_staff.go:20-22` (account-vs-business scope confirmed against lib signatures), `roundtrip_test.go:91-98` (`redirectTransport` and the tokenization host), `errors.go:11-16` (why a Go error must not be returned from a handler). These are exactly the comments that should survive a simplification pass -- none restates its code.
- `internal/server` generally: `bearerToken`/`requireBearer`/`getServer`/`HTTPHandler` are each short and single-purpose, and `getServer`'s unreachable-error branch (server.go:124-130) is documented and degrades safely.
- `Config.LogValue` and `Config.String` duplicating the field list -- they serve different consumers (slog vs `%v`) and both are load-bearing for the no-secret-leak constraint.

## Evidence

Library behaviour I checked rather than assumed, since #1, #4 and #7 all rest on it:

- `jsonschema-go@v0.4.3/jsonschema/infer.go:98-111` -- `ForType` returns an inlined schema; there is no `$defs`/`$ref` emission and no place a Go type name reaches the output.
- Same file, `forType` cycle check at line 127-135 -- `t.Name()` is used **only** as a `seen` map key for cycle detection, per call, and each generic instantiation is a distinct named type. Renaming or genericizing an input type cannot change its schema.
- Same file, `case reflect.Struct` -- fields come from `reflect.VisibleFields(t)` with `field.Anonymous` skipped, so an embedded struct's members are promoted in declaration order. `struct{ BizScope }` and `BizScope` therefore produce the same properties and the same `PropertyOrder`.
- `go-sdk@v1.7.0/mcp/protocol.go:1974-1991` -- `ToolAnnotations` really does mix `bool` and `*bool`; registry.go's hint sets mirror it.
- `scripts/check.sh:61-69` -- `inventory-check` short-circuits for any module other than `freshbooks`, which is why the `mcp` module's one `// inventory:` comment (#12) is inert.
