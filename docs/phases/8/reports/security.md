# Phase 8 security review -- `phase-8/converge`

Scope: `git diff main...phase-8/converge`, 6 commits `ccccafc..abbe407`, 39 files, 806 added lines. Read-only: no edits, no commits, no gate, no builds, no network. Scratch copies of `scripts/redaction-check.sh` and a throwaway git repo were created under `/tmp` for negative tests and deleted afterwards; the repo index and working tree are untouched (`git status --porcelain` shows only the other lanes' untracked reports).

## Verdict: **BLOCK**

One blocking finding (A1). It is not a leak -- the branch content is clean on every sweep I ran, including a corrected copy of the script. It is a **broken security control**: the `--range` mode this phase added, whose entire purpose was Phase 7 security A9 (give the gate lane a redaction check it can run against a branch diff), can never report a term-list hit. Shipping a control that always passes is worse than not having it, because the next phase will trust it. The fix is one character.

## Findings

### A1 -- BLOCKING -- `--range` mode's term-list scan silently scans nothing

`scripts/redaction-check.sh:143`

```
content=$(git diff "$usage_range" -U0 -- "$file" | grep -E '^\+' | grep -v '^\+\+\+' | sed 's/^\+//') || true
```

The second `grep` has no `-E`, so it is a **basic** regular expression, and in GNU BRE `\+` is the one-or-more operator, not a literal plus. `^\+\+\+` therefore matches any line beginning with one or more `+` -- which is every added line -- and `-v` discards all of them. `content` is always the empty string, so `scan_terms "$file" ""` compares 18 terms against nothing and can never set `found`.

Evidence, replicating the line verbatim against a real changed file:

```
$ f=freshbooks/expenses.go
$ content=$(git diff main..phase-8/converge -U0 -- "$f" | grep -E '^\+' | grep -v '^\+\+\+'   | sed 's/^\+//'); echo ${#content}
0
$ c2=$(git diff main..phase-8/converge -U0 -- "$f"      | grep -E '^\+' | grep -vE '^\+\+\+'  | sed 's/^\+//'); echo ${#c2}
3195
```

Isolated demonstration of the BRE semantics:

```
$ printf '+++ b/x\n+hello\n-bye\n' | grep -v '^\+\+\+'
-bye                      # "+hello" was dropped too
```

End-to-end A/B in a throwaway git repo (`/tmp`, since deleted) with a copy of the script, a file at `freshbooks/testdata/seed/x.json` carrying two planted terms from the live resolver list (a 5-char term that takes the word-boundary path and a 24-char term that takes the fixed-string path -- values not reproduced here) plus a planted 7-digit integer:

```
$ ./redaction-check.sh                       # staged/index mode, same content staged
redaction-check: possible leak in freshbooks/testdata/seed/x.json (term #1)
redaction-check: possible leak in freshbooks/testdata/seed/x.json (term #10)
redaction-check: unallowlisted 6+-digit number 1825574 in freshbooks/testdata/seed/x.json:4
exit=1

$ ./redaction-check.sh --range main..probe   # same content, range mode
redaction-check: unallowlisted 6+-digit number 1825574 in freshbooks/testdata/seed/x.json:4
exit=1
```

**The two modes do not agree.** Staged mode catches both planted terms; `--range` catches neither. The number sweep is unaffected because `added_lines_with_numbers` is pure bash (`[[ "$dline" == +* ]]`) and never touches `grep` -- only the term scan is dead.

The branch is genuinely clean: a `/tmp` copy of the script patched to `grep -vE` also reports `redaction-check: clean` on `main..phase-8/converge`. So no leak reached the diff; the control is simply inert.

**Fix.** Either `grep -vE '^\+\+\+'` on line 143, or -- preferable -- delete the grep pipeline entirely and build `content` from the already-correct `added_lines_with_numbers`, so both scans share one added-line extractor with no dependency on a grep dialect:

```
content=$(added_lines_with_numbers "$usage_range" "$file" | cut -f2-)
```

Please also add a regression check (a `scripts/redaction-check_test.sh`, or a probe recorded in the fix commit message) that a planted term fails in **both** modes -- the hand verification recorded in `docs/phases/8/reports/implementer.md:55` exercised only the number sweep positively, which is exactly why this survived.

### A2 -- ADVISORY -- a bad or unfetched range fails open

`scripts/redaction-check.sh:134-138`

`git diff` runs inside a process substitution (`done < <("${files_cmd[@]}")`), whose exit status is discarded, so a range that does not resolve produces an empty file list and a green result:

```
$ ./redaction-check.sh --range no-such-ref..probe
fatal: ambiguous argument 'no-such-ref..probe': unknown revision or path not in the working tree.
redaction-check: clean
exit=0
```

A gate lane that mistypes the range, or CI where the base ref was never fetched (shallow clone -- the common case in GitHub Actions), gets a pass. **Fix:** validate up front, e.g. `git rev-list --count "$usage_range" >/dev/null || { echo "redaction-check: unusable range" >&2; exit 2; }`.

### A3 -- ADVISORY -- UUID stripping can mask a 6+-digit id

`scripts/redaction-check.sh:103` deletes every `8-4-4-4-12` hex token before the digit sweep. Probes in the scratch repo, `--range` mode:

| planted | result |
|---|---|
| `12345678-0000-4000-8000-000000000001` | **not flagged** -- the 8-digit leading segment is masked |
| `18255740-1234-5678-9012-123456789012` | **not flagged** -- an entirely decimal UUID-shaped token is masked |
| `acct_1234567` | flagged (`1234567`) |
| `4550415 00000000-0000-4000-8000-000000000002` | flagged (`4550415`) |
| `4550415-0000-4000-8000-000000000003` | flagged (`4550415` and `000000000003`) -- 7-4-4-4-12 is not the UUID shape, so it is still swept |
| `9f8e7d6c-1a2b-4c3d-8e9f-0a1b2c3d4e5f` | not flagged (a genuine hex UUID -- intended) |

The hole is narrow: a real FreshBooks integer id does not naturally land inside a UUID-shaped token. But the stated justification for the blanket exemption is that "UUID redaction is a separate concern from this sweep (the term-list scan / manual review)" -- and A1 is precisely that scan being inert in the mode the gate uses. The two defects cover for each other. **Fix:** strip only tokens where at least one segment contains a non-decimal hex digit, or exempt by value (`^0+$` segments, this repo's actual convention) rather than by shape.

### A4 -- ADVISORY -- allowlist comment is inaccurate

`scripts/redaction-check.sh:50` annotates `4242424` as "the Stripe test-card digits". The Stripe test card is 16 digits (`4242` repeated four times); the `case` matches whole strings, so a real 16-digit run would correctly fail the sweep, not be allowlisted. `4242424` is in fact this repo's synthetic `identity_id` (`freshbooks/testdata/time_entries/list.json` and siblings). Comment-only; no behaviour change needed. The regexes themselves are correctly anchored and narrow -- `^700[0-9]{2}$` admits exactly 70000-70099, `^0+$` admits only all-zero runs, and the `case` literals are whole-string matches. Confirmed positively: `70023`, `8675309`, `000000000`, and `00000000-0000-4000-8000-000000000123` all pass while `1825574` fails with the correct `file:line`.

### A5 -- ADVISORY -- the sweep's path filter excludes the fixtures this phase re-seeded

`scripts/redaction-check.sh:90-93` restricts the number sweep to `freshbooks/testdata/seed/*`. D2 re-seeded `freshbooks/testdata/accounting/expenses_{get,list}.json` and `freshbooks/testdata/ledger_accounts/{list,get,create,update}.json` **from** those captures -- into paths the sweep does not cover. Phase 7 A1 was exactly a capture-derived row id sitting in a fixture, so the derived copies deserve the same sweep as the source. (This branch touches no `seed/` file at all, so the sweep had zero files to scan on `main..phase-8/converge`; the "clean" result recorded in the implementer report is vacuous for the number check.)

I verified the re-seeded values by hand and they are clean -- see the PASS section. **Fix:** widen the path filter to `freshbooks/testdata/*`, or add the re-seed targets explicitly.

### A6 -- ADVISORY -- a real account timestamp is copied into a Go doc comment and two fixtures

- `freshbooks/expenses.go` (`Expense.Version` doc comment) quotes `"2026-08-28 18:02:59.000000"` as the wire-format example. That is verbatim `freshbooks/testdata/seed/expenses/list.json:50` -- a real instant from the operator's account. Doc comments publish to pkg.go.dev, a wider surface than a fixture.
- `freshbooks/testdata/accounting/expenses_{get,list}.json` set `"version": "2026-08-22 04:32:55.000000"`, which is the account's real signup instant (the value Phase 7 A2 identified in `seed/accounting_clients_list.json`'s `signup_date`).

Same class as Phase 7 A2, which was accepted as ADVISORY on the Phase 1 precedent, so this is consistent rather than new -- but A2's proposed fix (round the fraction / the instant) would now have to chase three more copies. **Fix:** round both to `...T00:00:00.000000` in the comment and the fixtures; nothing asserts either value.

### A7 -- ADVISORY -- the new live test logs a real account field value

`freshbooks/live_conformance_test.go`, `TestLiveExpenseFields`:

```
t.Logf("expense %d: Billable=%v ExtInvoiceID=%d ExtSystemID=%d Version=%q", i, e.Billable, e.ExtInvoiceID, e.ExtSystemID, e.Version)
```

`Version` is a real per-expense timestamp. Every pre-existing `t.Logf` in this file on `main` logs a **count** or a **layout name** (`"decoded %d ledger account(s)"`, `"project %s CONFIRMS the zoneless ... layout"`) -- never a raw account value. The lead runs the live suite during QA and pastes its output into a report committed to a public repo, so this is a plausible path from live data to a public commit. The sibling ledger subtest gets this right (it logs only counts). **Fix:** log shape, not value -- `Version!=""`, or `len(e.Version)`.

### A8 -- ADVISORY -- the `usage` shebang makes the documented outside-contributor no-op unreachable

`scripts/redaction-check.sh:1`. `README.md:65` promises the check "is optional for outside contributors; it no-ops if the internal term list it looks for isn't present." With the new shebang, a contributor without `usage` on PATH never reaches the resolver check:

```
$ env -i PATH=/usr/bin:/bin HOME=$HOME scripts/redaction-check.sh --range main..phase-8/converge
/usr/bin/env: 'usage': No such file or directory
exit=127
```

Low priority: every other gate script (`branch-protection.sh`, `build.sh`, `changelog-section.sh`, `check.sh`, `coverage-gate.sh`) already carries the same shebang, so the repo already requires `usage`, and D4 explicitly decided to keep it. **Fix:** one README sentence, or a `command -v usage` guard.

### A9 -- ADVISORY -- the redaction check is not actually wired into CI

`docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md:357` states the check "greps staged content against the agent-ops redaction term list before commits (documented; **also run in CI**)". Nothing in `.github/`, `mise.toml`, or `scripts/check.sh` invokes it -- the only references in the repo are documentation. It is a manual, per-operator control. That is a large part of why A1 went unnoticed. **Fix:** wire it in (`scripts/redaction-check.sh --range origin/main..HEAD`, once A1 and A2 are fixed), or correct the spec claim.

## PASS

**Exit codes and the no-resolver graceful path are unchanged.** With a resolver-less `HOME`, both modes print `redaction-check: term list not available (optional for outside contributors)` and exit 0. Findings exit 1; clean exits 0; `--range` with no value is rejected by `usage` with exit 181 (non-zero, so CI still fails). Staged mode's term logic is byte-identical to `main`'s: same `short_term_threshold=8`, same `\b`-anchored escaped `grep -qiE` for short terms, same `grep -qiF` for long ones, same `git show ":$file"` full-content read.

**`usage` wiring is well-formed.** `#!/usr/bin/env -S usage bash` on line 1, one `#USAGE flag "--range <range>" help="..."` on line 9, and `usage lint scripts/redaction-check.sh` -> `No issues found.` (usage 6.0.0). Note for reproducibility: the flag only populates `$usage_range` when the script is executed through its shebang. Invoking it as `bash scripts/redaction-check.sh --range X..Y` silently falls back to staged mode -- worth a line in the report if any lane invokes it that way.

**Leak sweep over all 806 added lines is clean.** 0 email addresses, 0 `First Last` name pairs, 0 currency amounts, 0 JWT / 32+-hex / 64-hex / `sk_`/`pk_`/`acct_` / `Bearer` tokens. The only URLs are four `https://www.freshbooks.com/api/...` doc links. The only UUID is the synthetic `00000000-0000-4000-8000-000000000001`. Every 6+-digit run was traced to source:

- `5555550100`, `8675309`, `5550100100`, `4242424`, `999999999`, `1111111`, `000000`, `00000000`, `000000000001` -- allowlist literals and synthetic ids, appearing in the script's own allowlist and its comments.
- `668685`, `681477`, `485733`, `077973` -- fractional-second parts of `updated_at` in `freshbooks/testdata/ledger_accounts/*.json`, **pre-existing on `main`** (`git show main:freshbooks/testdata/ledger_accounts/get.json` carries `2022-09-22T08:47:04.668685Z` unchanged). The re-seed added keys, it did not rewrite these.
- `47634496`, `2976412` -- in the new `freshbooks/testdata/time_entries/list_with_totals.json`, reused verbatim from `main`'s `testdata/time_entries/list.json`, and both present in FreshBooks' own published `freshbooks.postman_collection.json`. FreshBooks example values, not operator data.
- `1825574`, `123456` -- appear only as probe values quoted in `docs/phases/8/reports/implementer.md:55` prose. `1825574` also originates in the published Postman collection and has been in `testdata/accounting/expenses_*.json` since Phase 2; it is in no `seed/` capture. Safe.

**No `seed/` capture is touched by this branch** (`git diff --name-only -- freshbooks/testdata/seed/` is empty).

**The re-seeded fixtures are synthetic and consistent with Phase 1 and the Phase 7 A1 fix.** `jea_id`/`jesa_id` use `70020`, `70021`, `70023`, `70024` -- exactly the `700NN` range A1's remediation mandated. `category_id` is `null` throughout. `accounting_systemid` is `"ACM123"`, `account_name`/`bank_name` are `""`, `bill_matches` is `[]`, the `*_id` externals are `null` or `0`. The new `list_with_totals.json` totals-breakdown elements use the established synthetic `identity_id: 4242424` / `client_id: 55001`.

**The new exposed surface is read-only and business-scoped.**

- Library (`freshbooks/time_entries.go`): `ListWithTotals` issues `s.client.do(ctx, http.MethodGet, timeEntriesPath(businessID), FamilyBusiness, nil, &resp, opts...)` -- GET, nil body, business family, the same path as `List`. One request, no new endpoint.
- MCP (`mcp/internal/tools/tools_time_entries.go:74-83`): `time_entries_list_with_totals` is annotated `hintRO` (`registry.go:241` -> `&mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(true)}`), same as `time_entries_list`. Input type is `timeEntriesListIn` -- the identical struct the list tool uses (`BizScope` + `listIn`), so it accepts no new input and echoes nothing new; the handler returns only the decoded response.
- CLI (`cli/internal/cmd/commands_time_entries.go:71-84`): `Class: ClassRO, Scope: ScopeBusiness, List: true`.
- Both are keyless (no inventory key) because they wrap the same wire request as the list variant, and both modules' `TestParityKeyCoverage` was relaxed from a single hardcoded exception to an allowed-set rather than dropped -- the parity contract still holds.

**`Expense`'s 14 new fields: stated plainly.** `account_name`, `bank_name`, `accountid`, `ext_accountid`, `accounting_systemid`, `bill_matches`, and `version` now appear in CLI and MCP output where they were previously discarded at decode. That is **correct behaviour, not a new disclosure**: the values were always in the FreshBooks wire response and are the caller's own account data, travelling from FreshBooks to the caller who authenticated for it. Nothing new leaves FreshBooks, and no new scope is required.

**`cli/internal/cmd/redaction_sweep_test.go` remains conceptually valid.** It asserts the fixture access token, refresh token, and client secret never reach stdout or stderr across five scenarios at `--log-level debug`, with positive markers so it cannot pass vacuously. The new fields cannot weaken it, because the debug log carries no response body: `freshbooks/transport.go:323` and `:345` log only `method`, `redactPath(endpoint)`, `status`, and `attempt`, at any level. The new fields therefore never enter a log line -- they only reach normal result output, which is what the caller asked for.

## Summary for triage

| Id | Tag | File | Fix |
|---|---|---|---|
| A1 | **BLOCKING** | `scripts/redaction-check.sh:143` | `grep -vE '^\+\+\+'`, or build `content` from `added_lines_with_numbers`; add a planted-term regression covering both modes |
| A2 | ADVISORY | `scripts/redaction-check.sh:134-138` | validate the range with `git rev-list --count` before scanning; non-zero exit on failure |
| A3 | ADVISORY | `scripts/redaction-check.sh:103` | strip only tokens with a non-decimal hex segment, or exempt by value not by shape |
| A4 | ADVISORY | `scripts/redaction-check.sh:50` | correct the `4242424` comment (it is the repo's `identity_id`, not a Stripe card) |
| A5 | ADVISORY | `scripts/redaction-check.sh:90-93` | widen the sweep path to `freshbooks/testdata/*` |
| A6 | ADVISORY | `freshbooks/expenses.go` Version comment; `testdata/accounting/expenses_{get,list}.json` | round the real instants to `00:00:00.000000` |
| A7 | ADVISORY | `freshbooks/live_conformance_test.go` `TestLiveExpenseFields` | log `Version`'s presence, not its value |
| A8 | ADVISORY | `scripts/redaction-check.sh:1`, `README.md:65` | note the `usage` prerequisite, or guard with `command -v usage` |
| A9 | ADVISORY | spec line 357; `.github/`, `mise.toml` | wire the check into CI, or correct the spec's "also run in CI" claim |

History scrub: **not needed.** No secret or operator-specific value reached the diff. A6's timestamps are copies of values already published on `main` and already accepted under Phase 7 A2; if the lead ever acts on A2, these copies join that cleanup rather than requiring their own.

Minimum to unblock: fix A1 and add its regression. A2 and A5 are cheap and belong in the same commit -- all three are one-line changes to the same script, and together they turn `--range` from a control that cannot fail into one that can.
