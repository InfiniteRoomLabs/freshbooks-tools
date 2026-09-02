# Phase 5 code-review lane report

Branch `phase-5/release`, `git diff main...phase-5/release`, 11 commits `1f0abbd..8b13939`. Read-only pass: no files modified outside this report, no gate/test/build runs (QA owns those). Evidence below comes from reading the diff and the code, plus `git`, `grep`, and reads of the implementer's own gitignored `mcp/dist/` snapshot output.

## Verdict

**REQUEST CHANGES** -- one blocking finding (R1), cheap to fix, and it lands in text that ships as the published GitHub release notes. Everything else is advisory.

The substance of the phase is sound. D1-D10 hold up on inspection: the goreleaser/`gh` split, the SHA pins and artifact-round-trip removal, `project_name` + `GOWORK=off`, the `docsgen` build-tag refactor, the `debug.ReadBuildInfo` version fallback, the `--login-timeout` rename, the dirty-tree pathspec, and the changelog reshaping all do what the plan says. The three deviations the implementer reported (D3 folded into F1, the `:(exclude)docs/phases/*/reports/*` pathspec form, Q13/Q16/Q18 already resolved) each check out against the code.

---

## R1 -- BLOCKING -- the SBOMs are SPDX 2.3, not CycloneDX; five committed files claim CycloneDX

**Where:** `README.md:21`, `docs/building.md:48`, `CHANGELOG.md:25`, `mcp/CHANGELOG.md:17`, `cli/CHANGELOG.md:21`.

**Why it matters.** Neither `.goreleaser.yaml` sets `sboms[].args`, so goreleaser applies its default, which is `spdx-json`, not `cyclonedx-json`. The effective config goreleaser itself wrote during the F10 dry run says so:

```
# mcp/dist/config.yaml
sboms:
  - id: default
    cmd: syft
    args:
      - $artifact
      - --output
      - spdx-json=$document
```

and the generated document confirms it:

```
# mcp/dist/freshbooks-mcp_0.1.0-SNAPSHOT-a52c753_darwin_amd64.tar.gz.sbom.json
{"spdxVersion":"SPDX-2.3","dataLicense":"CC0-1.0","SPDXID":"SPDXRef-DOCUMENT",...
```

The F10 acceptance criterion in the plan was `jq .bomFormat` (a CycloneDX-only field); the implementer substituted a generic `json.load` parse check, so the format mismatch was never caught. This is not cosmetic: `mcp/CHANGELOG.md` and `cli/CHANGELOG.md` are the exact text `scripts/changelog-section.sh` extracts into `gh release create --notes-file`, so tagging `mcp/v0.1.0` today would publish a release whose own notes misidentify the SBOM format sitting next to them. A consumer wiring the artifact into a CycloneDX-only ingest path would fail on a claim we made.

**Fix (pick one, either is a small commit):**
- **Preferred:** correct the wording in all five places -- "a CycloneDX SBOM" -> "an SPDX 2.3 JSON SBOM", and `docs/building.md`'s "SBOMs (CycloneDX via syft...)" -> "SBOMs (SPDX 2.3 JSON via syft...)". No config change, no re-verification of the dry run needed.
- **Alternative:** if CycloneDX is actually wanted, add to both `mcp/.goreleaser.yaml` and `cli/.goreleaser.yaml`:
  ```yaml
  sboms:
    - artifacts: archive
      args: ["$artifact", "--output", "cyclonedx-json=$document"]
  ```
  and re-run the F10 snapshot dry run, this time asserting `jq -e .bomFormat` on one document as the plan originally specified.

---

## R2 -- ADVISORY -- `docs/library.md:70` still forward-references Phase 2 as unfinished

**Where:** `docs/library.md:70`.

```
> The accounting family's `search[field]=value` spelling is confirmed against the live API; the business-scoped family's bare `field=value` is inferred from the FreshBooks documentation and has not been exercised live. Phase 2's first business-scoped list endpoint confirms it.
```

**Why it matters.** F8's stated job was to bring the guides in line with the shipped code; the implementer reports fixing two stale forward-references in `library.md` (lines 28 and 133 in the diff) but this third one survived. A public reader has no idea what "Phase 2" is, and the sentence promises a future confirmation for work that shipped. It is also the only remaining `Phase N` reference in any user-facing doc -- `grep -n 'Phase [0-9]' README.md docs/{mcp,library,authentication,getting-started,building}.md` returns exactly this one line.

**Fix.** Rewrite without the phase reference, and state the current confirmation status honestly, e.g.: `> The accounting family's `search[field]=value` spelling is confirmed against the live API; the business-scoped family's bare `field=value` is taken from the FreshBooks documentation and has not been exercised live.` If a business-scoped list has in fact been exercised live since, drop the caveat instead.

---

## R3 -- ADVISORY -- `NewRootCmd`'s doc comment still lists `docs` as an unconditional subcommand

**Where:** `cli/internal/cmd/root.go:21-22`.

```go
// NewRootCmd builds the freshbooks root command: every global flag, the
// non-registry commands (auth, config, api, version, docs), and the full
// 168-command registry tree.
```

**Why it matters.** After F4, `docs` exists only under `-tags docsgen`, attached through the `extraCommands` hook. The comment now describes a tree the default build never produces, which is exactly the confusion the build-tag split exists to prevent. Small, but it is on the function the refactor changed.

**Fix.** `... the non-registry commands (auth, config, api, version, plus any build-tag-gated extras registered in extraCommands -- the hidden docs command under -tags docsgen), and the full 168-command registry tree.`

---

## What I verified and found correct (no action needed)

**The `docsgen` refactor (F4/D6) is sound, including the equivalence question the brief flagged.**

- `cli/internal/docsgen/docsgen.go` is a faithful move of the deleted `cli/internal/cmd/docsgen.go`: `linkHandler`, `Generate`/`renderTree` (formerly `GenerateDocs`/`renderDocsTree`), `DisableAutoGenTag`, the `IsAvailableCommand()`/`IsAdditionalHelpTopicCommand()` filter and the name sort are byte-for-byte the same logic. Only the header text was reflowed (Q21) and the Q12/Q13/Q15/Q17/Q18 edits applied.
- The tag gating is real: `docs_cmd.go` carries `//go:build docsgen` and is the only non-test importer of `cli/internal/docsgen`; `hooks.go` is untagged so `extraCommands` exists in every build; `root.go` iterates it. Registration is deterministic -- a single `init()` appending one entry, iterated in a fixed order.
- **The untagged drift test is genuinely equivalent to `scripts/docs.sh`, and the equivalence is itself tested.** `docs_drift_test.go` has no build tag, so `scripts/check.sh`'s new `-tags integration,docsgen` second pass compiles it *with* the tag -- meaning `TestDocsUpToDate` runs once against a root without `docs` and once against a root with it, both compared byte-for-byte against the committed `docs/cli.md`. That closes the loop the brief was worried about; no extra assertion is needed.
- The `InitDefaultCompletionCmd()` side-effect reproduction is correct for cobra v1.10.2. Read `completions.go`: the `args`-sensitive branch only fires when `hasSubCommands` is false, and this root has ~170 subcommands, so the no-arg call behaves identically to `ExecuteC`'s `InitDefaultCompletionCmd(args...)`. Ordering differs between the two paths (docs.sh adds `help` then `completion`; the test adds `completion` then `help` inside `GenMarkdownCustom`) but both `renderTree` and cobra's SEE ALSO sort children by name, so output is order-independent. `docs/cli.md` contains the five `## freshbooks completion*` sections and correctly omits `help` (cobra's `IsAvailableCommand()` excludes a parent's own `helpCommand`).
- `docs_test.go` is tagged and therefore still executed by the gate's second pass. Its `[edge] docs is hidden from the generated reference itself` case is the guard that keeps the tagged and untagged trees producing identical bytes.
- D6 acceptance holds structurally: `cli/go.mod` keeps `cobra/doc`'s `go-md2man` and `blackfriday` as indirects (the untagged `docsgen` package imports `cobra/doc`, so `go mod tidy` must), but nothing reachable from `./cmd/freshbooks` imports that package without the tag. The implementer's `go version -m | grep -c` -> `0` proof is consistent with the code.

**The version fallback (F5/D7).**

- `resolveVersion` is placed in the tested seam in both binaries -- `cli/internal/cmd.Run` (`root := NewRootCmd(resolveVersion(version))`) and `mcp/cmd/freshbooks-mcp.run` -- and both `main.go` files remain a single `os.Exit(...)` statement.
- No behaviour change when ldflags are set: the first branch returns `version` untouched for anything `!= "0.0.0-dev"`, so a goreleaser build (`-X main.version={{.Version}}`) never reaches `ReadBuildInfo`.
- All four branches of the injected reader are tested in both modules (`ok==false`, `""`, `"(devel)"`, a real version) plus the ldflags passthrough. The `readBuildInfo` package var carries the Q22-style safety comment, and I confirmed `grep -rn 't.Parallel' cli/` returns only those comments -- no test in the module actually calls it, so the seam is safe as written.

**The `--login-timeout` rename (F7/Q12).** Registration (`auth_cmd.go:153`), the docs-header prose (`docsgen.go:80`), the regenerated `docs/cli.md` (lines 61 and 303), and `cli/CHANGELOG.md:12` all use the new spelling. `grep -rn 'auth login --timeout'` across the tree returns nothing. The flag has no test that names it directly, but `docs/cli.md:303` (`--login-timeout duration ...`) is locked by the drift test, so a silent rename back would fail the gate.

**Test quality of the Q4/Q20 folds.** The `len(DefaultScopes) != 44` pin plus two literal scope strings genuinely kills the mutation-blind `len(scopeObjects)*2` tautology. The Q20 rework is correct and, importantly, its central claim is true: I traced `buildClient` in `cli/internal/cmd/state.go:266` and the `if dryRun { return s.buildDryRunClient(...) }` early return does precede `clientIDCredentials()`, so dropping `FRESHBOOKS_CLIENT_SECRET` from the `config view` / `auth status` / `--dry-run` scenarios removed a genuinely vacuous assert rather than real coverage. The three HTTP-issuing scenarios each gained a positive marker (`"freshbooks request"` in the debug log; `POST` + `ACM000TEST` for the dry run) and kept the client-secret assertion. No `t.Skip`, no committed `-run` filters, no new non-determinism anywhere in the diff.

**Release plumbing.** `--skip=publish,validate` with `GORELEASER_CURRENT_TAG` matches D1 and the workflow comment states the reasoning; `project_name` is set per module so `freshbooks-mcp_*` and `freshbooks_*` archives cannot collide; `GOWORK=off` with no `replace` directive anywhere means the release binary resolves the lib from the proxy (and `dbd898b28413`, the pseudo-version both modules now pin, is `origin/main` HEAD, so it is fetchable). Actions are pinned to the SHAs D4 named with `# vX.Y.Z` comments; `upload-artifact`/`download-artifact`/`goreleaser-action` are gone and the `release` job re-runs `changelog-section.sh` itself; permissions are `contents: read` everywhere except the `release` job. `TAG_NAME` reaches `run:` through `env:` rather than direct interpolation, and the only interpolated values (`module`, `version`) are validated by `guard` before use.

**Changelogs.** All three module files satisfy D10's acceptance: `grep -c '^## \['` is `1` for each, and `scripts/changelog-section.sh <module> Unreleased` exits 0 with a non-empty body (6852 / 3782 / 3945 bytes). Each is a coherent first-release `### Added` grouped by area, with no `### Changed`/`### Fixed` pre-release churn left. Spot checks against the code confirm the bullets describe shipped behaviour (rotation write-back, retry with `Retry-After`, redaction, the `-o` shadow, the exit-code ladder, 213 keys / 36 services). Every line is unwrapped. The root `CHANGELOG.md` `### Changed` bullet accurately describes D1-D4 and D8. The only defect is the CycloneDX claim (R1).

**Docs accuracy (mechanical sweep).** I extracted the backticked identifiers from `README.md`, `docs/getting-started.md`, `docs/building.md`, `docs/library.md`, `docs/authentication.md`, `docs/mcp.md`, and the `docs/cli.md` header, and checked each against source rather than by eye. Everything resolves with the exact spelling: `freshbooks.NewClient` / `WithTokenSource` / `auth.StaticTokenSource` / `auth.NewTokenSource`; `Identity.Me` returning `[]Membership` whose `AccountID` (a `string` type) and `BusinessID` (an `int64` type) make the `%s`/`%d` in the getting-started snippet correct; `AccountID`/`BusinessID`/`BusinessUUID`; CLI `auth login`, `identity me`, `config set-context --account/--business/--business-uuid`, `--callback-port`, `--no-browser`, `--show-secrets`, and `--base-url` (which is in fact `MarkHidden`, matching the Q18 note); MCP `serve`/`version`/`tools`, `--transport`/`--addr`/`--path`/`--log-level`/`--log-format`, every `FRESHBOOKS_*` and `FRESHBOOKS_MCP_*` name in `mcp/internal/config/config.go`, and the tool names `identity_whoami`, `identity_applications`, `identity_create_application`, `identity_update_application`, `identity_delete_business`. `README.md:57`'s link target `docs/agentic-transformation.md` exists. `go install ...@latest` is correct for these submodules given `cli/vX.Y.Z`-shaped tags. No dead references found beyond R2.

**Hygiene.** `grep -nP '[^\x00-\x7F]'` over `README.md`, `docs/*.md`, and all four changelogs prints nothing; no smart quotes, em dashes, or arrows. `scripts/redaction-check.sh` -> `redaction-check: clean`. `git grep` for the operator home path over tracked files -> nothing; no internal hostnames, vault item names, or real account ids anywhere in the diff. No `// inventory:` comment was added, removed, or reworded (`git diff main...phase-5/release -- '*.go' | grep -c '// inventory:'` -> 0). `go.work.sum` has no diff, consistent with the implementer's note. Commit scopes follow `CLAUDE.md` and the fold list, in order, one per fold.

## Suggested fix order

1. R1 in one commit touching the five files (`docs: correct the SBOM format to SPDX`), or the goreleaser-config alternative plus a re-run of the F10 SBOM check.
2. R2 and R3 folded into the same commit.

None of these touch Go behaviour, so a re-gate should be a formality -- but `docs/cli.md` is untouched by all three, so the drift test stays green either way.
