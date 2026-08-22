# Phase 0 -- lead triage of the review gate

Lane reports: `reports/{code-review,simplify,security}.md` (QA runs after the fix). One fix commit: `fix(scaffold): apply the review-gate findings`. Items are numbered `<lane>-<finding>`; the decision is the lead's.

## Accepted (goes in the fix commit)

| Item | Decision / exact change |
|---|---|
| review-1 (BLOCKING) | `normalizeURL`: after `stripWhitespace`, `if !strings.Contains(raw, "://") { raw = "https://" + raw }`. Table case in `TestNormalizeStringURL`. Regenerate `testdata/inventory.json`. |
| review-2 (BLOCKING) | `ignore.list`: the three `internal`-family keys become `//go:inventory-todo <key> -- phase-2 (collection lists my.freshbooks.com; verify public host live)`. Keep the `//go:inventory-ignore` grammar and tests; the file just has 0 ignore entries now. Header comment and `docs/agentic-transformation.md` updated to match (ignore is for hard-coded-ID duplicates or endpoints confirmed dead, with evidence). |
| review-3 (BLOCKING) + simplify-5 | Coverage gate: filter scoped to `/cmd/[^/]+/main\.go:` only (matches `docs/building.md`); `freshbooks/internal/inventory/main.go` stays counted. Empty filtered profile is a **hard FAIL**, not a vacuous pass. To make that honest, `mcp/cmd/freshbooks-mcp/main.go` and `cli/cmd/freshbooks/main.go` contain only `func main() { os.Exit(run(...)) }` (one statement) and everything else moves to `run.go` in the same package (`mcp`) or stays in `internal/cmd` (`cli`), fully tested. Single `grep -v` instead of head/tail. `docs/building.md` wording matches. |
| review-4 (BLOCKING) | `docs/progress.md` line 28: replace the "collapse to one entry each" claim with the GET/DELETE name-collision fact and 213 keys. (Ledger row / next action are updated by the lead at ship.) |
| review-5 | Golden test gains an invariants subtest over all entries: `PathTemplate` starts with `/`, `Host` non-empty, no `freshbooks.com` or `{{` in `PathTemplate`, `Family` in the constant set, keys unique. |
| review-6 | `normalizeQueryString`: on `QueryUnescape` error fall back to the raw segment (name and value). Test case. |
| review-7 | `classifyFamily`: `/accounting/ledger_accounts/` -> `ledger`. Add a one-line `STATE AS OF 2026-08-22` note to the spec section 3 bullet that lists `/accounting/businesses/{business_uuid}/ledger_accounts/...` saying the type taxonomy lives at `/accounting/ledger_accounts/{types,sub_types}` and is ledger-family. Test case. |
| review-8 | `dedupe`: suffix BOTH sides of a name collision (`Single Tax (GET)` and `Single Tax (DELETE)`); plain key survives only when unique. Update the spec section 3 callout sentence, the golden test, `ignore.list`, regenerate `inventory.json`. |
| review-10 / simplify-14 | `mcp` `run` drops the unused `args` parameter. |
| review-11 | Delete the no-op `DisableDefaultCmd = false` line; merge the two `### Added` blocks in root `CHANGELOG.md`; README: replace the `go install ...@latest` "build from source" wording with `git clone` + `mise run build` (keep `go install` lines under a "once tagged" note); fix the "22 of them nested" sentence in `docs/agentic-transformation.md`. `ignore.list` grouping and `stripWhitespace` asymmetry: left as is (on record). |
| security-1 (BLOCKING) | `ci.yml`: top-level `permissions: contents: read`. `release.yml`: remove the workflow-level block; `guard` and `ci` jobs get `permissions: contents: read`, `release` gets `contents: write`. |
| security-2 (BLOCKING) | `docs/phases/0/plan.md`: replace the absolute repo path with `<repo root>`. (The lead scrubs history with `git filter-repo` after this commit, before any push.) |
| security-3 | `mise.toml`: `go = "1.26.6"`. New task `vuln` = `go -C <module> run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...` per module (pinned; newest as of 2026-08-22); wired into `check.sh` as the step after `cover`. |
| security-4 | `jdx/mise-action@v2` -> `jdx/mise-action@3c2e0cf82a5b2e5249f0d3635a4d83d0ae861518 # v4.2.5` in both workflows (v4 keeps the `install`/`cache` inputs). |
| security-5 | `redaction-check.sh`: terms shorter than 8 chars match with word boundaries (`grep -qiE "\b<escaped>\b"`), longer terms stay fixed-string substring. |
| security-6 / review-11 | `mise.toml` task `actionlint` = `actionlint .github/workflows/*.yml`; `check.sh` `all` path runs it once (not per module). |
| security-8 | `branch-protection.sh`: `-F "required_status_checks[strict]=true"` (typed boolean). |
| simplify-1,2,3,4,6,7,8 | Apply as sketched in `reports/simplify.md`. |
| simplify-9,10,11,12 | Apply (test helpers `mustCheck`, `oneRequest`/`clientsList`, `mustReadFile`; table-drive the four ignore-list error tests). |
| simplify-A (routed) | `check.sh` `all` path passes the module filter to `run_build`; `build.sh` accepts optional module names (default both). `mise run check -- freshbooks` therefore builds nothing. |

## Deferred (recorded, not in this commit)

- review-9: `GORELEASER_CURRENT_TAG` makes `mcp/v0.1.0` and `cli/v0.1.0` both publish to a release named `v0.1.0`. Phase 5 test case: goreleaser dry run per module; likely fix is `release.tag`/`name_template` or `gh release create` against the real tag. Noted in `docs/progress.md` at ship.
- security-5 (commit-message scan), security-9 (mise lockfile, build.sh tag whitespace): backlog.
- simplify-13, 15, B: not applied (neutral or touches gate arithmetic).

## Overrides

- None of the lane verdicts were overridden. The implementer's deviation #4 (filename-wide `main.go` exclusion) is rejected per review-3; deviations #1, #2, #3, #5, #6 are accepted.
