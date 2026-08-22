# Security lane -- Phase 0 (scaffold)

Branch `phase-0/scaffold`, diff `git diff main...phase-0/scaffold` (`b3d69ac..9545506`, 58 files, +8076).
Read-only review. `govulncheck` was run (via `go run golang.org/x/vuln/cmd/govulncheck@latest`); no files were modified, nothing was committed, and the gate/tests/builds were not run.

## Verdict: BLOCK

Two blocking findings. Both are one-line fixes. Nine advisories, none of which need to hold the merge.

Note on scope: `git remote -v` is empty -- nothing in this repo has been pushed anywhere yet. Every hygiene finding below is still fixable in-history at zero cost.

---

## Checklist 1-6: nothing present that violates them

Phase 0 ships no auth, transport, MCP, or CLI logic. Evidence rather than a skip:

1. **Secrets never leak.** The branch's entire non-test Go surface is 14 files (`git diff --name-only ... '*.go' | grep -v _test.go`). A grep across `freshbooks/ mcp/ cli/` for `net/http|crypto/|Authorization|token|secret|password|slog|log\.` returns four hits, all benign: `check.go:7` `"go/token"`, `check.go:9` `"os/exec"`, `check.go:207` `token.NewFileSet()`, and `freshbooks/doc.go:23` the word "token" in a doc comment pointing at `docs/authentication.md`. No logger is constructed anywhere; no `String()` method exists on any type. Nothing can leak because nothing holds a credential.
2. **Credential storage.** No credential file is created. The only file write in the branch is `inventory.go:446` `os.WriteFile(path, data, 0o600)` (the generated `inventory.json`), and test helpers use `0o600`/`0o750`. `cli/internal/config/doc.go` and `cli/internal/auth/doc.go` are package-doc stubs with no code. Nothing to get wrong yet, and the modes already in use are the right ones.
3. **OAuth flow.** Not implemented. No `crypto/rand`, no `net.Listen`, no `state`/PKCE handling in the branch.
4. **Transport.** No `net/http` import anywhere. No `tls.Config`, no `InsecureSkipVerify`.
5. **Trust boundaries.** Two argument-driven surfaces exist, both dev-facing: `inventory` reads `-in`/`-out`/`-inventory`/`-ignore` paths and shells out once at `check.go:188` `exec.Command("go", args...)` where `args` are Go package patterns (`./...`) from the developer's own command line. Both spots carry accurate `#nosec` justifications (`check.go:188`, `check.go:242`, `inventory.go:68`) rather than blanket suppressions. Postman JSON is decoded into typed structs (`postman.go`), including a hand-written `URL.UnmarshalJSON` that handles both schema forms and returns an error rather than panicking. No `unsafe` in the branch.
6. **Stateless MCP.** `mcp/cmd/freshbooks-mcp/main.go` is 27 lines that print a version string. No server, no transport, no `/healthz`.

`.golangci.yml` enables `gosec` and `errcheck` with `default: none` and no exclusion presets -- the linter that would catch regressions in 1-6 is on from day one, which is the right call for a repo that will hold OAuth tokens.

---

## BLOCKING

### 1. The reusable CI workflow inherits `contents: write` during a release, so the whole check gate runs privileged

`.github/workflows/ci.yml:1-8` declares no `permissions:` block. `.github/workflows/release.yml:10-11` sets `permissions: contents: write` at **workflow** level, and `release.yml:65-68` calls the CI workflow:

```yaml
  ci:
    name: ci
    needs: guard
    uses: ./.github/workflows/ci.yml
```

A called reusable workflow inherits the caller's `GITHUB_TOKEN` permissions when it does not declare its own; it can only narrow them, and ci.yml narrows nothing. So on every release run, `mise run check -- <module>` -- which executes `go test`, `golangci-lint`, `goreleaser`-adjacent build scripts, and every `TestMain`, `init()`, and transitive dependency in three modules -- runs with a token that can write to the repository and create releases.

That is the classic build-step-to-repo-write escalation path. It costs nothing to close, and closing it now means Phase 1's `go-sdk` and Phase 2's larger dependency tree never inherit the problem.

**Fix (either, prefer both):**

```yaml
# .github/workflows/ci.yml -- after `on:`
permissions:
  contents: read
```

and narrow the caller so only the job that needs write has it:

```yaml
# .github/workflows/release.yml -- delete the workflow-level permissions block, then:
jobs:
  guard:
    permissions:
      contents: read
    ...
  ci:
    permissions:
      contents: read
    uses: ./.github/workflows/ci.yml
  release:
    permissions:
      contents: write
    ...
```

Note that `guard` currently also runs with `contents: write` while doing nothing but parsing a tag, running `scripts/changelog-section.sh`, and uploading an artifact.

### 2. `docs/phases/0/plan.md:7` commits the operator's absolute home path

```
Work ONLY inside `/home/<operator>/projects/infinite-room-labs/freshbooks-tools` on branch `phase-0/scaffold`
```

(operator username redacted here; the tracked file spells it out.)

`CLAUDE.md` bans this explicitly: "Never commit operator-specific strings: internal/absolute paths." This is a public MIT repo whose whole `docs/phases/` tree is a portfolio artifact people will read.

This is not a judgement call about what counts as sensitive -- I ran the repo's own configured term list against every tracked file in the branch and this file trips **two** of the configured redaction terms. `scripts/redaction-check.sh` exists precisely to catch this and would have caught it; it evidently did not run for commit `9545506`.

**Fix:** replace the absolute path with `<repo root>` (or drop the clause -- the branch name already scopes the work), then confirm with `scripts/redaction-check.sh` on the staged fix. Because nothing has been pushed, a plain amend/fixup is enough; no history rewrite is needed if the fix lands before the first push. If the fix lands after a push, the leak lives in `9545506` forever and needs `git-filter-repo`.

---

## ADVISORY

### 3. The pinned Go toolchain has six known stdlib vulnerabilities, and nothing will ever bump it

`mise.toml:2` pins `go = "1.26.5"`. `govulncheck` on the `cli` module:

```
Vulnerability #1: GO-2026-6218   Found in: net/url@go1.26.5   Fixed in: net/url@go1.26.6
Vulnerability #2: GO-2026-5942   Found in: net@go1.26.5       Fixed in: net@go1.26.6
... plus GO-2026-6091, -6090, -6089, -6088, -5972, -5026 in stdlib@go1.26.5
```

**Symbol-level result is clean: "No vulnerabilities found. Your code is affected by 0 vulnerabilities."** Nothing in Phase 0 reaches these paths, which is why this is advisory and not blocking. Two of them are in `net` and `net/url` -- exactly the packages Phase 1's transport layer will live in, and the release pipeline this branch adds compiles the shipped binaries with this toolchain.

The reason to fix it now rather than later: `.github/dependabot.yml` has `gomod` lanes for all three modules and a `github-actions` lane, but **nothing watches `mise.toml`**. Dependabot does not parse it. So this pin will not self-correct -- "later" means "whenever a human happens to look."

**Fix:** `go = "1.26.6"` in `mise.toml:2`. Consider adding `govulncheck` as a `mise` task and a CI step so this is enforced instead of noticed.

### 4. Third-party action pinned to a mutable major tag

`ci.yml:15,27,41` and `release.yml:51,84` use `jdx/mise-action@v2`. `v2` is a mutable tag: whoever controls that repo can retarget it, and mise-action is the step that then downloads and executes the entire toolchain.

`actions/checkout@v4`, `actions/upload-artifact@v4`, `actions/download-artifact@v4`, and `goreleaser/goreleaser-action@v6` are first-party/trusted-vendor and acceptable on major tags per the work order. `jdx/mise-action` is neither, and it is the highest-privilege step in the file.

**Fix:** pin to a full commit SHA with the version in a trailing comment, e.g. `uses: jdx/mise-action@<40-char-sha> # v2.4.4`. The existing `github-actions` dependabot lane will keep the SHA current and annotate the bumps.

### 5. `scripts/redaction-check.sh` substring-matches, and I can prove the false positives

`redaction-check.sh:34` uses `grep -qiF -- "$search"` with no word boundaries. Scanning the tree, `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md:106` flags -- and that line is:

```
- **Method vocabulary:** `List`, `All`, `Get`, `Create`, `Update`, `Delete`, plus resource-specific verbs ...
```

A short configured term matched an ordinary English word in an API-vocabulary sentence. That is a false positive, and it is the pre-existing spec file, not something this branch introduced. But a hygiene gate that cries wolf on `Delete` is a gate people learn to `--no-verify` past -- which is plausibly how finding #2 got committed.

The script does get the important thing right: `redaction-check.sh:35` prints `possible leak in $file (term #$i)` -- the index, never the term. It does not leak the term list. Good.

Two secondary gaps: it exits 0 when the resolver is absent (`:10-13`, deliberate and documented for outside contributors, but it means the gate never runs in CI), and it scans only staged file contents -- never commit messages, which are equally public.

**Fix:** word-boundary matching for short terms, e.g. `grep -qiE "\b$(printf '%s' "$search" | sed 's/[][\.^$*+?(){}|\\/]/\\\\&/g')\b"`, or a minimum-length threshold below which terms require boundaries. Add `git log --format=%B` over the branch's commits to the scan.

### 6. `actionlint` is pinned but never wired to anything

`mise.toml:5` pins `actionlint = "1.7.12"`, and `docs/phases/0/reports/implementer.md:91` records it as run manually. There is no `[tasks.actionlint]`, `scripts/check.sh` has no actionlint step, and no CI job invokes it. Workflow files -- the highest-privilege code in the repo, per finding #1 -- are the one thing the gate does not check.

**Fix:** add an `actionlint` step to `scripts/check.sh`'s `all` path (it is a no-op for module-scoped invocations) and let CI inherit it.

### 7. Expired third-party demo credentials are duplicated into `inventory.json`

`freshbooks/internal/inventory/testdata/inventory.json` carries 36 fully-signed HS256 JWTs, e.g. `eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.<payload>.<sig>` decoding to `{"accountid":"wkMd2g","userid":1,...,"iat":1556130715,"exp":1558722715}` -- issued April 2019, expired May 2019. Also present: `"api_key": "pk_live_NuMBerSanDmoRELetTers"`, `"password": "jjjjjjjjjjjjjjjjjjj"`, `"client_id": "140702"`, and 30 unique email addresses, all Mockaroo-style fakes (`acraythorne5b@constantcontact.com`, `mjolley58@moonfruit.com`) or FreshBooks' own (`api.freshbooks@freshbooks.com`).

I checked this against the work order's specific question and the answer is clean: **none of it is operator data.** Every value traces to FreshBooks' own published Postman collection, which was already committed on `main` at `5f800ac` -- before this branch -- and `inventory_test.go:557` asserts `inventory.json` is byte-identical to what the tool emits from that collection, so it is mechanically derived, not hand-entered. No IRL account IDs, no business IDs, no real tokens. The `pk_live_` prefix looks alarming in a grep but the value is the literal string "NuMBerSanDmoRELetTers".

No action required. Logged so the next person who greps this repo and finds a `pk_live_` key and 36 JWTs does not have to redo the work. Worth a one-line `README` note in `testdata/` if it comes up again.

### 8. `scripts/branch-protection.sh` grants weaker protection than it reads like

`branch-protection.sh:16-17` sets `enforce_admins=false` and `required_approving_review_count=0`, so `main` stays directly writable by an admin with no review. For a solo repo that is a legitimate choice, but it does mean release.yml's "tag must be on main" guard (`release.yml:46-49`, which is otherwise correct -- `git merge-base --is-ancestor "$GITHUB_SHA" origin/main` after a `fetch-depth: 0` checkout) rests on a branch anyone with admin can force a commit onto. `allow_force_pushes=false` and `required_linear_history=true` are set, which is the right half of it.

Separately, `:12` passes the boolean `required_status_checks[strict]` via `-f` (string) rather than `-F` (typed). GitHub will most likely 422 on the string `"true"` rather than silently applying weaker protection -- but I could not verify that read-only, and a silent downgrade would be security-relevant. Worth one manual `gh api repos/.../branches/main/protection` read-back after the next run to confirm what actually landed.

### 9. Smaller items

- **No `mise` lockfile.** `mise.toml` pins exact versions (correct, and better than `latest`), but there is no `mise.lock`, so the downloaded toolchain artifacts have no integrity pinning. mise has no release-age gate of its own. Low risk given exact pins; worth adding when `mise` lockfile support is stable.
- **`scripts/build.sh:26,38`** interpolates `git describe --tags --always --dirty` output into `-ldflags "... -X main.version=${version}"`. Not a shell injection (no `eval`, single argv element), but a tag containing whitespace would corrupt the ldflags string. Tags are repo-controlled; noted for completeness.
- **`check.go:188`** `exec.Command("go", ...)` resolves `go` via `PATH`. Correct for a dev tool; only exploitable in an already-compromised environment.
- **Anti-injection patterns are used correctly** where it counts: `release.yml:27-28` and `:106-107` pass `github.ref_name` through `env:` rather than interpolating it into the `run:` body, and the `${{ steps.parse.outputs.* }}` expansions at `:58`, `:93`, `:101` are all constrained upstream by the `^[0-9]+\.[0-9]+\.[0-9]+$` regex and the `freshbooks|mcp|cli` case at `:32-42`. `ci.yml` triggers on `pull_request`, not `pull_request_target`, so fork PRs get a read-only token and no secrets. This part of the workflow was done well.

---

## Supply chain summary

| Module | Direct deps | `go.sum` | `govulncheck` |
|---|---|---|---|
| `freshbooks` | none (stdlib only) | absent, correct | n/a |
| `mcp` | none | absent, correct | n/a |
| `cli` | `spf13/cobra v1.10.2` | present, 10 lines | clean (0 affecting) |

`cli`'s tree is `cobra v1.10.2` + `pflag v1.0.9` + `mousetrap v1.1.0` (indirect), with `go-md2man`, `blackfriday`, `yaml.v3`, and `check.v1` present as `/go.mod`-only hashes. That matches `CLAUDE.md`'s declared dependency budget exactly -- no unjustified additions. `freshbooks` being genuinely stdlib-only at Phase 0 is the strongest supply-chain fact in this branch.
