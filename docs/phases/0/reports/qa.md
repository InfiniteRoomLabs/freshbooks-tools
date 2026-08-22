# Phase 0 (scaffold) -- QA / reality-check lane

Subject: branch `phase-0/scaffold` in `<repo root>`, 11 commits ahead of `main`, tip `c09331d` (`fix(scaffold): apply the review-gate findings`). Lane: QA (the only lane that runs the gate). Default verdict is NEEDS WORK.

## Verdict: **PASS**

The gate is green on a clean tree, every GOAL.md stage 2 deliverable exists and meets its acceptance criterion, all 30 Accepted triage rows are verifiably applied, none of the 6 Deferred rows leaked in, and an independent hand-derivation of 9 inventory entries plus all 14 per-folder leaf counts from the raw Postman JSON matches `testdata/inventory.json` exactly. Five ADVISORY findings, no BLOCKING.

---

## 1. Gate, on the current tree

```
$ mise run check                      # exit 0
== fmt-check: freshbooks ==  == vet: freshbooks ==  == lint: freshbooks ==
== test: freshbooks ==
ok  .../freshbooks                    (cached) coverage: [no statements]
ok  .../freshbooks/internal/inventory (cached) coverage: 92.2% of statements
== cover: freshbooks ==
coverage-gate: .../freshbooks/coverage.out total = 92.2% (floor 90%)
coverage-gate: PASS
== vuln: freshbooks ==   No vulnerabilities found.
== inventory-check: freshbooks ==
== fmt-check: mcp == ... == cover: mcp ==
coverage-gate: .../mcp/coverage.out total = 100.0% (floor 90%)
coverage-gate: PASS
== vuln: mcp ==          No vulnerabilities found.
== fmt-check: cli == ... == cover: cli ==
coverage-gate: .../cli/coverage.out total = 100.0% (floor 90%)
coverage-gate: PASS
== vuln: cli ==          No vulnerabilities found.
== actionlint ==
== build ==   (12 binaries: {linux,darwin,windows} x {amd64,arm64} x {mcp,cli})
check.sh: all OK

$ git status --porcelain      # empty, before and after every command in this report
```

Green HEAD **and** clean tree. No `DIRTY TREE:` banner. Raw package numbers before the `cmd/*/main.go` filter: `mcp/cmd/freshbooks-mcp` 80.0%, `cli/cmd/freshbooks` 0.0%, `cli/internal/cmd` 100.0% -- the filter is doing what review-3 specified, and `freshbooks/internal/inventory/main.go` is still counted (30 profile lines survive the filter, 4 funcs in `go tool cover -func`).

Additional gate invocations, all exit 0:

```
$ rm -rf dist && mise run check -- freshbooks
== build ==
build: no buildable modules in the filter (freshbooks) -- skipping
check.sh: all OK
$ ls dist  ->  No such file or directory        # simplify-A confirmed: builds nothing
$ mise run actionlint                            # clean
$ mise run vuln                                  # No vulnerabilities found. x3
$ (cd freshbooks && go run ./internal/inventory -check ./...)
implemented 0, ignored 0, todo 213, uncovered 0, double-covered 0, stale 0, unknown 0
$ go version  ->  go1.26.6 linux/amd64           # mise.toml go = "1.26.6" (security-3)
```

## 2. GOAL.md stage 2 deliverables

| Deliverable | Acceptance | Verdict |
|---|---|---|
| `go.work` + 3 modules | `go 1.26`, uses `./freshbooks ./mcp ./cli`; module paths `github.com/InfiniteRoomLabs/freshbooks-tools/{freshbooks,mcp,cli}` per spec 2 | MET |
| `freshbooks/doc.go` + `Version` | `const Version = "0.0.0-dev"` at `doc.go:30`, package overview above it; `version_test.go` present | MET |
| `mcp/cmd/freshbooks-mcp/main.go` prints version | `main()` = one statement `os.Exit(run(os.Stdout, os.Stderr, version))`; logic in `run.go`, `version` var `-ldflags`-settable | MET |
| `mcp/internal/{config,server,tools}` | three `doc.go` placeholders | MET |
| `cli/cmd/freshbooks/main.go` cobra root + version + completion | `main()` = one statement `os.Exit(cmd.Run(...))`; `NewRootCmd` at `internal/cmd/root.go:16` adds `version`, cobra supplies `completion` (`TestCompletionCommand`) | MET |
| `cli/internal/{config,output,auth}` | three `doc.go` placeholders (+ real `internal/cmd`) | MET |
| Per-module `CHANGELOG.md`, Keep a Changelog, `[Unreleased]` | all 4 (root + 3 modules) present, correct header block | MET |
| >=1 test per module so coverage is measurable | freshbooks 37 `func Test`, mcp 1, cli 3 | MET |
| Inventory tool: parse + normalize + emit | 213 entries; re-emit is byte-identical to the committed file (`cmp` clean) | MET |
| Inventory tool: `-check` with ignore/todo list | exercised end-to-end, all 7 classes (below) | MET |
| Collection moved under `internal/inventory/testdata/` | present; `docs/freshbooks.postman_collection.json` gone | MET |
| Inventory package >= 90% | 92.2% | MET |
| `mise.toml` pins + tasks | `go 1.26.6`, `golangci-lint 2.13.1`, `goreleaser 2.17.1`, `actionlint 1.7.12`, `usage 6.0.0`; tasks `fmt-check vet lint test cover vuln actionlint inventory-check build docs check` | MET (`vuln`/`actionlint` added by triage) |
| `.golangci.yml` | v2 schema, `default: none` + errcheck/govet/staticcheck/revive(`exported`)/gosec/misspell, `exclusions.presets: []` (the v2 spelling of `exclude-use-default: false`) | MET |
| `scripts/{coverage-gate,changelog-section,branch-protection,redaction-check,build,check,docs}.sh` | all 7 present, `usage` shebangs on the arg-taking ones | MET |
| `.github/workflows/ci.yml` | `pull_request`, `push: main`, `workflow_call`; jobs named exactly `lib`/`mcp`/`cli`, `mcp`+`cli` `needs: lib`, each `mise run check -- <module>` | MET |
| `.github/workflows/release.yml` | tag triggers for all three prefixes; `guard` (strict semver regex, `merge-base --is-ancestor`, changelog section -> artifact) -> `ci` (`uses: ./.github/workflows/ci.yml`) -> `release` (goreleaser for mcp/cli, `gh release create` for freshbooks) | MET |
| `mcp/.goreleaser.yaml`, `cli/.goreleaser.yaml` | present; `monorepo:` removed (Pro-only) and replaced by `workdir` + `GORELEASER_CURRENT_TAG`, documented in-file | MET (documented deviation) |
| `.github/dependabot.yml` | gomod x3 + github-actions, all weekly | MET |
| `README.md` | what/why, module table, install (git clone + `mise run build`, `go install` under "Once tagged"), doc links, Contributing with the agent-ops marketplace note and the "redaction-check is optional" note | MET |
| Doc stubs with real headings | `getting-started` 6, `building` 8, `authentication` 6, `library` 6, `mcp` 6, `cli` 6 headings | MET |
| `docs/agentic-transformation.md` written for real | documenter.gw.postman.com pull with the real id/slug, no-codegen rationale, inventory tool + parity contract, work orders + gate, GOAL treadmill | MET |
| Root `CHANGELOG.md` `[Unreleased]` scaffold line | present, single merged `### Added` block | MET |
| **Acceptance:** check green, clean tree, `-check` passes, actionlint clean | see section 1 | MET |

## 3. Fidelity to the source, entry by entry (substituted for "fidelity to the docs")

No FreshBooks resources exist yet, so this is a hand-derivation from the raw Postman JSON using the `docs/phases/0/plan.md` section B rules, compared against `testdata/inventory.json`. I walked the collection independently (Python, recursive `item` descent) rather than reusing the tool's own parser.

**Per-folder leaf counts, re-derived from the raw collection:**

```
Invoices 50, Expenses 29, Settings 28, Projects 19, Accounting 19, Reports 15,
Clients 13, Estimates 8, Time Tracking 7, My Team 7, Tokenization 6,
Webhooks 5, Authorization 4, Uploader 3   ->  total 213
Nested subfolders: 22
```

Identical to the spec section 3 `STATE AS OF` callout, all 14 folders, and to the `folder` distribution in `inventory.json` (`My Team ` trailing space trimmed to `My Team`). 213 entries / 213 unique keys / 211 distinct pre-disambiguation keys / `sum(duplicates) == 213` / keys sorted ascending -- all as claimed.

**Nine entries derived by hand (all 6 required categories covered).** Every field below is my derivation; all nine matched `inventory.json` byte-for-byte on `key`, `folder`, `path`, `method`, `pathTemplate`, `host`, `family`, `query`, `duplicates`.

1. **`Estimates/Update Estimate`** (scheme-less URL -- the review-1 regression case). Raw: `api.freshbooks.com/accounting/account/{{accountId}}/estimates/estimates/{{estimateId}}`, no scheme. Rule: prepend `https://`, then rule 2 on both vars. Derived `PUT`, host `api.freshbooks.com`, path `/accounting/account/{accountId}/estimates/estimates/{estimateId}`, family `accounting`, query `[]`. **Match.**
2. **`Expenses/Single Tax (GET)`** and 3. **`Expenses/Single Tax (DELETE)`** (the Single Tax pair). Raw for both: `https://api.freshbooks.com/accounting/account/{{accountid}}/taxes/taxes/{{taxid}}`, same URL, different method. Rule 2 lowercase-spelling normalization gives `{accountId}`/`{taxId}`; rule 8 (triage) suffixes **both** sides. Derived path `/accounting/account/{accountId}/taxes/taxes/{taxId}`, family `accounting`, `duplicates` 1 each, no bare `Expenses/Single Tax` key. **Match** -- and the same holds for the second pair, `Settings/Items and Services/Single Tax (GET)`/`(DELETE)`, which I also checked. `grep` over all keys confirms no bare `Single Tax` key survives.
4. **`Settings/Businesses/Delete Business - Subscription`** (the `my.freshbooks.com` + nested-subfolder + hard-coded-ID case). Raw: `https://my.freshbooks.com/service/api/auth/api/v1/billing/account/0XaOw/subscription`. Rule 4 rewrites host to `api.freshbooks.com` and strips `/service/api`; rule 3 turns the non-variable segment after `/account/` into `{accountId}`; rule 4 also pins family `internal`, which must win over rule 5's `/auth/` -> `auth`. Derived `DELETE`, `/auth/api/v1/billing/account/{accountId}/subscription`, host `api.freshbooks.com`, family `internal`, folder `Settings`, path `["Businesses"]`. **Match** (family is `internal`, not `auth` -- precedence correct).
5. **`Invoices/Items and Services/List Items Filtered by SKU`** (query params + percent-escaping + nested subfolder). Raw: `...items/items?search%5Bsku%5D={{sku}}`. Derived path `/accounting/account/{accountId}/items/items` (query stripped off the template), query `[{name: "search[sku]", value: "{sku}", description: ""}]` -- `%5B`/`%5D` unescaped in the name, variable rewrite applied to the value. **Match.**
6. **`Tokenization/1a. [STRIPE] -  Get Publishable Key`** (Tokenization + hard-coded ID + name whitespace). Raw: `https://api.freshbooks.com/payments/account/E86Qp/gateway`. Derived `/payments/account/{accountId}/gateway`, family `payments`, and the key retains the request name's internal double space (rule 1 trims only leading/trailing). **Match.**
7. **`Tokenization/1. [STRIPE] - Create Payment Method`** (third host). Raw host `paid.freshbooks.com`, path `/gateway/stripe/payment-method`, no rule-5 prefix hits. Derived family `unknown`, host preserved. **Match** -- and this is one of only two `unknown` entries in the whole inventory.
8. / 9. `Settings/Items and Services/Single Tax (GET)` / `(DELETE)` as noted above.

**Byte stability:** `go run ./internal/inventory -in testdata/freshbooks.postman_collection.json -out /tmp/regen.json` then `cmp` against the committed file -- identical.

**Family distribution** (sanity, from `inventory.json`): accounting 133, business 33, payments 13, auth 11, ledger 7, uploads 6, events 5, internal 3, unknown 2 = 213. The 3 `internal` keys are exactly the 3 `my.freshbooks.com` requests I found by grepping the raw collection -- no more, no fewer.

## 4. `-check` end to end (substituted for "seams", none exist yet)

Built the tool to `/tmp/qa-invtool`, created a throwaway module at `/tmp/qa-invcheck` (outside the repo; deleted afterwards) with a 4-key synthetic inventory, an ignore list (`Gamma` todo, `Delta` ignore) and one package carrying `// inventory:` comments. Results:

| Case | Output | Exit |
|---|---|---|
| pass | `implemented 2, ignored 1, todo 1, uncovered 0, double-covered 0, stale 0, unknown 0` | 0 |
| uncovered | `uncovered: Folder/Beta` | 1 |
| double-covered | `double-covered: Folder/Alpha` | 1 |
| stale (todo-listed AND implemented) | `stale: Folder/Gamma` | 1 |
| unknown key in the list | `ig.list: 1 key(s) not present in the inventory: Folder/Nope` | 1 |
| unknown key in a code comment | `unknown: Folder/Zeta` | 1 |
| key listed twice | `ig.list:4: key "Folder/Gamma" listed twice` | 1 |
| pass again after revert | as row 1 | 0 |

Every failure class fails loudly with a precise message. The scanner correctly skips `_test.go` (`check.go:166`) and `testdata/` dirs (`check.go:157`).

**The check is not vacuous against the real tree either:** appending `// inventory: Estimates/Update Estimate` to `freshbooks/doc.go` flipped the real run to `todo 212 ... stale 1 / stale: Estimates/Update Estimate`, exit 1. Reverted; tree clean.

## 5. Test quality

- No `t.Skip` anywhere. No committed `-run` filters. `-race` on every `go test` invocation in `check.sh`. `t.Run` names carry the `[happy] [sad] [edge] [corner]` tags the convention asks for.
- The golden test (`inventory_test.go:508`) asserts leaf count, per-folder counts against the spec callout, both Single Tax collisions, the review-5 well-formedness invariants over all 213 entries (`PathTemplate` leading `/`, non-empty `Host`, no `freshbooks.com` or `{{` in the template, `Family` in the constant set, key uniqueness), and byte-for-byte re-emission. That subtest is the one that would have caught review-1, and it is present.
- Error paths are covered rather than padded: `TestURLUnmarshalJSONInvalid`, `TestNormalizeBadURL`, `TestNormalizeMissingTopLevelFolder`, `TestNormalizeConflictingDuplicatesFailWhenDisambiguationCollides`, `TestNormalizeQueryUnescapeFallback` (`%zz`), table-driven `TestLoadIgnoreListErrors`, and the `mcp` stdout-write-failure branch via a fake writer.
- Coverage is honest, not gamed: the only exclusion is `/cmd/[^/]*/main\.go:` (verified by reading the filter and by confirming `internal/inventory/main.go` survives it), and an empty filtered profile is a hard fail -- see finding evidence in section 7.

## 6. Parity / ignore list

`mise run inventory-check` passes. `testdata/ignore.list`: **0** `//go:inventory-ignore`, **213** `//go:inventory-todo`, 213 = the full inventory. Every non-comment line carries a ` -- ` reason (`grep -vE '^\s*(#|$)' | grep -vF ' -- '` returns nothing). The 4 `Authorization/*` keys are `phase-1`; everything else `phase-2`. The three former `my.freshbooks.com` entries (lines 18, 143, 176) each carry `-- phase-2 (collection lists my.freshbooks.com; verify public host live)` exactly as review-2 required, and the file header explains why `ignore` is empty.

## 7. Triage compliance

**Accepted -- all verified applied** (evidence in parentheses):

review-1 (`inventory.go:185-187`) - review-2 (`ignore.list` 0/213 + header + `agentic-transformation.md:38`) - review-3+simplify-5 (`coverage-gate.sh` single `grep -v '/cmd/[^/]*/main\.go:'`, empty profile -> `exit 1`, both `main()`s one statement, `building.md:20` matches) - review-4 (`progress.md` bullet now states the GET/DELETE collision + 213 keys) - review-5 (`inventory_test.go:562`) - review-6 (`inventory.go:251-262`) - review-7 (`inventory.go:321` + spec section 3 callout at line 52) - review-8 (`dedupe` at `inventory.go:415`, two-pass grouping, both sides suffixed) - review-10/simplify-14 (`run(stdout, stderr io.Writer, v string) int`) - review-11 (`DisableDefaultCmd` gone from all code, single `### Added` in root changelog, README `git clone`/`mise run build` first with `go install` under "Once tagged", `agentic-transformation.md:15` "with 22 subfolders nested one level deeper") - security-1 (`ci.yml:9-10` top-level `contents: read`; `release.yml` has no workflow-level block, `guard`/`ci` `contents: read`, `release` `contents: write`) - security-2 (`grep -rn "<home>"` over all tracked files returns nothing) - security-3 (`go = "1.26.6"`, `vuln` task, wired after `cover` in `check.sh`) - security-4 (`jdx/mise-action@3c2e0cf82a5b2e5249f0d3635a4d83d0ae861518 # v4.2.5` in both workflows, 4 occurrences) - security-5 word-boundary half (`redaction-check.sh` `short_term_threshold=8`, `grep -qiE "\b..\b"` below it, `grep -qiF` at or above) - security-6 (`mise run actionlint` task + `run_actionlint` called once in the `all` path) - security-8 (`-F "required_status_checks[strict]=true"`) - simplify-1,2,3,4,6,7,8 (`dedupe` two-map rewrite; `loadIgnoreList` single alternation + `into`/`other` swap; `scanFile`/`scanPackages` return `[]string`, `implementation` struct gone; `pkgDir` rename; `build.sh` hoists `git describe` above the loop; `check.sh` single `steps=(...)` array; `content=$(git show ":$file") || continue`) - simplify-9,10,11,12 (`TestLoadIgnoreListErrors`, `mustCheck`, `oneRequest`/`clientsList`, `mustReadFile`) - simplify-A (verified live: `mise run check -- freshbooks` builds nothing, no `dist/`).

**Deferred -- all verified NOT applied:** review-9 (`GORELEASER_CURRENT_TAG: v${{ needs.guard.outputs.version }}` still at `release.yml:104`) - security-5 commit-message half (`redaction-check.sh` scans `git diff --cached --name-only` only) - security-9 (no `mise.lock` in the tree; `build.sh` still interpolates `$version` from `git describe` unquoted into `-ldflags`) - simplify-13/15/B (not applied).

**Additional targeted verifications requested by the work order:**

- `scripts/coverage-gate.sh` hard-fails on an empty filtered profile. Constructed two temp profiles: `mode: atomic` + one `cmd/foo/main.go` line -> `coverage-gate: ... has no measurable statements outside cmd/*/main.go`, exit 1; `mode: atomic` alone -> same, exit 1. A zeroed-out copy of the real `freshbooks/coverage.out` -> `total = 0.0% (floor 90%)`, `FAIL`, exit 1. Missing file -> exit 1. Temp files deleted.
- `scripts/redaction-check.sh` exits 0 with nothing staged (`redaction-check: clean`). Staged a synthetic file containing this repo's real absolute path -> `redaction-check: possible leak in .qa-redaction-probe.txt (term #11)` and `(term #13)`, exit 1, and it prints term **indices**, never the terms. Unstaged with `git rm --cached`, file deleted, `git status --porcelain` empty. The real path was never written into any file left behind, and does not appear in this report.
- Both workflows carry the `permissions:` blocks security-1 requires; `jdx/mise-action` SHA-pinned in all 4 uses; `mise.toml` pins `go = "1.26.6"` and `go version` confirms 1.26.6 is what actually runs.
- `mcp/cmd/freshbooks-mcp/main.go` and `cli/cmd/freshbooks/main.go` each contain exactly one statement in `main()`.
- ASCII-only: `grep -rlP '[^\x00-\x7F]'` over `*.md *.go *.sh *.toml *.yml *.yaml` (excluding `.git/`) returns nothing. Note this glob set naturally excludes the Postman JSON and `inventory.json`, which are `.json`.
- `git status --porcelain` is empty at the end of this lane.

## 8. Findings

### 1. ADVISORY -- `docs/phases/0/reports/implementer.md:103` still describes the *rejected* coverage-gate design as current

Deviation #4 in the implementer report says, in the present tense, that `coverage-gate.sh` "now excludes files literally named `main.go` ... by filename, not directory" and that an empty filtered profile is treated as "a vacuous pass rather than reporting a nonsensical 0%". Triage review-3 rejected exactly that (`the implementer's deviation #4 ... is rejected per review-3`) and the shipped script does the opposite on both counts.

- Expected: the paragraph marked superseded, or rewritten to describe the `/cmd/[^/]*/main\.go:` scope and the hard failure.
- Observed: an unqualified present-tense claim. The later "Fix commit" table (same file, review-3 row) contradicts it correctly, so the file argues with itself.
- Impact: phase artifact only, no code effect -- but this is the document Phase 1 reads to understand what Phase 0 shipped.

### 2. ADVISORY -- `scripts/changelog-section.sh:26` fails silently

`awk`'s `END { if (!found) exit 1 }` exits non-zero with nothing on stderr.

- Reproduced: `./scripts/changelog-section.sh freshbooks 0.1.0` -> no output at all, exit 1.
- Expected: something like `changelog-section: no "## [0.1.0]" section in freshbooks/CHANGELOG.md`.
- Impact: `release.yml:57` runs this as `scripts/changelog-section.sh <module> <version> > /tmp/release-notes.md`. A missing section aborts the release with an empty artifact and a log that says nothing about why -- exactly the guard whose failure most needs to be legible. Cheap fix, Phase 5 territory.

### 3. ADVISORY -- two factual errors in the work order `docs/phases/0/plan.md` were silently overtaken by reality

- Section B rule 6: "Query params come from the Postman `url.query` array." The real collection has **zero** `url.query` arrays (I checked all 213 requests). All 25 query-bearing requests carry the query in the raw URL string, and the tool correctly parses it from there (`normalizeQueryString`).
- Gotchas: "~6 `my.freshbooks.com/service/api/...` requests". There are exactly **3**.
- Expected, per CLAUDE.md's inferred-vs-confirmed rule: implement reality (done, correctly) *and* record the divergence. Neither divergence is noted in the implementer report's deviations list or anywhere else.
- Impact: low -- `plan.md` is a spent work order, not the spec, and the spec's own callout never claimed 6. Flagged so Phase 1's plan does not inherit the `url.query` assumption.

### 4. ADVISORY -- the `-tags integration` test pass is currently a no-op that doubles test time

`scripts/check.sh` `run_test` runs `go test -race -tags integration ./...` after the coverage run, but no file in the repo carries `//go:build integration` (`grep -rln 'build integration'` returns nothing). The second pass just re-runs the same untagged tests.

- Expected (spec 8.1): the tagged pass exercises cross-package seams.
- Observed: no seams exist in Phase 0, so this is correct-but-empty rather than wrong. Not a Phase 0 deliverable gap -- GOAL.md stage 2 does not ask for integration tests.
- Impact: wasted CI minutes now; becomes a real gap if Phase 1 lands the refresh-rotation seam without a tagged test. Worth an explicit acceptance line in the Phase 1 work order.

### 5. ADVISORY -- `freshbooks/internal/inventory/main.go:6` documents a flag order that silently ignores the flags

The doc comment shows:

```
go run ./internal/inventory -check ./... [-inventory <inventory.json>] [-ignore <ignore.list>]
```

Go's `flag` package stops parsing at the first non-flag argument, so with that literal ordering `-inventory` and `-ignore` land in `fs.Args()` as package names and the defaults are used instead.

- Reproduced: `/tmp/qa-invtool -check ./... -inventory inv.json -ignore ig.list` in a temp module -> `inventory: reading internal/inventory/testdata/inventory.json: no such file or directory`. Moving the flags before `./...` works.
- Impact: none on the shipped call sites (`mise run inventory-check` uses defaults). It would bite the first person who tries to point the check at a different inventory, and here it failed loudly only because the default path was absent -- inside the repo it would have silently checked against the wrong files.
- Fix: reorder the usage line to `-check -inventory <p> -ignore <p> ./...`, or call `fs.Parse` on the trailing args too.

## 9. Commands run

```
mise run check
mise run check -- freshbooks                     # after rm -rf dist
mise run actionlint
mise run vuln
(cd freshbooks && go run ./internal/inventory -check ./...)
(cd freshbooks && go run ./internal/inventory -in internal/inventory/testdata/freshbooks.postman_collection.json -out /tmp/qa-inv-regen.json)
cmp /tmp/qa-inv-regen.json freshbooks/internal/inventory/testdata/inventory.json
go tool cover -func=freshbooks/coverage.out | grep inventory/main.go
scripts/coverage-gate.sh 90 <4 synthetic temp profiles>
scripts/redaction-check.sh                        # empty index, and with a synthetic staged probe
git add -f .qa-redaction-probe.txt ; git rm --cached .qa-redaction-probe.txt ; rm .qa-redaction-probe.txt
go build -o /tmp/qa-invtool ./internal/inventory  # + 8 -check runs against a throwaway module
grep -rlP '[^\x00-\x7F]' --include='*.md' --include='*.go' --include='*.sh' --include='*.toml' --include='*.yml' --include='*.yaml' .
grep -rn '<home>' --exclude-dir=.git .
git log --oneline main..phase-0/scaffold ; git show --stat c09331d ; git status --porcelain
uv run python  # independent recursive walk of the Postman collection: leaf counts,
               # subfolder count, my.freshbooks.com requests, url.query survey,
               # and field-by-field comparison against inventory.json
```

Everything created during this lane (`/tmp/qa-invtool`, `/tmp/qa-invcheck/`, `/tmp/qa-covtest/`, `/tmp/qa-inv-regen.json`, `.qa-redaction-probe.txt`, `dist/`) was removed or is gitignored. No tracked file was modified; nothing was committed. Final `git status --porcelain`: empty.
