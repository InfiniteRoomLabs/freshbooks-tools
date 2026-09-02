# Phase 5 QA / reality-check lane report

**Subject:** branch `phase-5/release` in `<repo root>`, HEAD `6e41003` (13 commits ahead of `main`, `1f0abbd..6e41003`).
**Date:** 2026-09-02.
**Verdict: PASS** -- with three ADVISORY findings (Q1-Q3). No blocking findings.

All four mandatory probes passed on evidence. Every "Applied" row in `docs/phases/5/triage.md` was independently confirmed present in the tree. The gate is green on a clean tree, the release plumbing produces exactly the artifacts the workflow's globs expect, `go install` outside the workspace yields a real pseudo-version, and every doc example command runs.

---

## Findings

### Q1 -- ADVISORY -- `docs/library.md` says `DateTime` accepts three wire formats; the shipped lib accepts four

`docs/library.md:47`

> `DateTime` accepts all three formats FreshBooks uses -- RFC 3339, `YYYY-MM-DD HH:MM:SS`, and a bare date

**Expected** (computed from the shipped code): four. `freshbooks/types.go:166` declares

```go
var dateTimeLayouts = []string{RFC3339Layout, noZoneLayout, DateTimeLayout, DateLayout}
```

and `noZoneLayout` (`freshbooks/types.go:163`) is `"2006-01-02T15:04:05"`, the zoneless variant Phase 2 batch c added for the Projects/Time Tracking family. `UnmarshalJSON`'s own doc comment (`freshbooks/types.go:168`) lists all four, and `freshbooks/CHANGELOG.md:14` -- the text that will ship as the 0.1.0 release notes -- also says four, naming the zoneless format explicitly.

**Observed:** the guide says three and omits the zoneless format entirely.

So the module's release notes and its user guide contradict each other on a decoding contract, in the same release. This is the exact class of defect F8 (the docs pass) existed to remove; it understates capability rather than misdirecting a user, hence advisory rather than blocking. A user whose Projects payloads carry `2019-04-19T18:25:00` would read `docs/library.md` and conclude the lib cannot decode them.

**Suggested fix:** one sentence in `docs/library.md`, matching the changelog's wording -- "all four formats FreshBooks uses -- RFC 3339, `YYYY-MM-DD HH:MM:SS`, a bare date, and a zoneless `YYYY-MM-DDTHH:MM:SS` seen in the Projects/Time Tracking family". Note `InLayout` correctly documents only the three *exported* layout constants, since `noZoneLayout` is unexported and cannot be selected for marshaling; that part needs no change.

Commands run:
```sh
sed -n '155,200p' freshbooks/types.go
grep -n 'DateTime' docs/library.md freshbooks/CHANGELOG.md
```

### Q2 -- ADVISORY -- triage row A4 was applied to `docs/getting-started.md` only; `docs/mcp.md` still publishes `--addr 0.0.0.0:8080`

`docs/mcp.md:63`

> Run `freshbooks-mcp serve --transport http --addr 0.0.0.0:8080` behind a TLS-terminating reverse proxy

**Expected:** loopback, consistent with the fix applied one file over and with the binary's own default. `docs/getting-started.md:101` now reads `--addr 127.0.0.1:8080` (A4, applied in `6e41003`), and `serve --help` reports `--addr string  address to listen on in http mode (default "127.0.0.1:8080")`. `docs/mcp.md`'s own configuration table (line ~108) also documents the default as `127.0.0.1:8080`.

**Observed:** `docs/mcp.md`'s prose example is an explicit override that binds every interface, contradicting both the table three sections below it and the sibling guide. The surrounding paragraph does carry the plain-HTTP/TLS warning, which is why this is advisory rather than blocking -- but A4's rationale ("do not print a bind-all address next to a plain-HTTP bearer-token server") applies identically here, and the security lane simply did not look at this file's example.

**Suggested fix:** `--addr 127.0.0.1:8080` in `docs/mcp.md` too, or keep `0.0.0.0` with an explicit one-clause justification (a container needs it) so the inconsistency reads as deliberate.

Commands run:
```sh
grep -n 'addr' docs/mcp.md docs/getting-started.md
/tmp/qa-bins/freshbooks-mcp serve --help | grep -E '^\s+--(transport|addr|path|log-level|log-format)'
```

### Q3 -- ADVISORY -- the draft tag runbook's step 4 inherits an assertion that is false for the CLI

`docs/phases/5/plan.md:91-92`

> 3. **MCP.** ... `freshbooks-mcp version` prints `freshbooks-mcp v0.1.0`.
> 4. **CLI.** Same as 3 with `cli/v0.1.0`, `freshbooks version`.

**Expected** by "same as 3": `freshbooks version` prints `freshbooks v0.1.0`.

**Observed:** the CLI prints the bare version with no binary-name prefix. The two binaries deliberately differ:

- `mcp/cmd/freshbooks-mcp/root.go:44` -- `fmt.Fprintf(stdout, "freshbooks-mcp %s\n", version)`
- `cli/internal/cmd/root.go:108` -- `fmt.Fprintln(cmd.OutOrStdout(), version)`

Measured on the real artifacts:

```
$ ./freshbooks-mcp version   ->  freshbooks-mcp 0.1.0-SNAPSHOT-6e41003
$ ./freshbooks version       ->  0.1.0-SNAPSHOT-6e41003
$ /tmp/qa-bin/freshbooks version      -> v0.0.0-20260902183025-6e410036599a
$ /tmp/qa-bin/freshbooks-mcp version  -> freshbooks-mcp v0.0.0-20260902183025-6e410036599a
```

No shipped doc claims otherwise, so this is not a docs defect and the asymmetry itself is fine. The risk is purely that the lead is about to transcribe this draft into `docs/progress.md` as the attended runbook: step 4 as written would have the operator assert an output the CLI never produces, and the tag step would appear to fail. **This is lead-owned, not implementer-owned** -- flagging it before the runbook is written, not as a change to the branch.

**Suggested fix:** step 4 spells out its own assertion -- "`freshbooks version` prints `v0.1.0` (bare, no binary-name prefix -- unlike `freshbooks-mcp`)".

---

## Probe 1 -- goreleaser, clean checkout, both modules

Setup: `git clone <repo root> /tmp/qa-clone && git -C /tmp/qa-clone checkout phase-5/release` (HEAD `6e41003`), `mise trust /tmp/qa-clone/mise.toml`.

**Snapshot form**, from `/tmp/qa-clone/mcp` and `/tmp/qa-clone/cli`:

```sh
GORELEASER_CURRENT_TAG=v0.1.0 mise exec -- goreleaser release --snapshot --skip=publish --clean
```

Both exit **0** (mcp 25s, cli 16s). Assertions, all met:

| Assertion | mcp | cli |
|---|---|---|
| 6 archives, correct prefix | `freshbooks-mcp_0.1.0-SNAPSHOT-6e41003_*` -- 4 `.tar.gz` + 2 `.zip` | `freshbooks_0.1.0-SNAPSHOT-6e41003_*` -- 4 `.tar.gz` + 2 `.zip` |
| windows is `.zip` | yes (`windows_amd64.zip`, `windows_arm64.zip`) | yes |
| linux/amd64 binary runs, `version` contains `0.1.0-SNAPSHOT` | `freshbooks-mcp 0.1.0-SNAPSHOT-6e41003` | `0.1.0-SNAPSHOT-6e41003` |
| `sha256sum -c checksums.txt --ignore-missing` | exit 0, all `OK` (12 entries: 6 archives + 6 SBOMs) | exit 0, all `OK` |
| one `.sbom.json` per archive | 6 | 6 |
| **R1:** every SBOM is SPDX | `jq -r .spdxVersion` = `SPDX-2.3` on all 6; `.bomFormat` absent | same on all 6 |

**R1 docs cross-check.** `grep -rn -i cyclonedx` over `README.md docs/ CHANGELOG.md */CHANGELOG.md */.goreleaser.yaml .github/` returns hits only inside `docs/phases/5/{reports/code-review.md,plan.md,triage.md}` -- i.e. the review artifacts that discuss the finding, never a user-facing claim. All five files the triage named now say "SPDX 2.3 JSON SBOM": `README.md:21`, `docs/building.md:50`, `CHANGELOG.md:25`, `mcp/CHANGELOG.md:17`, `cli/CHANGELOG.md:21`. R1 confirmed applied and confirmed true against the generated documents. (Note: `docs/phases/5/plan.md:55` still carries the pre-triage phrase "CycloneDX/SPDX SBOMs". That is the historical work order, deliberately outside R1's five-file scope; no action recommended, recorded only so nobody re-finds it later.)

**Real-tag form** (D1's exact CI command line):

```sh
git -C /tmp/qa-clone tag mcp/v0.1.0
cd /tmp/qa-clone/mcp && GORELEASER_CURRENT_TAG=v0.1.0 mise exec -- goreleaser release --skip=publish,validate --clean
```

Exit **0**, 16s. Archives came out as plain semver -- `freshbooks-mcp_0.1.0_{linux,darwin}_{amd64,arm64}.tar.gz` and `freshbooks-mcp_0.1.0_windows_{amd64,arm64}.zip` -- and the extracted linux/amd64 binary prints exactly `freshbooks-mcp 0.1.0`. D1 holds as written.

**Publish-glob rehearsal.** Reproducing the workflow's publish step verbatim (its `working-directory` is the module, its globs are relative):

```sh
cd /tmp/qa-clone/mcp && bash -c 'shopt -s failglob; ls -1 dist/*.tar.gz dist/*.zip dist/checksums.txt dist/*.sbom.json | wc -l'
```

**13** files, exit 0 -- 6 archives + `checksums.txt` + 6 SBOMs. Every glob in `gh release create` matches; `failglob` (A5) does not trip on a correct run, and would abort before the release exists on an incorrect one.

**Lib module.** Confirmed `freshbooks/` has no `.goreleaser.yaml` (`find . -name '.goreleaser*'` returns only `mcp/` and `cli/`), and both goreleaser steps in `release.yml` are gated `if: needs.guard.outputs.module == 'mcp' || ... == 'cli'`, so a `freshbooks/vX.Y.Z` tag falls straight through to `gh release create --verify-tag`. Correct and consistent with `docs/building.md`.

## Probe 2 -- `go install` outside the workspace (D7, D6)

Run from `/tmp` (no `go.work` in any parent -- confirmed). First attempt with `mise exec -- go` failed with `compile: version "go1.26.6" does not match go tool version "go1.26.7"`: outside the repo, `mise exec` does not pick up this repo's `mise.toml` pin. **Recorded as a method note, not a finding** -- it is the known toolchain-skew gotcha in `CLAUDE.md`, and is an artifact of this machine, not of the branch. Re-run against the pinned toolchain by absolute path:

```sh
GO=~/.local/share/mise/installs/go/1.26.6/bin/go
env -u GOWORK -u GOROOT GOTOOLCHAIN=local GOFLAGS=-mod=mod GOBIN=/tmp/qa-bin \
  $GO install github.com/InfiniteRoomLabs/freshbooks-tools/cli/cmd/freshbooks@6e410036599aa746fc2560a7a759a938ca9513cc
# and .../mcp/cmd/freshbooks-mcp@6e410036599aa746fc2560a7a759a938ca9513cc
```

Both exit **0**. The public proxy had already indexed the sha (`go: downloading .../mcp v0.0.0-20260902183025-6e410036599a`), so no `GOPROXY=direct` fallback was needed.

**D7 acceptance -- met:**

```
/tmp/qa-bin/freshbooks version      -> v0.0.0-20260902183025-6e410036599a
/tmp/qa-bin/freshbooks-mcp version  -> freshbooks-mcp v0.0.0-20260902183025-6e410036599a
```

A real pseudo-version, **not** `0.0.0-dev`. The `debug.ReadBuildInfo()` fallback works end to end through a genuine proxy install, which is what the unit tests could not prove.

**D6 acceptance -- met:** `go version -m /tmp/qa-bin/freshbooks | grep -c 'md2man\|blackfriday'` = **0**. The recorded dependency list is exactly cobra, pflag, x/sys, x/term, yaml.v3, and the lib -- `cobra/doc` and its `go-md2man`/`blackfriday` dependents are in `cli/go.mod` (tests and the tagged command need them) but are not linked into the untagged binary. Same check on the goreleaser snapshot binary: also **0**.

## Probe 3 -- every doc example command runs

Scope: `README.md`, `docs/{getting-started,building,library,authentication,mcp}.md`, and the first 150 lines of `docs/cli.md`.

**CLI commands, `--help` against the snapshot binary** -- all exit 0: `auth login`, `auth token`, `identity me`, `config set-context`, `config view`, `config contexts`, `config use-context`, `api`, `version`.

**MCP commands** -- all exit 0: `serve --help`, `tools --help`, `version --help`.

**Tool manifest.** `freshbooks-mcp tools` exits 0 and emits **168** tools, matching `docs/mcp.md`'s opening claim. Every tool name the docs cite exists exactly once: `identity_whoami`, `invoices_list`, `reports_download_invoice_details_csv`, the four tokenization tools, the three `identity_*application*` tools, `identity_delete_business`, and the four non-paginated list tools. The doc's specific claim that exactly four `*_list` tools lack `page`/`per_page` is exact -- a manifest-wide query returns precisely `ledger_accounts_list retainers_list service_rates_list staff_list`.

**Live HTTP transport** (`freshbooks-mcp serve --transport http --addr 127.0.0.1:18080`, fake bearer, running the two `curl` bodies from `docs/mcp.md` and `docs/getting-started.md` verbatim):

| Documented claim | Observed |
|---|---|
| `GET /healthz` unauthenticated, always 200 | `status=200` |
| missing `Authorization` -> 401 + `WWW-Authenticate: Bearer` | `status=401`, `Www-Authenticate: Bearer realm="freshbooks-mcp"` |
| `initialize` answers | `status=200`, `serverInfo.name=freshbooks-mcp` |
| `tools/call identity_whoami` reaches upstream | `status=200`, `isError:true`, body `{"status":401,"message":"unauthenticated: ...","family":"auth"}` -- the documented error shape, carrying the real upstream 401 |
| `Stateless: true` -- no `Mcp-Session-Id` ever returned | 0 occurrences in response headers |
| http mode refuses a default scope | `FRESHBOOKS_ACCOUNT_ID=... serve --transport http` exits with the documented refusal message |
| `--transport`/`--addr`/`--path`/`--log-level`/`--log-format` defaults | `stdio` / `127.0.0.1:8080` / `/mcp` / `info` / `text` -- all match the doc's table |

**Lib snippet compiles.** The single ```go``` block in `docs/getting-started.md` extracted verbatim into `/tmp/qa-libsnip/main.go` with a scratch `go.mod` and `replace ... => /tmp/qa-clone/freshbooks`: `go mod tidy && go build ./...` exit **0**.

**Mechanical identifier audit.** Every `FRESHBOOKS_*` env name appearing in any of the seven docs (21 distinct names) resolves to at least one non-test Go occurrence -- zero dead references. Every backticked Go identifier in `docs/library.md` resolves in `freshbooks/` (`NewClient`, all seven `With*` options, `Do`, `NoRetry`, `RetryPolicy`, `MaxDelay`, `Include`/`Search`/`Sort`/`SortDesc`/`PageNumber`/`PerPage`, `Page[T]`, `All`, the five `Err*` sentinels, the six `Error` fields, `RetryAfter`, `Money.Rat`, `AccountID`/`BusinessID`/`BusinessUUID`, `VisState`, `NewDateTime`, `InLayout`, `Identity.Me`/`Whoami`, `Example`/`ExampleAll`). Every symbol in `docs/authentication.md` resolves in `freshbooks/auth/` (`MetadataEndpoints`, `DocumentedEndpoints`, `Config.AuthCodeURL`/`Exchange`/`Refresh`/`Revoke`, `TokenStore`, `NewFileStore`, `FileStore`, `DefaultTokenPath`, `NewMemoryStore`, `StaticTokenSource`, `DefaultExpirySkew`). Numeric claims verified against source: `DefaultExpirySkew = 60 * time.Second`; the PKCE verifier is 32 `crypto/rand` bytes, `base64.RawURLEncoding`, 43 chars, exactly as documented; `DefaultTokenPath()` resolves `$XDG_CONFIG_HOME/freshbooks/token.json`; the CLI's credentials path is `$XDG_CONFIG_HOME/freshbooks/credentials/<context>.json`, matching `docs/getting-started.md`. `docs/cli.md`'s "First login" section, referenced from `docs/authentication.md`, exists at line 9. All seven relative `*.md` links across README and docs resolve to real files. The only identifier-level defect found in the whole sweep is Q1.

**`mise run docs` idempotent.** Run twice in the clean clone: both exit 0, `git status --porcelain` byte-identical between runs and **empty** both times -- `docs/cli.md` as committed is exactly what the generator produces, and a second run changes nothing.

**ASCII.** `grep -nP '[^\x00-\x7F]' README.md docs/*.md */CHANGELOG.md CHANGELOG.md` prints **nothing**.

**No stale phase references.** `grep -n 'Phase [0-9]' README.md docs/{getting-started,building,library,authentication,mcp}.md` returns **nothing** -- R2 confirmed applied and complete across all six user-facing guides, not just the one file the finding named.

**Hygiene.** `scripts/redaction-check.sh` exits 0 (`redaction-check: clean`).

## Probe 4 -- `mise run check` on the current tree

```sh
mise run check > /tmp/qa-gate.log 2>&1   # exit 0
```

Log tail:

```
build: cli ./cmd/freshbooks -> <repo root>/dist/freshbooks_windows_arm64.exe
build: done, artifacts in <repo root>/dist
check.sh: all OK
GATE_EXIT=0
```

Every `coverage-gate:` line, all above the 90% floor:

```
coverage-gate: <repo root>/freshbooks/coverage.out total = 91.8% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo root>/mcp/coverage.out total = 92.1% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo root>/cli/coverage.out total = 91.5% (floor 90%)
coverage-gate: PASS
```

All 21 per-module steps ran (`fmt-check`, `vet`, `lint`, `test`, `cover`, `vuln`, `inventory-check` x 3 modules) plus `actionlint` and `build`.

- **`git status --porcelain`** shows exactly one line, `?? docs/phases/5/reports/qa.md` -- my own report and nothing else.
- **D8 exclusion verified both ways.** With the report present the gate printed **no** `DIRTY TREE` banner (`grep -c 'DIRTY TREE' /tmp/qa-gate.log` = 0). Then `touch cli/QA_STRAY.txt` and the gate's own pathspec, `git status --porcelain -- . ':(exclude)docs/phases/*/reports/*'`, **did** report `?? cli/QA_STRAY.txt`. The guard still catches a real dirty tree; it only ignores the reports directory. Stray file deleted.
- **`mise run actionlint`** standalone: exit 0, no output.
- **`mise run inventory-check`** standalone: `implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0`.
- **`mise run vuln`**: `No vulnerabilities found.` for all three modules.
- **`mise run check -- <module>`** (the single-module form README and `docs/building.md` both document) works: `mise run fmt-check -- cli` exits 0.
- **Green rule:** zero real `t.Parallel()` calls anywhere (the four grep hits in `cli/internal/cmd` are the Q22 comments *mentioning* `t.Parallel()`, not calls -- so the Q22 safety claim is true). Zero committed `-run` filters. One `t.Skip`, at `freshbooks/live_test.go:26`, gated behind both `-tags live` and `FRESHBOOKS_LIVE=1` -- the documented live-suite opt-out, not a hidden skip.

## Probe 5 -- GOAL.md stage 2 deliverables

| Deliverable | Met | Evidence |
|---|---|---|
| Docs pass accurate against the shipped lib/MCP/CLI | **met**, one defect | Full mechanical sweep above; only Q1 (advisory) found. ASCII clean, unwrapped, `--`/`->` only, no dead command/env/identifier references. |
| goreleaser dry runs per module, artifacts inspected | **met** | Probe 1: both snapshot runs + the real-tag run, all exit 0; names, `version` output, checksums, SBOM format all asserted. |
| CI hygiene, SHA pins verified | **met** | Both pins resolve exactly and are `commit` (lightweight) refs: `actions/checkout` v7.0.1 = `3d3c42e5aac5ba805825da76410c181273ba90b1`, `jdx/mise-action` v4.3.0 = `c2a87611a18de5b3828c5652fe268e992400cb5c`, via `gh api repos/<r>/git/ref/tags/<t>`. Both are also the current `releases/latest`. Only these two third-party actions remain across both workflows -- `upload-artifact`, `download-artifact`, and `goreleaser-action` are gone (D4). `mise.toml` pins are all current latest: goreleaser 2.18.0, golangci-lint 2.13.2, syft 1.51.1. `gh auth status` carries `repo` + `workflow`. |
| Dependency refresh, nothing stale | **met** | `go list -m -u all` per module reports only three upgradable entries -- `cloud.google.com/go/compute/metadata`, `golang.org/x/tools`, `gopkg.in/check.v1` -- and `go list -deps ./...` shows **0** of them in any module's build list. They are module-graph-only entries inherited from `x/oauth2`/testify go.mod files, never compiled or linked. All D5 bumps landed: pflag 1.0.10, x/sys 0.47.0 (closes GO-2026-5024), x/oauth2 0.36.0, x/sync 0.22.0, go-md2man 2.0.7, go.yaml.in/yaml/v3 3.0.5, segmentio/asm 1.2.1, lib pseudo-version `v0.0.0-20260902041524-dbd898b28413`. Dependency sets are exactly the ones CLAUDE.md allows; no new direct dependency. `go.work.sum` restored after each `go list` -- tree clean. |
| Backlog folds D6-D9 | **met** | D6 verified by `go version -m` = 0 on both a goreleaser binary and a `go install` binary. D7 verified by real proxy install. D8 verified both ways in probe 4. D9: Q4 -- `paths_test.go` pins the literal `44` plus `user:clients:read`/`user:invoices:write`, `TestDefaultScopes` passes all three subtests; Q12 -- `auth_cmd.go:153` registers `--login-timeout` and no local `--timeout`, documented at `docs/cli.md:61`; Q13 -- `docs/cli.md:32`; Q15 -- `docs/cli.md:49` is an open list ("a usage error, for example: ..."); Q18 -- `docs/cli.md:33` marks `--base-url` hidden; Q22 -- comments at `state.go:406` (covers both `stdinIsTerminal` and `stdoutIsTerminal`), `state.go:309` (`testTransport`), `auth_cmd.go:79` (`testAuthEndpoints`), and the claim is factually true (zero `t.Parallel()`). |
| `## [Unreleased]` shaped for 0.1.0, `changelog-section.sh` non-empty for all three | **met** | `scripts/changelog-section.sh <m> Unreleased` exits 0 for all three: freshbooks 6874 bytes, mcp 3786, cli 3949. `grep -c '^## \['` = **1** in each. All unwrapped (max line lengths 905 / 937 / 583 -- one paragraph per line, D10 / backlog item 12 satisfied). Root `CHANGELOG.md` likewise 1 heading, unwrapped. |
| `docs/progress.md` | **n/a this gate** | Still the pre-Phase-5 state, as expected -- the lead writes the ledger and the attended tag runbook after this verdict. Not failed on. The **draft** runbook in `docs/phases/5/plan.md:87-93` was checked against the workflow as it now is: preconditions, the guard's semver + on-`main` checks, the changelog rename ordering, the lib-first / bump / mcp / cli tag order, the six-archive + `checksums.txt` + six-SBOM expectation, and the `sha256sum -c` and `go install` verifications are all correct and match what I measured. The one defect is Q3 (step 4's inherited assertion). Two lead decisions already carried into it -- A1's tag-creation ruleset and S7's "once v0.1.0 tags ship" grep -- are recorded in `docs/phases/5/triage.md` and should survive into `docs/progress.md`. |

## Probe 6 -- release workflow, by reading

**`mcp` / `cli` tag path** (`mcp/v0.1.0`):

1. `guard` -- checkout `@3d3c42e5` with `fetch-depth: 0`, `permissions: contents: read`. Parses `module="${TAG_NAME%%/*}"` -> `mcp`, `version="${TAG_NAME#*/v}"` -> `0.1.0`. Rehearsed all three real tag shapes; all parse correctly and pass the semver regex. Rejects a non-semver version and an unknown module. `git fetch origin main` + `git merge-base --is-ancestor "$GITHUB_SHA" origin/main` -- a tag not on `main` cannot publish. Then mise install, then `scripts/changelog-section.sh mcp 0.1.0` as the early fail. **Verified this fails today** (exit 1) because the sections are still `## [Unreleased]` -- exactly the fail-fast `docs/building.md` step 1 describes, and it will pass once the attended rename lands.
2. `ci` -- `uses: ./.github/workflows/ci.yml` (`workflow_call`), `permissions: contents: read`. `ci.yml` also declares `permissions: contents: read` at workflow level. Three jobs, `lib` then `mcp`/`cli`, each `mise run check -- <module>`.
3. `release` -- `permissions: contents: write`, checkout with **`persist-credentials: false`** (A2, confirmed at `release.yml:76`). Re-extracts release notes to `/tmp/release-notes.md` (absolute, so `working-directory` on the later steps cannot break it). Then `working-directory: mcp` (S3, `release.yml:95`) + `GORELEASER_CURRENT_TAG=v0.1.0` + `mise exec -- goreleaser release --skip=publish,validate --clean`. Then `working-directory: mcp` again (`release.yml:103`), `shopt -s failglob` (A5, `release.yml:110`), `gh release create "$TAG_NAME" --verify-tag --notes-file /tmp/release-notes.md dist/*.tar.gz dist/*.zip dist/checksums.txt dist/*.sbom.json`.

**The globs match what probe 1 produced** -- rehearsed from the real `dist/`, 13 files, exit 0. `persist-credentials: false` costs nothing here: `gh` authenticates from `GITHUB_TOKEN` in the step env, not from the checkout's git credential helper, and goreleaser needs only local history (`fetch-depth: 0` supplies it, and `mod_timestamp: {{ .CommitTimestamp }}` reads it). syft reaches goreleaser via mise: `mise-action` with `install: true` installs everything in `[tools]` including `aqua:anchore/syft`, and `mise exec --` puts it on the child's PATH -- proven locally, since the same invocation generated all 12 SBOMs.

**`freshbooks` tag path** (`freshbooks/v0.1.0`): identical `guard` + `ci`. In `release`, both goreleaser and publish steps are skipped by their `if:`, the changelog extraction still runs (unconditional), and the `Create the lib release` step (`if: module == 'freshbooks'`) runs `gh release create "$TAG_NAME" --verify-tag --notes-file /tmp/release-notes.md` with no assets. Correct -- the lib has no binary and no `.goreleaser.yaml`, and the module proxy picks the tag up on its own.

**Nothing found that would fail on the first real tag push**, beyond the two things already understood and scheduled:

- The changelog sections must be renamed from `## [Unreleased]` to `## [0.1.0] - <date>` **before** the tag is pushed, or `guard` fails at the changelog step. This is runbook step 1/2 and is by design; I verified the failure is real and early (it happens in `guard`, before `ci` burns any time).
- A1 (no tag-creation ruleset on the repo; `enforce_admins: false`) remains a repository-settings gap the triage deferred to the runbook's pre-flight. Unchanged by this branch; noted so it is not lost.

`actionlint` is green over both workflows.

---

## Summary

| Probe | Result |
|---|---|
| 1 -- goreleaser, clean checkout, snapshot + real tag | **PASS** |
| 2 -- `go install` outside the workspace (D7, D6) | **PASS** |
| 3 -- every doc example command runs | **PASS** (Q1, Q2 advisory) |
| 4 -- `mise run check` green, clean tree, D8 exclusion | **PASS** |
| 5 -- GOAL.md stage 2 deliverables | **PASS** (Q3 advisory, lead-owned) |
| 6 -- release workflow by reading | **PASS** |

**Verdict: PASS.** Nothing on this branch blocks the merge. Q1 and Q2 are two-line doc edits that would be cheap to fold into a follow-up commit or to carry as backlog; Q3 is a correction the lead should make while writing the runbook into `docs/progress.md`, not a change to the branch.

Working tree at the end of this run: `git status --porcelain` shows only `?? docs/phases/5/reports/qa.md`. All `/tmp` scratch (`qa-clone`, `qa-bin`, `qa-bins`, `qa-libsnip`, `qa-tagbin`, logs) removed; no scratch files left in the tree.
