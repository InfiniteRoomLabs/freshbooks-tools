# Phase 8 gate -- fix report

Branch `phase-8/converge`, from `2bc0ff7`. Four checkpoint commits, `docs/phases/8/triage.md` F1-F20 applied as written; nothing under "Not applied" was touched. No live calls, no token, no push, no merge, no tags. The `-tags live` suite was never run, only compiled (`mise exec -- go vet -tags live ./freshbooks/` -- clean at every checkpoint that touched it).

| F | Source | Status | Commit |
|---|---|---|---|
| F1 | A1, S6 | done | `9f29402` |
| F2 | A1 | done | `9f29402` |
| F3 | A2 | done | `9f29402` |
| F4 | A3, R4 | done | `9f29402` |
| F5 | A5, R5 | done, with fallout -- see below | `9f29402`, `9182736`-probe adjustments in checkpoint 4 |
| F6 | S4, S5, S8, R3, A4 | done | `9f29402` |
| F7 | A8 | done | `9f29402` |
| F8 | A9 | done | `9f29402` |
| F9 | R2, S12 | done | `6f5b4d0` |
| F10 | R9, S12 | done | `6f5b4d0` |
| F11 | R1, R7 | done | `6f5b4d0` |
| F12 | S1 | done | `142f790` |
| F13 | S3 | done | `142f790` |
| F14 | R8 | done | `142f790` |
| F15 | R10 | done | (checkpoint 4) |
| F16 | A7 | done | (checkpoint 4) |
| F17 | A6 | done | (checkpoint 4) |
| F18 | R6 | done | (checkpoint 4) |
| F19 | R12 | done | (checkpoint 4) |
| F20 | S9 | done | (checkpoint 4) |

Nothing was skipped.

## Checkpoint 1 -- `9f29402` `fix(converge): apply the review-gate findings (F1-F8)`

`scripts/redaction-check.sh`, new `scripts/redaction-selftest.sh`, `scripts/check.sh`, `mise.toml`, `README.md`, the spec, root `CHANGELOG.md`.

- **F1 + F6.** Range mode's term-scan `content` is now `numbered_lines "$file" | cut -f2-` -- the one extractor both scans read, in both modes -- and the BRE pipeline is gone. `seed_number_allowed` is one anchored alternation, the digit sweep one `grep -oE '[0-9]{6,}'` pass, `scan_seed_numbers` declares `lineno`, `content`, `u` and `n` `local` (`content` was shadowing the main loop's), the dead `^700[0-9]{2}$` branch is gone with a comment saying 5-digit ids are below the 6-digit threshold, and the `4242424` comment now reads "the repo's synthetic identity_id".
- **F2.** `scripts/redaction-selftest.sh`: `usage` shebang, no arguments, `mktemp -d` scratch, a throwaway git repo per probe with a copy of the script under test, cleaned up on exit. 14 probes, each asserted in **both** modes. Wired into `scripts/check.sh` beside actionlint (repo-wide, once per gate) and as `mise run redaction-selftest`.
- **F3.** `git rev-list --count "$range" >/dev/null` up front; an unusable range prints `redaction-check: unusable range: <range>` and exits 2.
- **F4.** A UUID-shaped token is dropped before the sweep only if it carries a non-decimal hex digit or matches this repo's synthetic convention (`^0{8}-0{4}-[0-9]0{3}-[0-9]0{3}-0{4}[0-9]{8}$`). Both A3 probe rows are in the self-test.
- **F5.** Sweep path `freshbooks/testdata/*`.
- **F7.** README names the `usage` prerequisite (`mise install`) and points at the self-test.
- **F8.** Spec section 10 no longer claims the check runs in CI.

### Two things the order could not have known

1. **`usage lint` refuses a script with no `#USAGE` directive.** It falls back to parsing the file as KDL and errors out (the same happens today on `scripts/docs.sh`). Since the self-test takes no arguments, it carries `#USAGE bin` + `#USAGE about` nodes so `usage lint` has something to parse. Both scripts lint clean:

```
$ mise exec -- usage lint scripts/redaction-check.sh
No issues found.
$ mise exec -- usage lint scripts/redaction-selftest.sh
No issues found.
$ mise exec -- usage lint scripts/check.sh
No issues found.
```

2. **The self-test cannot arm the term scan without a term list, so it brings its own.** `redaction-check.sh` exits 0 with a notice when the private resolver is absent (F7 reaffirms that promise), which would make every probe vacuous on a machine or CI runner without it. The self-test therefore runs its stub probes under a fake `$HOME` carrying a stub resolver plus a `uv` shim on `PATH` that prints two nonsense terms -- so the term scan and the digit sweep are both genuinely exercised everywhere, offline, with no private data. Two consequences worth recording for the next phase:
   - `BASH_ENV` is dropped for those runs (`env -u BASH_ENV`). On this machine `~/.bash_env` re-activates mise's shims and re-prepends them to `PATH`, which shadowed the `uv` shim.
   - `usage` is normally reached through a mise shim that resolves the tool out of the real `$HOME`, so under the fake `$HOME` the shim errors out. The resolved binary (`mise which usage`) is symlinked into the stub `PATH` instead.
   - Two preflights guard the stub so a broken stub can never be mistaken for a passing probe: the resolved `uv` must be the shim, and the script must run to completion and report `clean` on an empty repo. Either failing exits 1 with a named reason.

   The private-resolver probes are separate and additive: when the real resolver is present the self-test plants one short (<8 chars, word-boundary path) and one long (>=8, fixed-string path) term from it and asserts both modes fail. Without it they are skipped with a `NOTICE` line. Term values are never printed, by the self-test or by `redaction-check.sh` (its message names only `term #N`).

### The self-test, as the gate runs it

```
== redaction-selftest ==
redaction-selftest: PASS 7-digit integer fails, naming file:line
redaction-selftest: PASS the sweep reaches re-seeded fixtures
redaction-selftest: PASS a real id wearing a synthetic uuid tail fails
redaction-selftest: PASS an entirely decimal uuid-shaped token fails
redaction-selftest: PASS the synthetic uuid convention passes
redaction-selftest: PASS a genuine hex uuid passes
redaction-selftest: PASS a microsecond timestamp passes
redaction-selftest: PASS a space-separated instant passes
redaction-selftest: PASS a 700NN synthetic id passes (below the sweep threshold)
redaction-selftest: PASS an allowlisted filler number passes
redaction-selftest: PASS an all-zero run passes
redaction-selftest: PASS a short stub term fails in both modes
redaction-selftest: PASS a long stub term fails in both modes, mid-identifier
redaction-selftest: PASS an unusable range exits 2
redaction-selftest: PASS a short private term fails in both modes
redaction-selftest: PASS a long private term fails in both modes
redaction-selftest: OK
```

### Hand verification, independent of the self-test

A throwaway git repo under `mktemp -d`, the fixed script copied in, one real short term and one real long term drawn from the live private resolver (values withheld; only their lengths are reported), a planted 7-digit integer, and the placeholders on the lines below it. Same content staged, then committed, so both modes see the identical bytes.

```
planted term lengths: short=5 long=12 (values withheld)

=== HAND-VERIFY 1: staged mode (planted term + planted 7-digit) ===
redaction-check: possible leak in freshbooks/testdata/seed/x.json (term #0)
redaction-check: possible leak in freshbooks/testdata/seed/x.json (term #1)
redaction-check: unallowlisted 6+-digit number 9182736 in freshbooks/testdata/seed/x.json:4
exit=1

=== HAND-VERIFY 2: --range mode, same content ===
redaction-check: possible leak in freshbooks/testdata/seed/x.json (term #0)
redaction-check: possible leak in freshbooks/testdata/seed/x.json (term #1)
redaction-check: unallowlisted 6+-digit number 9182736 in freshbooks/testdata/seed/x.json:4
exit=1
```

Byte-identical findings in both modes, same `file:line`, and the short (word-boundary) and long (fixed-string) term paths each fire. This is the A1 regression: before the fix, `--range` reported only the number.

The synthetic placeholders and the all-zero UUID pass, in both modes:

```
=== HAND-VERIFY 3: placeholders only, staged ===
redaction-check: clean
exit=0

=== HAND-VERIFY 4: placeholders only, --range ===
redaction-check: clean
exit=0

=== HAND-VERIFY 5: unusable range ===
redaction-check: unusable range: no-such-ref..HEAD
exit=2
```

(The file for 3/4 held `00000000-0000-4000-8000-000000000123`, `00000000-0000-0000-0000-000000000000`, `70023`, `8675309`, `5555550100`, `5550100100`, `999999999`, `1111111`, `000000000` and `2022-09-22T08:47:04.668685Z`.) The scratch repos were deleted afterwards; the working tree was never involved.

## F5's fallout -- read this before the next fixture edit

**F5 as written does not survive first contact, and two extra changes were needed to keep the branch's own commits green.** Recording the whole of it, because the residual is a landmine for the next phase.

Widening the sweep from `freshbooks/testdata/seed/*` to `freshbooks/testdata/*` is right on A5's reasoning, but the paths it newly covers are the Phase 2 fixtures, which are full of 6+-digit runs that are neither capture-derived nor allowlisted. Staging checkpoint 4's two `expenses_*.json` edits made the pre-commit check fail with ten findings, none of them a leak:

```
redaction-check: unallowlisted 6+-digit number 1825574 in freshbooks/testdata/accounting/expenses_get.json:5
redaction-check: unallowlisted 6+-digit number 2003174 in freshbooks/testdata/accounting/expenses_get.json:9
redaction-check: unallowlisted 6+-digit number 2003170 in freshbooks/testdata/accounting/expenses_get.json:16
redaction-check: unallowlisted 6+-digit number 900123 in freshbooks/testdata/accounting/expenses_get.json:34
...
```

and `--range main..HEAD` failed with seven more, in `ledger_accounts/*.json` and `time_entries/list_with_totals.json`.

Two fixes, both forced by F5 and both inside the same script:

1. **Instants are stripped before the digit sweep**, the way UUID-shaped tokens already are. `077973`, `668685`, `681477` and `485733` are the microsecond fields of `updated_at` in the ledger fixtures -- pre-existing on `main` (`git grep main -- freshbooks/testdata/ledger_accounts/` confirms each), and a fractional second is not an identifier. Whether an instant is itself too revealing is A6's concern (rounding), not this sweep's. Two self-test probes cover it.
2. **Six FreshBooks-published example ids joined the allowlist**, each traced before adding: `1825574`, `2003170`, `2003174`, `47634496` and `2976412` all appear in `freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json` (`grep -c` = 1 each), and `47634496`/`2976412` are additionally verbatim on `main` in `testdata/time_entries/list.json`; `900123` is a hand-written `transactionid` added in Phase 2's `9147ade`, before this repo had a live token at all. This is the same class the allowlist already carries for `8675309` and friends (QA Q4: pre-existing legitimate data), and the Phase 8 security lane had already traced all of them in its PASS section.

The self-test's planted integer moved from `1825574` to `9182736` for the obvious reason.

**The residual, which I did not fix and which the lead should rule on.** The sweep fires on the whole content of any *staged* file (that is what F1's single extractor gives staged mode), so widening the path means every Phase 2 fixture a future commit touches is re-swept in full. Measured against the current tree:

- 151 tracked files under `freshbooks/testdata/`
- **55 of them would report at least one finding if staged**, roughly 70 distinct numbers (`4054453`, `2003192`, `635972`, `45454545`, `1313600`, ...)
- **0 of them are under `seed/`** -- the original scope is entirely clean, and stays clean

So the control is correct on this branch and will cry wolf on the next fixture edit. Allowlisting 70 more literals would gut the sweep, so it needs a rule, not entries. The cheapest one that keeps F1 intact: in staged mode, skip a finding whose number already appears in `git show HEAD:$file` -- pre-existing content is not a newly introduced leak, which is exactly what a pre-commit control is for, and Phase 7 A1's capture-derived id would still have been caught the moment it was introduced. Five lines, no new extractor, and it subsumes both of the fixes above. I left it out because it changes a security control's semantics in a way the triage did not ask for, and two forced changes to that control already felt like the ceiling for an unattended fix agent. Recommend it as a Phase 9 item, or as a one-line follow-up here if the lead wants it in this gate.

## Checkpoint 2 -- `6f5b4d0` `fix(converge): apply the review-gate findings (F9-F11)`

- **F9.** `TimeEntriesPage.Totals` carries `json:"totals"`, with a doc note on why the embedded `Page[TimeEntry]` stays untagged. New `[corner]` subtest in `TestTimeEntriesListWithTotals` asserts the marshalled key set: `totals` present, `Totals` absent, `items` still promoted flat.
- **F10.** The command's `Short` is now "List time entries with logged/unbilled totals (totals in -o json or -o yaml only; table mode shows the entries)". `mise run docs` regenerated `docs/cli.md` (2 lines); the drift test passes. No output-layer change.
- **F11.** `docs/mcp.md:3` 168 -> 169 with the keyless caveat. `docs/phases/3/tools.md` rows 164-168 line references refreshed. (They moved again in checkpoint 3 -- see below.)

`freshbooks/CHANGELOG.md` and `cli/CHANGELOG.md`'s existing `[Unreleased]` entries were extended by one clause each (the `totals` key; the output-format caveat) rather than gaining new bullets.

## Checkpoint 3 -- `142f790` `fix(converge): apply the review-gate findings (F12-F14)`

- **F12.** One `const wantRegistrySize = 169` per module, beside its doc-table parser, read by that module's roundtrip size guard. `mcp/internal/tools/unit_test.go`'s `TestManifest` now asserts `len(Manifest()) == len(All)` and `mcp/cmd/freshbooks-mcp/run_test.go` asserts against `len(tools.All)` -- each testing its own seam instead of restating the frozen number. Six literals became two. Both parity tests still assert the 212-key total (`parity_test.go:241` mcp, `:244` cli) and doc-row/registry set equality in both directions; `keylessTools`/`keylessCommands` are untouched.
- **F13.** `timeEntriesListWithTotalsResponse` is gone; `timeEntriesListResponse` is the single struct (its `Meta` embedding `PageMeta` + `TimeEntryTotals`), `listWithTotals` does the one `do`/decode/`newPage`, and `list` is a projection returning `&p.Page`. `ListWithTotals` is one line. The `PageMeta` `newPage` receives is byte-identical -- `TimeEntryTotals`' four keys collide with none of `PageMeta`'s five -- and every `List`/`Search` test still passes unchanged.
- **F14.** `https://www.freshbooks.com/api/time_tracking` in all three live citations (the `TimeEntryTotals` doc comment, the spec 5.1 callout, `docs/progress.md` item 15). The phase reports and `docs/phases/8/plan.md` keep their original spellings as historical record.

**Where the triage was slightly off:** F11 refreshed `docs/phases/3/tools.md`'s line references in checkpoint 2, but F13 (checkpoint 3) moves the same methods again. The rows were re-checked after F13 and corrected in the same commit -- final values `time_entries.go:161/183/208/235/251` for rows 164-168 and `:173` for row 169. Anyone re-deriving them from the checkpoint-2 commit will see the intermediate numbers; only the branch tip is authoritative. Not a defect in the order, just an interaction between two of its items.

## Checkpoint 4 -- F15-F20 (this commit)

- **F15.** The live ledger subtest now reads the same body raw (`c.Do` into `[]map[string]json.RawMessage` under `data`) beside the typed call and, for each of `category_id`, `jea_id` and `jesa_id`, asserts the count of non-null wire occurrences equals the count of non-nil decoded pointers -- plus an up-front assertion that the two reads returned the same number of accounts. That is the falsifiable form of the property the subtest name claims; a wrong tag now fails it. It still cannot distinguish "always null" from "this account has none set", and the comment says so.
- **F16.** `TestLiveExpenseFields` logs `Version set=%v` (`e.Version != ""`), never the value.
- **F17.** The three copies this branch added are rounded to `...00:00:00.000000`: the `Expense.Version` doc example (`2026-08-28`) and the `version` values in `freshbooks/testdata/accounting/expenses_{get,list}.json` (`2026-08-22`). `freshbooks/expenses_test.go:49` asserts that value, so it was updated in lockstep. The seed captures and the Phase 1/7 precedent values are untouched, per the triage.
- **F18.** `docs/progress.md`: "the corpus carries both phone placeholders ... both are allowlisted", naming where each lives.
- **F19.** `listIn.reqOpts` (mcp) and `Invocation.ReqOpts` (cli) doc comments name `TimeEntries.ListWithTotals` alongside `JournalEntries.Details` and `JournalEntryAccounts.List`.
- **F20.** The two `TimeEntryTotals` field comments that restated the type doc are gone; the type doc and the json tags carry the evidence.

## Gate

Full `mise run check` after every checkpoint, green except the dirty-tree banner (expected -- it runs before the commit). Final run on the committed tree is clean.

```
coverage-gate: <repo root>/freshbooks/coverage.out total = 91.9% (floor 90%)
coverage-gate: <repo root>/mcp/coverage.out total = 92.1% (floor 90%)
coverage-gate: <repo root>/cli/coverage.out total = 91.6% (floor 90%)
```

Coverage is unchanged from the implementer's numbers in all three modules.

`scripts/redaction-check.sh` (staged mode, the fixed one) was run before each of the four commits: `redaction-check: clean`, exit 0 each time.
