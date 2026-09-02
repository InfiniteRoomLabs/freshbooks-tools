# Phase 5 implementer report (release hardening)

Branch `phase-5/release`, folds F1-F10, all ten checkpoint commits green. Full gate green on a clean tree at the end.

## Commits

```
a52c753 docs: shape the module changelogs for 0.1.0
6f17152 docs: bring the guides in line with the shipped lib, MCP, and CLI
db0febb fix(cli): fold the Phase 4 QA advisories
8997f1d chore(scripts): ignore phase reports in the dirty-tree guard
b43be80 feat(cli,mcp): fall back to the module version when no ldflags are set
bde175e refactor(cli): move cobra/doc generation behind a docsgen build tag
dfcc6bb fix(release): let goreleaser build and gh publish the prefixed tags
630637a chore(ci): pin actions by SHA and drop the artifact round trip
e1c4bf1 chore(deps): refresh module dependencies and toolchain pins
1f0abbd docs(phase-5): add the stage-1 plan and implementer work order
```

(This report is F10's commit, landing after `a52c753`.)

## Gate: final `mise run check` tail

```
coverage-gate: PASS
== vuln: cli ==
No vulnerabilities found.
== inventory-check: cli (skipped -- only freshbooks has an inventory) ==
== actionlint ==
== build ==
build: mcp ./cmd/freshbooks-mcp -> .../dist/freshbooks-mcp_linux_amd64
build: mcp ./cmd/freshbooks-mcp -> .../dist/freshbooks-mcp_linux_arm64
build: mcp ./cmd/freshbooks-mcp -> .../dist/freshbooks-mcp_darwin_amd64
build: mcp ./cmd/freshbooks-mcp -> .../dist/freshbooks-mcp_darwin_arm64
build: mcp ./cmd/freshbooks-mcp -> .../dist/freshbooks-mcp_windows_amd64.exe
build: mcp ./cmd/freshbooks-mcp -> .../dist/freshbooks-mcp_windows_arm64.exe
build: cli ./cmd/freshbooks -> .../dist/freshbooks_linux_amd64
build: cli ./cmd/freshbooks -> .../dist/freshbooks_linux_arm64
build: cli ./cmd/freshbooks -> .../dist/freshbooks_darwin_amd64
build: cli ./cmd/freshbooks -> .../dist/freshbooks_darwin_arm64
build: cli ./cmd/freshbooks -> .../dist/freshbooks_windows_amd64.exe
build: cli ./cmd/freshbooks -> .../dist/freshbooks_windows_arm64.exe
build: done, artifacts in .../dist
check.sh: all OK
```

Exit 0. `git status --porcelain` empty on the same run.

## Coverage per module

- `freshbooks`: 91.8% (floor 90%)
- `mcp`: 92.1% (floor 90%)
- `cli`: 91.5% (floor 90%)

`mise run inventory-check`: `implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0`. `mise run vuln`: clean in all three modules (3x "No vulnerabilities found." in the gate log).

## Dependency diff (F1)

```
git diff main -- '*/go.mod' go.work.sum | grep '^[-+]' | grep -v '^[-+][-+]'
```

```
-	github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks v0.0.0-20260902001017-f9a722046dd4
+	github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks v0.0.0-20260902041524-dbd898b28413
-	github.com/spf13/pflag v1.0.9
+	github.com/spf13/pflag v1.0.10
-	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
+	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
-	go.yaml.in/yaml/v3 v3.0.4 // indirect
+	go.yaml.in/yaml/v3 v3.0.5 // indirect
-	github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks v0.0.0-20260901220418-d795b3fedd2b
+	github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks v0.0.0-20260902041524-dbd898b28413
-	github.com/segmentio/asm v1.1.3 // indirect
+	github.com/segmentio/asm v1.2.1 // indirect
-	github.com/spf13/pflag v1.0.9 // indirect
+	github.com/spf13/pflag v1.0.10 // indirect
-	golang.org/x/oauth2 v0.35.0 // indirect
-	golang.org/x/sync v0.20.0 // indirect
-	golang.org/x/sys v0.41.0 // indirect
+	golang.org/x/oauth2 v0.36.0 // indirect
+	golang.org/x/sync v0.22.0 // indirect
+	golang.org/x/sys v0.47.0 // indirect
```

Matches D5 exactly. `go.work.sum` had no diff to commit (unlike the Phase 4 lesson's expectation, `go get -u`/`go mod tidy` this time round happened not to touch it -- verified no stray diff appeared in any later commit either). No new direct dependencies in any module; `freshbooks-mcp tools` manifest (`tools > /tmp/tools-before.json` before F1, diffed after) byte-identical.

## D6 proof (F4)

```
mise exec -- go -C cli build -o /tmp/fb ./cmd/freshbooks
mise exec -- go version -m /tmp/fb | grep -c 'md2man\|blackfriday'
0
```

`docs/cli.md` diff across F4 was empty (verified against a pre-F4 copy); `mise run docs` run twice back to back produced byte-identical output (idempotent).

## D7 test names (F5)

Both `cli/internal/cmd/version_test.go` and `mcp/cmd/freshbooks-mcp/version_test.go` (identical structure, one per module):

- `TestResolveVersion/[happy]_a_real_ldflags_version_is_returned_unchanged`
- `TestResolveVersion/[happy]_falls_back_to_the_module_pseudo-version_when_unbuilt`
- `TestResolveVersion/[sad]_ReadBuildInfo_returning_false_leaves_the_placeholder`
- `TestResolveVersion/[edge]_a_(devel)_Main.Version_leaves_the_placeholder`
- `TestResolveVersion/[edge]_an_empty_Main.Version_leaves_the_placeholder`

Verified locally (not a unit test, a manual sanity check) that a plain `go build` reports `Main.Version` `"(devel)"` (`go version -m` on the resulting binary), confirming the fallback correctly leaves `0.0.0-dev` alone on that path -- only `go install <module>@<tag-or-sha>` gives Go a real `Main.Version` to substitute. A real `go install .../cmd/freshbooks@<sha>` probe needs the branch pushed to the real remote (this branch is local-only); that is QA's mandatory probe 2 per `GOAL.md` stage 3, not something this implementer could exercise.

## F10 dry-run evidence

### Snapshot builds (from `mcp/` and `cli/`, this working tree)

```
GORELEASER_CURRENT_TAG=v0.1.0 mise exec -- goreleaser release --snapshot --skip=publish --clean
```

Both exit 0, ~18s each. Archive names all start `freshbooks-mcp_0.1.0-SNAPSHOT-a52c753_*` / `freshbooks_0.1.0-SNAPSHOT-a52c753_*`, six each (`{linux,darwin,windows} x {amd64,arm64}`, windows as `.zip`), plus one `.sbom.json` per archive and a shared `checksums.txt`.

- `sha256sum -c checksums.txt --ignore-missing` from each `dist/`: all 12 entries (6 archives + 6 sboms) `OK`, exit 0.
- Every `.sbom.json` parses as JSON (`python3 -c "import json; json.load(open(f))"`, one per file, all OK).
- `freshbooks-mcp version` (linux/amd64 binary, extracted from its archive): `freshbooks-mcp 0.1.0-SNAPSHOT-a52c753`.
- `freshbooks version` (linux/amd64 binary, extracted): `0.1.0-SNAPSHOT-a52c753` (no binary-name prefix -- the CLI's `newVersionCmd` prints the bare version string; this is existing, unchanged Phase 4 behavior, not a Phase 5 defect).

### Real prefixed-tag dry run (scratch clone)

```sh
git clone <repo root> /tmp/gr-clone
cd /tmp/gr-clone && git tag mcp/v0.1.0
mise trust
cd mcp && GORELEASER_CURRENT_TAG=v0.1.0 mise exec -- goreleaser release --skip=publish,validate --clean
```

Tail:

```
    - building                                       paths=cmd/freshbooks-mcp binaries=freshbooks-mcp target=linux_amd64_v1
    - building                                       paths=cmd/freshbooks-mcp binaries=freshbooks-mcp target=linux_arm64_v8.0
    - building                                       paths=cmd/freshbooks-mcp binaries=freshbooks-mcp target=windows_arm64_v8.0
    - building                                       paths=cmd/freshbooks-mcp binaries=freshbooks-mcp target=darwin_amd64_v1
    - building                                       paths=cmd/freshbooks-mcp binaries=freshbooks-mcp target=darwin_arm64_v8.0
      - took: 14s
  - archives
    - archiving                                      name=dist/freshbooks-mcp_0.1.0_darwin_amd64.tar.gz
    - archiving                                      name=dist/freshbooks-mcp_0.1.0_windows_amd64.zip
    - archiving                                      name=dist/freshbooks-mcp_0.1.0_linux_amd64.tar.gz
    - archiving                                      name=dist/freshbooks-mcp_0.1.0_darwin_arm64.tar.gz
    - archiving                                      name=dist/freshbooks-mcp_0.1.0_windows_arm64.zip
    - archiving                                      name=dist/freshbooks-mcp_0.1.0_linux_arm64.tar.gz
  - software bill of materials
    (... 6x cataloging ...)
  - calculating checksums
  - writing artifacts metadata
  - release succeeded after 31s
  - thanks for using GoReleaser!
```

Exit 0. `sha256sum -c checksums.txt --ignore-missing`: all OK. `freshbooks-mcp version` on the resulting linux/amd64 binary: `freshbooks-mcp 0.1.0` (no `-SNAPSHOT` suffix, no sha -- exactly the release shape). This is the strongest evidence for D1: the exact command line the release workflow now runs works end to end against a real `mcp/vX.Y.Z`-shaped tag, not just a snapshot build.

`/tmp/gr-clone` removed after the probe; the working tree's own `dist/` (gitignored) still holds the two snapshot runs' output at report time.

## Dead doc references fixed

None found. Every backticked CLI command path in the docs pass (F8) ran `--help` against the built binary; every `FRESHBOOKS_*`/`FRESHBOOKS_MCP_*` env var was grepped live in non-test Go source across `cli/`, `mcp/`, `freshbooks/`; the lib snippet in `docs/getting-started.md` was verified to compile and run (in a scratch module with a `replace` directive, reaching the real FreshBooks API and getting the expected 401 for a placeholder token) rather than just read plausible. Three stale forward-references to already-shipped phases were corrected in prose (`docs/authentication.md` x2, `docs/library.md` x2) but none of those were dead *identifiers* -- just stale "lands in Phase N" wording.

## `git status --porcelain`

Empty (verified immediately before writing this report, and again after `mise run check`).

## Where reality disagreed with D1-D10

- **D3's fold placement.** D3 (syft) was assigned to F3 in the fold list, but I added `"aqua:anchore/syft" = "1.51.1"` to `mise.toml` in F1 alongside D5's toolchain pins, since it was the same file and the same kind of edit. Functionally identical outcome (syft is pinned and working by F3, when the `.goreleaser.yaml` `sboms:` stanza actually needs it); noting the process deviation since the plan called it out as a distinct F3 item.
- **D8's exact pathspec doesn't work on this git (2.43.0).** `git status --porcelain -- . ':(exclude)docs/phases/*/reports'` excludes nothing -- verified both for a brand-new untracked `reports/` directory (git collapses it to one `??` entry that the pathspec doesn't reach) and for a modification to an already-tracked report file. `':(exclude)docs/phases/*/reports/*'` (matching files under the directory, not the directory pathspec itself) does exclude both cases, and still flags genuinely dirty files elsewhere -- verified both directions before committing. Used that form instead; documented in the F6 commit message and `docs/building.md`.
- **Q13, Q16, and Q18 (inside D9) were already resolved on this branch before Phase 5 started**, contradicting D9's listing of them as Phase 5 backlog. The Phase 4 QA round-2 report's own summary line says exactly this ("Q1-Q3, Q5, Q6, Q7, Q10, Q11, **Q13**, Q14, **Q16** and **Q18** all resolved"), and I verified it live: the docs header already carried the `FRESHBOOKS_OUTPUT`/Binary-command caveat (Q13) and the `--base-url`-hidden note (Q18), and `errors.go`'s `runtimeError` doc comment already matched what it wraps (Q16), with no reference anywhere to the old "a filesystem failure reading `--file`" text Q16 flagged. Left the substance as-is in the F7 commit; only the internal "G6/QA Qn:" review-trail prefixes on those two table rows were dropped as part of the Q21 reflow (cosmetic, not a content change).
- Nothing else disagreed. D1's goreleaser command line, D2's `project_name`/`GOWORK=off` fix, D4's SHA pins, D6's build-tag design, D7's fallback seam, and D10's changelog shape all worked exactly as specified on the first attempt, including the two live goreleaser dry runs (snapshot and real-tag) both matching D1's predicted behavior precisely.

## Not done here (lead-owned, per the work order)

Gate dispatch (code-review/simplify/security in parallel, then QA), the five stale dependabot PRs, the root `CHANGELOG.md` phase-ship line, `docs/progress.md` ledger + the attended tag runbook, and `GOAL.md` retarget.
