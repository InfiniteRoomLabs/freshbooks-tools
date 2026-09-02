# Phase 5 security review -- `phase-5/release`

Lane: security (read-only). Branch `phase-5/release`, 11 commits `1f0abbd..8b13939`, `git diff main...phase-5/release`. Repo referred to as `<repo root>`. No files outside this report were written; no gate, test, or build task was run. Scratch artifacts (`/tmp/fbsec-*`) removed; `git status --porcelain` was empty before and after.

## Verdict: **PASS**

No BLOCKING findings. Five ADVISORY findings, none of which should hold the merge. The release surface is materially stronger than it was on `main`: actions are SHA-pinned and verified, the cross-job artifact channel is gone, the release binary no longer links the markdown stack, and every dependency change is a same-path version increase.

---

## Findings

### A1 -- ADVISORY: no tag-creation ruleset; `guard` accepts any commit that is an ancestor of `main`

`.github/workflows/release.yml:45-48`

```
git fetch origin main
git merge-base --is-ancestor "$GITHUB_SHA" origin/main
```

Evidence: `gh api repos/InfiniteRoomLabs/freshbooks-tools/rulesets` returns `[]` -- there is no tag protection. `gh api .../branches/main/protection` shows `allow_force_pushes: false`, `allow_deletions: false`, required checks `lib,mcp,cli`, `strict: true`, but `enforce_admins: false`.

The ancestry check is the right check and it fails closed (a force-push of `main` that orphans the tag commit makes the guard fail, not pass; a tag pushed from a fork never triggers this workflow at all, because `on.push.tags` only fires in the repo that owns the ref). What it does not constrain is *which* ancestor. Anyone with write access can create `cli/v0.2.0` pointing at an arbitrary older commit on `main` -- for example a commit from before a security fix -- and the workflow will build and publish it as a new version, because the only version-shaped assertion is that `<module>/CHANGELOG.md` contains a matching `## [X.Y.Z]` section, and the changelog on that older commit is whatever it was then. `enforce_admins: false` also means an admin can push directly to `main` and skip the required checks entirely, so "on main" is a weaker statement than it reads.

Fix: add a repo ruleset for `refs/tags/freshbooks/v*`, `refs/tags/mcp/v*`, `refs/tags/cli/v*` restricting tag creation (and forbidding tag update/deletion), and consider tightening the guard to require the tag commit to be `origin/main`'s HEAD, or within a small ancestry window, rather than any ancestor. Flipping `enforce_admins: true` on `main` costs nothing on a single-maintainer repo and makes the guard's premise true.

### A2 -- ADVISORY: the `contents: write` job checks out with credentials persisted

`.github/workflows/release.yml:73-75`

```yaml
- uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
  with:
    fetch-depth: 0
```

`actions/checkout` defaults to `persist-credentials: true`, which writes an `http.https://github.com/.extraheader` Authorization entry carrying the job's `GITHUB_TOKEN` into `.git/config`. This is the one job in the repo with `contents: write` (line 71), and it then runs three third-party binaries inside that checkout: `goreleaser`, the `syft` it shells out to for SBOMs, and `go build` with the modules' full dependency graph. Any of those gaining code execution inherits a push-capable credential sitting on disk, not just the environment's token.

`goreleaser` needs git metadata (`.CommitTimestamp`, tag info) but reads it from the local clone, not the remote, so credentials are not required for the build. `gh release create` authenticates from `GITHUB_TOKEN` in its own step's `env:`, independently of `.git/config`.

Fix: add `persist-credentials: false` to the release job's checkout. The `guard` and `ci` checkouts are `contents: read`, so the same change there is optional.

### A3 -- ADVISORY: `usage` is the one unverified tool, and it is in the release path twice

`mise.toml:6` (`usage = "6.0.0"`), `scripts/changelog-section.sh:1` (`#!/usr/bin/env -S usage bash`), consumed at `.github/workflows/release.yml:57` and `:84`.

`mise tool` reports, per pinned tool:

| tool | backend | verification |
|---|---|---|
| `go` | `core:go` | checksum (sha256) |
| `goreleaser` | `aqua:goreleaser/goreleaser` | checksum (sha256), github_attestations, cosign |
| `golangci-lint` | `aqua:golangci/golangci-lint` | checksum (sha256), github_attestations |
| `actionlint` | `aqua:rhysd/actionlint` | checksum (sha256), github_attestations |
| `aqua:anchore/syft` (new this phase) | `aqua:anchore/syft` | checksum (sha256), cosign |
| `usage` | `aqua:jdx/usage` | **[none]** |

The new `aqua:anchore/syft` pin the lead asked about verifies both a sha256 checksum and a cosign signature, per the aqua registry metadata mise resolves -- that is a good pin and D3's explicit-backend form (`aqua:anchore/syft` rather than the `syft` short name) correctly nails the backend down. `usage` is the outlier: mise fetches and executes it with no checksum and no signature, and it is the interpreter for `changelog-section.sh`, which runs in `guard` (fail-fast) and again in `release` (where the job holds `contents: write`).

This is pre-existing -- `usage` was already pinned on `main`, and `guard` already ran `changelog-section.sh` there -- so it is not a regression introduced by this phase. Recording it because the lead asked what mise actually verifies, and because the release path's trust now rests on one unverified binary.

Fix (not for this merge): either drop the `usage` shebang from `changelog-section.sh` (it takes two positional args and needs no spec) so the release path stops depending on an unverified tool, or pin `usage` by a checksum mise can enforce.

### A4 -- ADVISORY: the onboarding doc's only HTTP example binds to every interface

`docs/getting-started.md:101`

```sh
freshbooks-mcp serve --transport http --addr 0.0.0.0:8080
```

The shipped default is `127.0.0.1:8080` (`docs/mcp.md:81`), and the complete warning exists -- but only in the reference doc, at `docs/mcp.md:52`: "behind a TLS-terminating reverse proxy -- the server itself speaks plain HTTP; nothing about stateless mode changes what carries the bearer token over the wire, so it must not be exposed without TLS in front of it." `getting-started.md` carries only the half-sentence "run the stateless HTTP transport behind TLS" ahead of a copy-pasteable command that overrides a safe default with an unsafe one, followed by a `curl` against a *different* host (`https://mcp.example.com/mcp`), so nothing in the block visibly connects the two.

The exposure is bounded -- `mcp/internal/server/server.go:110-119` rejects any request without a usable bearer with `401` + `WWW-Authenticate` before JSON-RPC parsing, and `/healthz` is the only unauthenticated route -- so this is plaintext-token-on-the-wire and reachable-surface, not an open door. It is still the pattern where the doc a new user reads first carries the least safe example.

Fix: use `--addr 127.0.0.1:8080` in `getting-started.md` (a same-host TLS proxy reaches loopback fine) and inline one sentence of `docs/mcp.md:52`'s warning, or drop the `--addr` override entirely and let the default stand.

### A5 -- ADVISORY: an unmatched asset glob is passed literally to `gh release create`

`.github/workflows/release.yml:106-110`

```sh
gh release create "$TAG_NAME" --verify-tag --notes-file /tmp/release-notes.md \
  ${{ needs.guard.outputs.module }}/dist/*.tar.gz \
  ${{ needs.guard.outputs.module }}/dist/*.zip \
  ${{ needs.guard.outputs.module }}/dist/checksums.txt \
  ${{ needs.guard.outputs.module }}/dist/*.sbom.json
```

Bash's default `nullglob`-off behaviour passes an unmatched pattern through as a literal string. If a future `.goreleaser.yaml` edit drops the windows targets (no `*.zip`), or the `sboms:` stanza stops producing `<archive>.sbom.json`, or `syft` fails in a way goreleaser tolerates, the step reaches `gh` with a nonexistent path. `gh release create` creates the release and then uploads assets, so the plausible failure shape is a *published* release missing its checksums or SBOMs rather than a clean abort -- and consumers verifying downloads against `checksums.txt` would find it absent.

The implementer's F10 dry runs are good evidence that the globs match today: `docs/phases/5/reports/implementer.md:118-121` records six archives per module with windows as `.zip`, one `.sbom.json` per archive, a shared `checksums.txt`, and `sha256sum -c checksums.txt --ignore-missing` passing all 12 entries. This is about the failure mode later, not the current shape.

Fix: `shopt -s failglob` (or an explicit `ls`/`test` guard listing the expected asset count) in the publish step, so a missing artifact aborts before the release is created rather than after.

---

## What was checked and found clean

**Release workflow trust.** Permissions are least-privilege per job: `guard` and `ci` are `contents: read` (lines 15, 68), `release` alone is `contents: write` (line 71). The repo's own default is already `read` (`gh api .../actions/permissions/workflow` -> `default_workflow_permissions: "read"`), so nothing relies on a permissive default. The reusable-workflow call at line 64 passes `contents: read` and `ci.yml` itself declares `permissions: contents: read` at the top level (`ci.yml:9-10`), so the called workflow cannot exceed the caller's grant.

**Tag injection.** `github.ref_name` appears in exactly three places (`release.yml:27`, `:104`, `:116`) and every one is an `env:` binding, never an inline `${{ }}` inside a `run:` body -- so a crafted tag name reaches the shell only as `"$TAG_NAME"`, always quoted. The only `${{ }}` interpolations inside `run:` bodies are `steps.parse.outputs.{module,version}` and `needs.guard.outputs.{module,version}`, both of which are validated *before* they are written to `$GITHUB_OUTPUT`: `version` must match `^[0-9]+\.[0-9]+\.[0-9]+$` (line 31) and `module` must be one of `freshbooks|mcp|cli` (lines 35-41). Neither can carry a quote, a newline, a `$`, or a `;`. Git's own ref-name rules independently forbid whitespace, `~^:?*[` and control characters in a tag, so `$GITHUB_OUTPUT` line-injection via the tag is not reachable either. `--verify-tag` is present on both `gh release create` calls (lines 106, 118), so a release cannot be created against a tag the remote does not have.

**The `--skip=validate` trade-off is stated honestly.** The workflow comment (lines 86-91) and `docs/building.md`'s release-flow section both name exactly what is skipped: goreleaser's `git describe --exact-match` check, which cannot pass against a prefixed tag, and its dirty-tree check, which is moot on a fresh CI checkout where `guard` has already enforced semver and on-`main`. `--skip=publish` is separately justified (goreleaser would otherwise create a release on the bare `vX.Y.Z` tag it was told about, not on the real prefixed one). No overclaiming.

**Improvement over `main`.** Dropping `actions/upload-artifact` / `actions/download-artifact` (present at `main`'s `release.yml:59-62` and `:82-85`) removed a cross-job channel through which a `contents: read` job handed a file to a `contents: write` job. The release notes are now re-derived in the write-scoped job from its own pristine checkout (`release.yml:82-84`). That is a real reduction in trust surface, not just fewer steps.

**Pinned actions -- both SHAs verified live.**

| action | claimed | `gh api repos/<r>/git/ref/tags/<t>` |
|---|---|---|
| `actions/checkout` | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` -- matches |
| `jdx/mise-action` | v4.3.0 | `c2a87611a18de5b3828c5652fe268e992400cb5c` -- matches |

Every one of the 11 `uses:` lines across both workflows is a 40-hex SHA with a version comment, except `release.yml:64` (`uses: ./.github/workflows/ci.yml`), which is a local path and correctly not pinned. No floating tags remain. `goreleaser/goreleaser-action` is gone entirely, so the only third-party actions left are `checkout` and `mise-action`.

**Supply chain -- the dependency diff.** Every change in `cli/go.mod`, `cli/go.sum`, `mcp/go.mod`, `mcp/go.sum` is a version increase on an existing module path. No new direct or indirect module appears; no module path changes; the `go.sum` additions correspond one-to-one to the bumped versions, and the superseded `h1:` lines are removed while the older `/go.mod` hashes are correctly retained where the graph still references them. `go.work` and `go.work.sum` are untouched by this branch (`git diff --stat main...phase-5/release -- go.work go.work.sum` is empty), which matches D5's "commit it in F1" expectation resolving to no net change.

Bumps: `pflag` 1.0.9->1.0.10, `go-md2man/v2` 2.0.6->2.0.7, `go.yaml.in/yaml/v3` 3.0.4->3.0.5, `segmentio/asm` 1.1.3->1.2.1, `x/oauth2` 0.35.0->0.36.0, `x/sync` 0.20.0->0.22.0, `x/sys` 0.41.0->0.47.0 (mcp only; cli was already at 0.47.0), plus the lib pseudo-version to `v0.0.0-20260902041524-dbd898b28413` in both consumers.

The `x/sys` bump is a genuine security fix, verified against the Go vulnerability database rather than taken on faith: `GO-2026-5024` / `CVE-2026-39824` (integer overflow in `NewNTUnicodeString`, `golang.org/x/sys/windows`) is `{"introduced":"0"},{"fixed":"0.44.0"}`, so `mcp`'s 0.41.0 was inside the affected range and 0.47.0 clears it. Symbol reachability is windows-only, so exposure was likely nil, but the pin was genuinely stale.

`mise.toml` pins are exact -- no `latest` anywhere (`go = "1.26.6"`, `golangci-lint = "2.13.2"`, `goreleaser = "2.18.0"`, `actionlint = "1.7.12"`, `usage = "6.0.0"`, `"aqua:anchore/syft" = "1.51.1"`). The `goreleaser` 2.17.1->2.18.0 and `golangci-lint` 2.13.1->2.13.2 bumps resolve through the aqua backend with checksum + attestation/cosign verification (table in A3), so both are real published releases, not typosquats. Verification model per tool is in A3; `usage` is the only gap.

**Artifacts.** Both `.goreleaser.yaml` files set `CGO_ENABLED=0` and `GOWORK=off` in `builds[].env` (`cli/.goreleaser.yaml:14-16`, `mcp/.goreleaser.yaml:14-16`), `mod_timestamp: "{{ .CommitTimestamp }}"` for reproducible archive timestamps, `checksum.name_template: "checksums.txt"`, and `sboms: [{artifacts: archive}]`. `GOWORK=off` is the security-relevant one: it forces the release binary to build against the module's own `go.mod` -- the same lib pseudo-version a `go install ...@tag` user resolves from the proxy -- rather than the workspace's sibling checkout, so what ships matches what a user building from the tag gets. `project_name` is set per module (`freshbooks-mcp`, `freshbooks`), which is what stops the two modules' archives from colliding in a shared release namespace.

`dist/` is gitignored (`.gitignore:2`, `dist/`, verified to cover both module subdirectories: `git check-ignore -v cli/dist mcp/dist` reports both matched by that line) and nothing under any `dist/` is tracked (`git ls-files | grep dist/` is empty).

**D6 verified by building, not by reading.** `mise exec -- env GOWORK=off CGO_ENABLED=0 go -C cli build -o /tmp/... ./cmd/freshbooks` then `go version -m` on the result: zero lines matching `md2man` or `blackfriday`. The same source built with `-tags docsgen` lists both (`go-md2man/v2 v2.0.7`, `blackfriday/v2 v2.1.0`). The seam is clean: `cli/internal/cmd/hooks.go` holds an untagged `var extraCommands []func(*cobra.Command) *cobra.Command`, `cli/internal/cmd/docs_cmd.go` is `//go:build docsgen` and appends to it from `init()`, `cli/internal/cmd/root.go:44-46` iterates it, and `cli/internal/docsgen` is the module's only non-test importer of `cobra/doc`. Both builds went to `/tmp`; the working tree was unchanged afterward.

**D7 -- `debug.ReadBuildInfo` cannot leak or panic.** `cli/internal/cmd/version.go:23-32` and `mcp/cmd/freshbooks-mcp/version.go:24-33` are byte-identical in logic. They read exactly one field, `info.Main.Version`, and never touch `info.Settings` (which is where VCS state, build flags, and toolchain paths live, and which is what a leak would come from). The nil guard is correctly ordered: `if !ok || info.Main.Version == ""` short-circuits on `!ok` before dereferencing `info`, so a stripped or non-module binary -- where `ReadBuildInfo` returns `(nil, false)` -- returns the `0.0.0-dev` placeholder rather than panicking. The fallback only fires when `version` is still exactly the placeholder, so a goreleaser build (which always sets `-X main.version`) is never second-guessed.

**`scripts/check.sh` dirty-tree exclusion is narrow -- tested, not assumed.** `scripts/check.sh:146` is `git status --porcelain -- . ':(exclude)docs/phases/*/reports/*'`. I cloned the branch to `/tmp` and planted strays:

| planted | shown with the exclusion? |
|---|---|
| modified tracked file `cli/internal/cmd/root.go` | yes |
| untracked `freshbooks/EVIL.go` | yes |
| untracked `docs/phases/5/evil.md` (sibling of a tracked `reports/`) | yes |
| untracked `docs/phases/9/reports/stray.md` | no (intended) |

One edge worth knowing but not worth fixing: a wholly new untracked `docs/phases/N/` directory whose *only* contents are report files is collapsed by git into `?? docs/phases/N/` and then excluded as a unit -- so a stray dropped alongside a report in a brand-new phase directory would be hidden with it. As soon as that phase directory has any tracked file (which it does the moment the plan is committed), non-report strays inside it are reported normally, as row 3 shows. The exclusion cannot hide anything outside `docs/phases/*/reports/`.

**Secrets and public-repo hygiene.** `scripts/redaction-check.sh` reports `clean`. An independent sweep of the full diff for `client_secret`/`access_token`/`refresh_token` literals, `Bearer <value>`, `gh[pousr]_`, `xox[baprs]-`, `AKIA`, and `-----BEGIN` returned only environment-variable *names*, documentation prose about redaction behaviour, and test fixture identifiers -- no values. A second sweep of every changed file for `100.x` tailnet addresses, `192.168.`, `.lab.`/`.internal.` domains, `infiniteroomlabs.cloud`, `/home/<user>/` paths, Bitwarden `IRL/` item paths, the operator's name or email, and `gitea` returned nothing. Doc example values are obviously synthetic placeholders in angle brackets (`<your account id>`, `<your business id>`, `<a FreshBooks access token>`, `<your client secret>`), never plausible-looking real IDs.

Redirect URIs in docs are HTTPS throughout: `docs/getting-started.md:6` and `docs/authentication.md:35` both give `https://localhost:8765/callback` and both explicitly say the portal rejects `http://localhost`. The only non-HTTPS URLs anywhere in the docs are `127.0.0.1` listen addresses, which is correct.

The doc examples put the env vars first and the flags second, with the reason attached rather than left implicit -- `docs/cli.md:12` (the generated header, authored in `cli/internal/docsgen/docsgen.go`) and the `docs/getting-started.md` quickstart both read "or pass `--client-id <id> --client-secret <secret>` directly, though that puts the secret in `ps` output and shell history -- see Security notes below", and the Security notes section repeats it. That is the Q17 advisory correctly folded.

**Not re-reviewed (unchanged in this diff, covered by Phase 4's security report):** the OAuth/PKCE flow, `FileStore` permissions, transport redirect handling, MCP bearer extraction, and tool input validation. The Phase 5 diff touches none of their logic; the only code changes outside the version fallback and the docsgen move are comment additions (`state.go`, `auth_cmd.go`) and the `--timeout` -> `--login-timeout` flag rename, which removes a shadowing footgun rather than creating one.

## Gate note

`docs/phases/5/reports/security.md` (this file) is written but **not committed**, per the work order. It falls inside the `scripts/check.sh` dirty-tree exclusion verified above, so it will not trip the QA lane's gate run.
