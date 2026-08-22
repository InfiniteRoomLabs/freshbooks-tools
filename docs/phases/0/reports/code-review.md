# Phase 0 (scaffold) -- code-review lane

Branch `phase-0/scaffold`, `git diff main...phase-0/scaffold` (`b3d69ac..9545506`, 9 commits). Read-only pass. No exemplar exists; judged against the design spec (sections 2, 3, 4, 5.2, 8, 9.5, 10), `docs/phases/0/plan.md`, `CLAUDE.md`, and Go canon.

## Verdict

**REQUEST CHANGES**

Four blocking findings. Three are real defects in the only substantive code this phase ships (the inventory tool and the gate that guards it); one is a read-first doc that now contradicts itself. The rest of the scaffold -- module layout, cobra wiring, workflows, goreleaser, docs -- is in good shape, and the inventory test suite is genuinely thorough (38 test functions, table-driven, tagged, no `t.Skip`, no committed `-run` filters).

The headline problem is that finding 1 is a data-corruption bug that the golden test could not see, because the golden test asserts *counts* and *byte-stability* but never asserts that the normalized output is well-formed. Two of the 213 committed entries are wrong, and `inventory.json` is the contract every later phase is built from.

---

## Verdicts on the implementer's six declared deviations

**#1 `Single Tax` GET/DELETE disambiguation via a `" (DELETE)"` key suffix -- ACCEPT.**
Verified independently against the real collection: `Expenses/Single Tax` and `Settings/Items and Services/Single Tax` are each a GET and a DELETE sharing one Postman name and one URL. The spec's "exact duplicates" claim was wrong; collapsing them would have silently destroyed both DELETE operations. The implementer chose correctly, corrected spec section 3 in the same branch (`4102a06`), and `TestLoadRealCollectionGolden` asserts the disambiguated keys. `dedupe` still fails loudly if the suffix itself collides, and `TestNormalizeConflictingDuplicatesFailWhenDisambiguationCollides` covers that. Good call. One ergonomic note in finding 8 below.

**#2 `#USAGE` with no space -- ACCEPT.** `usage` 6.0.0's KDL parser rejects the spaced form the global `CLAUDE.md` shows. All four argument-taking scripts use the working form. Nothing to change here; the global example is what is wrong.

**#3 goreleaser `monorepo` removed, `sbom:` -> `sboms:` -- ACCEPT the schema facts, ADVISORY on the workaround.** Both corrections are right for goreleaser 2.17 OSS (`monorepo` is Pro-only; `sboms` is the correct list key). The `GORELEASER_CURRENT_TAG` substitute has a specific predictable failure -- see finding 9.

**#4 coverage gate excludes `main.go` by filename -- REJECT (blocking).** The *intent* is legitimate: `func main()` calling `os.Exit` genuinely cannot be exercised in-process. The *mechanism* is too blunt and does real damage. See finding 3.

**#5 inline Go fixtures instead of `testdata/fixtures/` -- ACCEPT.** Idiomatic Go, keeps each fixture adjacent to its assertion, and the JSON-parsing path is separately covered by `TestNormalizeStringURL` / `TestNormalizeObjectURL` / `TestURLUnmarshalJSONInvalid` / the golden test. The work order's phrasing was a suggestion, not a contract.

**#6 `required_approving_review_count=0` unverified -- ACCEPT as documented.** GitHub does accept 0 on this endpoint. Low risk, correctly flagged for whoever runs the script during Ship.

---

## Findings

### 1. BLOCKING -- scheme-less URLs put the host into `pathTemplate`, blank the `host` field, and mis-classify the family

`freshbooks/internal/inventory/inventory.go:184`

`normalizeURL` calls `url.Parse(raw)` and then trusts `parsed.Host` and `parsed.Path`. Two requests in the real collection have no scheme:

```
Estimates/Update Estimate -> 'api.freshbooks.com/accounting/account/{{accountId}}/estimates/estimates/{{estimateId}}'
Estimates/Accept Estimate -> 'api.freshbooks.com/accounting/account/{{accountId}}/estimates/estimates/{{estimateId}}'
```

`url.Parse` on a scheme-less, non-rooted string treats the entire thing as an opaque path. Both entries are committed to `testdata/inventory.json` as:

```json
"pathTemplate": "api.freshbooks.com/accounting/account/{accountId}/estimates/estimates/{estimateId}",
"host": "",
"family": "unknown"
```

Three separate violations of the Entry contract in `docs/phases/0/plan.md` section B: `pathTemplate` is documented as "path only, no host, leading `/`" (it has the host and no leading slash), `host` is documented as "original host, after rewrite" (it is empty), and `family` should be `accounting` -- `classifyFamily` returns `unknown` only because the `/accounting/account/` prefix test fails against a string that starts with `api.freshbooks.com`.

This matters beyond cosmetics: `inventory.json` is the work-order source for Phase 2. An implementer reading these two entries gets a malformed URL template and a wrong family bucket, and `family` is what the phase plan says drives the Phase 2 batch split.

Fix (in `normalizeURL`, before `url.Parse`):

```go
raw := stripWhitespace(u.Raw)
if !strings.Contains(raw, "://") {
    raw = "https://" + raw
}
```

Then regenerate `testdata/inventory.json` and add a table case to `TestNormalizeStringURL` for a scheme-less raw URL. Note the `{{accountId}}` substitution happens to still work today only because `normalizePathSegments` is position-independent -- do not rely on that.

### 2. BLOCKING -- three unique API operations are permanently removed from the parity contract

`freshbooks/internal/inventory/testdata/ignore.list:5-7`

The three `internal`-family entries are listed as `//go:inventory-ignore ... -- internal my.freshbooks.com endpoint`, under a header that reads "never delete an ignore entry." Their normalized paths are:

| Key | Method | Normalized path |
|---|---|---|
| `Accounting/Journal Entries/Add Journal Entry` | POST | `/accounting/account/{accountId}/journal_entries/journal_entries` |
| `Projects/Delete Project` | DELETE | `/comments/business/{businessId}/project/{projectId}` |
| `Settings/Businesses/Delete Business - Subscription` | DELETE | `/auth/api/v1/billing/account/{accountId}/subscription` |

I checked the full inventory: none of the three has a public-host duplicate. So this is not "an internal alias of an endpoint we already cover" -- it permanently drops the *only* create-journal-entry, the *only* delete-project, and the *only* delete-subscription operations in the collection. After these paths go through rule 4's rewrite they look like ordinary public endpoints; the only thing marking them "internal" is the host FreshBooks' own collection author happened to type.

The parity contract exists precisely so nothing falls off the list unnoticed. `ignore` is the one bucket that says "never revisit this," and it is the wrong bucket here.

Fix: move all three to `//go:inventory-todo <key> -- phase-2`, with the reason recorded (e.g. `phase-2 (verify host: collection lists my.freshbooks.com)`). Phase 2 can then confirm against the live API and demote to `ignore` with evidence if the public host genuinely 404s. If the ignore/todo format cannot carry that nuance, keep `todo` and put the note in the file header. `docs/agentic-transformation.md:38` cites "the three `my.freshbooks.com` internal endpoints" as the example of `ignore` and needs the same edit.

### 3. BLOCKING -- the coverage gate's `main.go` filter over-excludes tested logic and lets a whole module pass vacuously

`scripts/coverage-gate.sh:24-30`

```sh
tail -n +2 "$usage_coverprofile" | grep -v '/main\.go:' || true
...
if [ "$(wc -l <"$filtered")" -le 1 ]; then
  echo "... no measurable statements outside main.go -- nothing to cover, PASS"
```

Two concrete problems:

**(a) It excludes substantive, already-tested code.** `freshbooks/internal/inventory/main.go` is not thin `os.Exit` wiring -- it is roughly 60 statements of flag parsing, mode dispatch, error reporting, and report rendering across `run`, `runEmit`, and `runCheck`. `main_test.go` covers all three with nine subtests including five sad paths. The filter throws all of that measured coverage away, and from here on any logic added to any `main.go` in any module is invisible to the 90% floor. `CLAUDE.md` states the floor as "90% per module," unqualified.

**(b) The vacuous-pass branch disables the gate entirely for `mcp`.** The `mcp` module's only code today lives in `main.go`, so the filter empties the profile and the script exits 0 with no number. `mise run check -- mcp` currently reports a passing coverage gate while enforcing nothing, and it will keep doing so for as long as `mcp`'s code lives in `main.go` -- which is exactly the window where a regression would be easiest to introduce.

The honest number is small: `main()` in all three binaries is 1-4 statements. `mcp`'s raw 40% is low only because the module has almost no statements at all, not because `main()` is disproportionate.

Suggested fix, cheapest first:

- Drop the filter entirely and set `main()` to a single statement (`os.Exit(run(os.Stdout, os.Stderr, os.Args[1:]))`), as `freshbooks/internal/inventory/main.go:16-18` already does. The one uncovered statement barely moves the number once each module has real code.
- If a filter is kept, scope it to `cmd/*/main.go` (matching what `docs/building.md:20` already claims it does) so `internal/inventory/main.go` stays counted.
- Replace the vacuous pass with a hard failure. "No measurable statements" should be a red gate for a module that is supposed to have code, not a green one.

Whatever is chosen, `docs/building.md:20` must match it -- today it says `cmd/*/main.go` while the filter matches any `main.go` anywhere.

### 4. BLOCKING -- `docs/progress.md` contradicts the spec correction made in the same branch

`docs/progress.md:28`

The bullet added by this branch says:

> 2 exact duplicate entries (`Single Tax` under `Expenses` and under `Settings/Items and Services`) collapse to one entry each.

Commit `4102a06` on this same branch adds a spec callout stating the exact opposite -- that they are *not* duplicates, must *not* collapse, and that the collection normalizes to 213 distinct keys. `CLAUDE.md` names `progress.md` "living status -- always read before starting work," so this is the first thing the Phase 1 agent reads, and it is wrong.

The rest of the file is also stale for a phase boundary: "No Go code yet. No GitHub repo yet (Phase 0 creates it)", the ledger row `0 Scaffold | not started`, and "Next action: ... targets Phase 0 (scaffold)."

Fix: correct the duplicate claim (blocking). Updating the ledger row and current-state paragraph can reasonably ride on the lead's merge commit, but it should not be skipped.

### 5. ADVISORY -- the golden test cannot catch malformed normalization

`freshbooks/internal/inventory/inventory_test.go:508`

`TestLoadRealCollectionGolden` asserts the total, the per-folder counts, the `Single Tax` disambiguation, and byte-for-byte re-emission. All useful, none of it structural -- which is why finding 1 shipped green and got committed into the golden file. Byte-stability only proves the output is *reproducible*, not *correct*.

Add one cheap invariant subtest over all 213 entries: every `PathTemplate` starts with `/`; every `Host` is non-empty; no `PathTemplate` contains `freshbooks.com` or `{{`; every `Family` is one of the nine constants; every `Key` is unique. That single subtest would have failed on both Estimates entries, and it guards every future regeneration.

### 6. ADVISORY -- `normalizeQueryString` swallows unescape errors into empty names

`freshbooks/internal/inventory/inventory.go:247,250`

```go
name, _ := url.QueryUnescape(kv[0])
...
value, _ = url.QueryUnescape(kv[1])
```

On a malformed escape (`%zz`), `QueryUnescape` returns `("", err)` and the entry silently becomes a query parameter with an empty name. The rest of this file is admirably loud about bad input (`buildEntry` errors on a missing folder, `dedupe` errors on a colliding key, `normalizeURL` wraps parse failures) -- this is the one place that quietly drops data. Fall back to the raw segment when the error is non-nil, or propagate it.

### 7. ADVISORY -- three ledger endpoints land in `family: "unknown"`

`freshbooks/internal/inventory/inventory.go:305`

`classifyFamily` implements the spec's prefix list literally, so these fall through:

```
/accounting/ledger_accounts/types
/accounting/ledger_accounts/sub_types
/accounting/ledger_accounts/sub_types/{subtypeId}
```

They are plainly ledger-family (they sit in `Accounting/Accounts` and are the type taxonomy for the ledger accounts that `/accounting/businesses/{businessUuid}/ledger_accounts/` returns). The implementation matches the spec; the spec's rule list is incomplete. Since `family` drives the Phase 2 batch split, silently bucketing them as `unknown` is a small planning hazard.

Either add a `/accounting/ledger_accounts/` -> `ledger` case (with the usual `STATE AS OF` callout in the affected spec section), or leave them and note in the spec that `unknown` is expected for these three. The remaining four `unknown` entries -- the two Tokenization requests on `paid.freshbooks.com` and the two Estimates entries from finding 1 -- are separate: the Tokenization pair is genuinely a different service and correctly `unknown`, though `paid.freshbooks.com` is a non-`api` host that gets none of the flagging `my.freshbooks.com` receives.

### 8. ADVISORY -- the surviving plain key in a disambiguated pair is method-implicit and collection-order-dependent

`freshbooks/internal/inventory/inventory.go:409-412`

`dedupe` gives the *first* occurrence the plain key and suffixes only later ones. So `Expenses/Single Tax` means "the GET" purely because GET appears first in the collection JSON, while `Expenses/Single Tax (DELETE)` is explicit. Two consequences: the parity contract's keys are asymmetric (a Phase 2 implementer writing `// inventory: Expenses/Single Tax` has to know which verb that is), and if FreshBooks ever reorders the collection, the plain key silently changes meaning while remaining byte-stable-looking in a diff.

Suffixing *both* sides of a collision (`... (GET)` and `... (DELETE)`) makes keys self-describing and order-independent. Only four keys are affected. Worth doing before Phase 2 writes `// inventory:` comments against them, since changing it later means touching implementation comments.

### 9. ADVISORY -- `GORELEASER_CURRENT_TAG` will publish unprefixed releases that collide across modules

`.github/workflows/release.yml:101`

`GORELEASER_CURRENT_TAG: v${{ needs.guard.outputs.version }}` strips the module prefix so `{{.Version}}` renders cleanly -- but goreleaser also uses the current tag as the *release* it publishes to. Pushing `mcp/v0.1.0` will make goreleaser target a release named `v0.1.0`, not `mcp/v0.1.0`. Two follow-on problems: the release is detached from the tag that actually triggered the workflow, and `mcp/v0.1.0` and `cli/v0.1.0` both resolve to the same `v0.1.0` release -- the second one to ship overwrites or fails against the first.

The implementer correctly flagged the whole approach as unverified and deferred it to Phase 5, and the `.goreleaser.yaml` files self-document it, so this is not a Phase 0 blocker. Recording the specific failure mode here so the Phase 5 dry run has a concrete test case rather than a general "verify goreleaser." The likely fix is `release: { tag: <full prefixed tag> }` plus `name_template` for the archive names, or dropping goreleaser's release step and publishing artifacts with `gh release create` against the real tag.

### 10. ADVISORY -- `run()` in the MCP binary takes an `args` parameter it never reads

`mcp/cmd/freshbooks-mcp/main.go:17`

```go
func run(w io.Writer, args []string, v string) error {
```

`args` is unused. `revive`'s `unused-parameter` rule is not enabled, so lint stays green. The signature advertises argument handling that does not exist -- `freshbooks-mcp --help` prints the version. Either drop the parameter until Phase 3 needs it, or name it `_`.

### 11. ADVISORY -- smaller items

- `cli/internal/cmd/root.go:24` -- `root.CompletionOptions.DisableDefaultCmd = false` is cobra's default; the line is a no-op. Delete it or add a comment saying it is deliberate documentation of intent.
- `CHANGELOG.md:7-24` -- two separate `### Added` blocks under one `## [Unreleased]`, split by a `### Changed`. Keep a Changelog expects one block per type. `scripts/changelog-section.sh` still extracts it correctly (it prints everything between `## [` headings), so this is cosmetic -- merge the two `### Added` blocks.
- `mise.toml:5` -- `actionlint` is pinned but nothing consumes it: no `[tasks]` entry, no CI step, not in `scripts/check.sh`. The plan treated a clean `actionlint` run as an acceptance criterion; without a task it is a one-time manual check that will drift. Add `[tasks.actionlint]` and a step in `check.sh all`, or drop the pin.
- `freshbooks/internal/inventory/testdata/ignore.list` -- grouped by directive then sorted within each group, rather than globally sorted as the plan says. Deliberate and arguably more readable; calling it out so the deviation is on the record rather than rediscovered later.
- `freshbooks/internal/inventory/inventory.go:225` -- `stripWhitespace` runs over the whole raw URL, so a string-form query value like `search[name]=John Doe` becomes `JohnDoe`, while the same value in object form (taken from `u.Query` verbatim) keeps its space. Spec rule 1 asks for exactly this whitespace strip, so the implementation is compliant; noting the string/object asymmetry in case a Phase 2 reader trusts `query[].value` to be faithful.
- `README.md:21-26` -- "Until then, build from source:" is followed by `go install ...@latest`, which installs from the module proxy, not from a local clone, and pre-tag resolves to a default-branch pseudo-version (and fails outright until the repo is public). Either reword to "install the latest commit" or give the actual from-source instructions (`git clone && mise run build`).
- `docs/agentic-transformation.md:15` -- "14 top-level folders (22 of them nested one level deeper" parses as 22 of the 14 folders. Should read "with 22 subfolders nested one level deeper."

---

## What is solid (no action)

- Module and workspace layout, `go.work`, per-module `go.mod` with `go 1.26`, tidy dependency sets matching the locked design (lib stdlib-only, cli adds cobra + pflag only).
- Every exported identifier carries an accurate doc comment; `revive`'s `exported` rule is on and clean. `freshbooks/doc.go` is a genuinely good package overview and correctly states the two-family / two-ID-type model from spec section 3 and `CLAUDE.md`.
- Inventory test suite: 38 test functions, table-driven with `[happy] [sad] [edge] [corner]` tags, no `t.Skip`, no vacuous asserts, sad paths covered (bad URL, missing folder, unmarshal failure, unwritable output, all seven check failure classes). `newFixtureModule` correctly isolates `go list` in a temp module so the real source tree is never scanned.
- Every check failure class from the plan is implemented and tested: uncovered, double-covered, stale, unknown-comment, unknown-ignore-key, key-listed-twice, key-in-both-lists.
- `Entry` JSON field order matches the spec exactly; slices are initialized to empty rather than nil so `inventory.json` has no `null` arrays; `TestEntryJSONHasNoNullSlices` and `TestWriteJSONIsSortedAndStable` guard the byte-stability requirement.
- Normalization rules 1, 2, 4, and 6 are correct and individually table-tested. Rule 3 is correct. Rule 5 is correct as specified (see finding 7 for the spec gap).
- `check.sh` / `mise.toml` task wiring is clean, the dirty-tree banner and exit-1 behave as specified, and per-module invocation (`mise run check -- freshbooks`) works.
- Workflows: job names are exactly `lib` / `mcp` / `cli` as the branch-protection script requires; `workflow_call` wiring is correct; the release guard's semver check, module allow-list, ancestor-of-main check, and changelog extraction all do what spec section 8 asks.
- goreleaser configs use the correct 2.x keys (`formats` list, `archives[].ids`, `sboms`).
- `docs/building.md` and `docs/agentic-transformation.md` are accurate and genuinely useful (modulo the `main.go` scope wording in finding 3 and the two small items in finding 11). Doc stubs have real headings with per-phase ownership.
- Public-repo hygiene: no vault item names, internal hostnames, IPs, or personal correspondents anywhere in the diff. `redaction-check.sh` scans full staged file content rather than only the diff hunks, which is stricter than the plan asked for, and never echoes the matched term.

## Suggested triage

Fix in one commit before re-gating: findings 1 (+5, same fix session), 2, 3, 4. Findings 6, 8, and 10 are cheap and worth folding into the same commit. Findings 7, 9, and 11 can be deferred to their owning phases if the lead prefers, provided 9 is recorded as a Phase 5 test case.
