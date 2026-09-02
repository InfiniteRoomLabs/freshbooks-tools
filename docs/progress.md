# Progress

Living status doc. Read first, update at every phase boundary. Last updated: 2026-09-02 (v0.1.0 tags shipped).

## Current state

- **v0.1.0 shipped 2026-09-02** (attended, then finished on Wes's go-ahead in-session): `freshbooks/v0.1.0` @ `0b63858` (notes-only release, proxy serves `freshbooks@v0.1.0`), `mcp/v0.1.0` and `cli/v0.1.0` @ `90cc8cd` (13 assets each: six archives, `checksums.txt`, six SPDX 2.3 SBOMs; checksums verified on a download; `go install ...@v0.1.0` prints `freshbooks-mcp v0.1.0` and `v0.1.0`; the CLI binary links no `cobra/doc`). The release workflow worked first time on all three tags; the lib release shipped untitled (fixed by `gh release edit`, and the workflow now passes `--title`). Repository tag-protection ruleset (backlog item 10) was NOT applied: still open.
- **Phase 5 (release hardening) shipped 2026-09-02**, `phase-5/release` -> `main` @ `6cbe6e4` (`--no-ff`). The release workflow was broken as written and is now proven: goreleaser builds with `--skip=publish,validate` and `gh release create --verify-tag` publishes onto the real prefixed tag (goreleaser OSS validates `GORELEASER_CURRENT_TAG` against `git describe --exact-match` and would have created the GitHub release on a new `v0.1.0` tag); `project_name` per module (archives no longer collide as `freshbooks-tools_*`), `GOWORK=off` + `CGO_ENABLED=0` builds, syft (mise-pinned `aqua:anchore/syft`) SPDX 2.3 SBOMs. CI: `actions/checkout` v7.0.1 and `jdx/mise-action` v4.3.0 pinned by SHA, `upload-artifact`/`download-artifact`/`goreleaser-action` removed, `persist-credentials: false` on the write-scoped job, `failglob` before publishing. Toolchain goreleaser 2.18.0, golangci-lint 2.13.2; dependencies refreshed (x/sys 0.47.0 closes GO-2026-5024). Folds: `cobra/doc` behind the `docsgen` build tag (`cli/internal/docsgen`, drift test still untagged and in the default gate), `version` falls back to `debug.ReadBuildInfo` so `go install ...@<ref>` prints the module version, `scripts/check.sh` ignores `docs/phases/*/reports/*`, `auth login --timeout` renamed `--login-timeout`, Phase 4 QA advisories Q4/Q12/Q15/Q17/Q20/Q21/Q22 folded. Docs pass complete (`getting-started.md` written for real; every command/env/identifier verified mechanically). Module changelogs reshaped into their 0.1.0 `### Added` sections, unwrapped, still under `## [Unreleased]`. Coverage at ship: freshbooks 91.8%, mcp 92.1%, cli 91.5%.
- **Gate outcome:** code review REQUEST CHANGES (R1 blocking: docs claimed CycloneDX SBOMs, syft's goreleaser default is SPDX), security PASS (A1-A5 advisory), simplification 3 apply-recommended -> one lead-landed fix commit (`6e41003`, 11 edits) -> QA PASS on the first round (goreleaser snapshot + real-tag dry runs from a clean clone, `go install @sha` outside the workspace, every doc example run, gate green). Q1/Q2 doc advisories folded in `5e3108f`.
- **Phase 4 (CLI) shipped 2026-09-02** (`3ae853d`): 168 registry commands, loopback PKCE login, contexts, `json|yaml|table|name`, `--dry-run`, `api`, exit codes, generated `docs/cli.md`. **Phase 3 (MCP server) shipped 2026-09-01** (`7ed5891`): 168 tools over 212 keys. **Phase 2 (lib resources) shipped 2026-09-01**; inventory `implemented 213, todo 0`. **Phase 1 (lib core) shipped 2026-08-23** (`98ea08c`). **Phase 0 (scaffold) shipped 2026-08-22** (`b3063ba`).
- Spec callouts: section 7 `STATE AS OF 2026-09-02` (Phase 4), section 6 (Phase 3), sections 3/5.1 (Phase 2). Phase 5 added none: the release design in section 8.4 still describes the intent correctly; `docs/building.md` carries the mechanics. **Everything API-facing is docs-confirmed, not live-confirmed.**

## Phase ledger

| Phase | Status | Branch / merge | Notes |
|---|---|---|---|
| 0 Scaffold | **SHIPPED 2026-08-22** | `phase-0/scaffold` -> `main` @ `b3063ba` | reports in `docs/phases/0/` |
| 1 Lib core | **SHIPPED 2026-08-23** | `phase-1/lib-core` -> `main` @ `98ea08c` | one converged blocker (`Token.String`); reports in `docs/phases/1/` |
| 2a-2d Lib resources | **SHIPPED 2026-09-01** | `phase-2/{a,b,c,d}` merged sequentially | 213/213 keys; reports in `docs/phases/2/` |
| 3 MCP | **SHIPPED 2026-09-01** | `phase-3/mcp` -> `main` @ `7ed5891` | review 1 + security 1 blocking -> 1 fix commit -> QA PASS; reports in `docs/phases/3/` |
| 4 CLI | **SHIPPED 2026-09-02** | `phase-4/cli` -> `main` @ `3ae853d` | review 10 + security 3 blocking -> F1-F30 (checkpointed) -> QA NEEDS WORK x2 -> PASS; reports in `docs/phases/4/` |
| 5 Release hardening | **SHIPPED 2026-09-02** | `phase-5/release` -> `main` @ `6cbe6e4` | stage 1 found the release workflow could not ship a prefixed tag; review 1 blocking -> 1 lead-landed fix -> QA PASS first round; reports in `docs/phases/5/` |
| 6 v0.1.0 tags | **SHIPPED 2026-09-02** | `freshbooks/v0.1.0` @ `0b63858`, `mcp/v0.1.0` + `cli/v0.1.0` @ `90cc8cd` | Release workflow green on all three; assets, checksums, SBOMs, and `go install` verified |
| 7 Live conformance | next (**attended**) | `phase-7/live` | needs FreshBooks credentials; upgrades every INFERRED/docs-only fact |

## Discoveries (Phase 5)

- 2026-09-02: goreleaser OSS 2.18.0 has no `monorepo` config (Pro-only still). With `GORELEASER_CURRENT_TAG=v0.1.0` on a commit tagged `mcp/v0.1.0`, `validate()` runs `git describe --exact-match --match v0.1.0` and fails ("tag v0.1.0 was not made against commit"); and the GitHub release pipe creates the release on `ctx.Git.CurrentTag`, i.e. it would have minted a new `v0.1.0` tag. `--skip=publish,validate` plus `gh release create --verify-tag <real tag> dist/...` is the working shape (verified from source and in a scratch clone).
- 2026-09-02: goreleaser's `project_name` defaults to the git remote's repository name, so both modules produced `freshbooks-tools_0.1.0_*` archives until `project_name` was set per module. SBOM generation shells out to `syft`, which nothing had installed; the default SBOM format is SPDX 2.3 JSON, not CycloneDX.
- 2026-09-02: git 2.43's pathspec `:(exclude)docs/phases/*/reports` excludes nothing; the trailing `/*` form (`:(exclude)docs/phases/*/reports/*`) works for both untracked and modified files. A wholly new untracked phase directory whose only contents are reports collapses to one `??` entry and is excluded as a unit; once the phase has any tracked file (the plan), non-report strays inside it are reported normally.
- 2026-09-02: a plain `go build` reports `debug.BuildInfo.Main.Version` as `(devel)`, a `go install ...@<sha>` reports the pseudo-version, and a `go install ...@vX.Y.Z` reports `vX.Y.Z`; the `version` fallback keys on the `0.0.0-dev` ldflags placeholder so goreleaser builds are never second-guessed. `mise exec -- go` outside the repo uses the machine's global Go pin, not this repo's (`compile: version mismatch`); QA used the pinned toolchain by absolute path under `~/.local/share/mise/installs/go/`.
- 2026-09-02: `freshbooks version` prints the bare version; `freshbooks-mcp version` prints `freshbooks-mcp <version>`. Deliberate and documented nowhere as a contract; the runbook below asserts each binary's real output.

## Phase-close backlog (convergence + live conformance)

Cross-phase items deferred by triage. Items folded by Phase 5 were removed (cobra/doc linkage, the dirty-tree guard, the dependency refresh, the changelog reflow, Q4/Q12/Q15/Q17/Q20/Q21/Q22).

1. **Business-family sort direction**: `Sort()` emits `field_desc`; docs + one Postman capture say `-field` for business endpoints. Documented as a caveat on the CLI `--sort` flag and the MCP tools; resolve in the live-conformance pass.
2. **CLI test hardening leftovers** (Phase 4 QA Q8, Q9): `-o json` output asserted for every command; permission tests use literal modes.
3. **Dual-stack loopback binding** (Phase 4 security A7): the listener is on `127.0.0.1` while the redirect says `localhost`; binding `::1` too is the real fix.
4. **`PageMeta` drops `meta.sort`** (captured on Projects list). Model or document.
5. **`StaffService.List` discards the sibling business fields**; live-check then decide.
6. **Full-fixture sweep**: one verbatim captured-response fixture per resource; most fixtures are still trimmed.
7. **Live-conformance pass** (attended, needs a sandbox): every `STATE AS OF` docs-only fact; checkout-link response shape, `EnablePaymentOptions` response, tokenization shapes, ledger taxonomy endpoints, webhook `callback_id` body, invoice delete verb, quoted-ID writes, `Expenses/Create Custom Expense Category`, the `DateTime` zoneless format's real producers; the MCP and the CLI end to end with a real token, including a real `auth login` through the self-signed loopback.
8. **govulncheck locally**: not installed as a tool; `scripts/check.sh` runs it via `go run ...@v1.7.0`, which is pinned and sufficient. Consider a mise pin only if the `go run` fetch becomes a CI cost.
9. **`usage` is the one mise tool with no checksum or signature** (Phase 5 security A3) and it interprets `scripts/changelog-section.sh` in the release path. Either pin it verifiably when aqua grows a checksum for it, or drop the shebang from that one script (it takes two positional args and needs no spec).
10. **Repository tag-protection ruleset** (Phase 5 security A1): no ruleset guards `refs/tags/{freshbooks,mcp,cli}/v*`, and `enforce_admins` is off on `main`, so the release guard's "on main" premise is weaker than it reads. Pre-flight step 0 of the tag runbook below.
11. **`docsgen_test.go` could table-drive its five `strings.Contains` checks** (Phase 5 simplify S4). Cosmetic; do it the next time that file is touched.

## v0.1.0 tag step (done 2026-09-02)

Ran as designed: lib changelog cut (`0b63858`) -> `freshbooks/v0.1.0` -> `go.mod` bump + mcp/cli changelog cuts + `--title` on `gh release create` (`90cc8cd`) -> `mcp/v0.1.0` -> `cli/v0.1.0`. Each Release run went guard -> ci -> release green. Still open from the pre-flight: the tag-protection ruleset (backlog item 10). The three "once the tags ship" doc caveats were flipped in the ship commit.

## Next action: the live-conformance pass (attended)

Run `/goal complete everything in @GOAL.md` with Wes available. Attended; needs Wes's FreshBooks credentials through `fnox exec -- <cmd>` (never exported, never written). Branch `phase-7/live`, opus implementer, fable reviewers. `FRESHBOOKS_LIVE=1 mise exec -- go test -tags live ./freshbooks/...` for the read-only smoke; then, for each backlog item 7 fact, one captured request/response with synthetic IDs into `freshbooks/testdata/seed/`, the spec `STATE AS OF` callout upgraded from docs-only to CONFIRMED (or corrected), and a fixture seeded from the capture. Finish with a real `freshbooks auth login` through the self-signed loopback and one MCP `tools/call` with the resulting token.

## How to resume in a fresh session

1. Read this file, then `GOAL.md`, then `CLAUDE.md`.
2. `git status --porcelain` must be empty and `git log --oneline -5` should match the ledger above. If not, reconcile before starting.
3. Read only the spec sections the current phase names.
4. Start the goal.
