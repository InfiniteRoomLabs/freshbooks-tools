# Building freshbooks-tools

## Toolchain

Everything runs through [mise](https://mise.jdx.dev/). `mise.toml` pins exact versions for `go`, `golangci-lint`, `goreleaser`, `actionlint`, `shellcheck`, `usage`, and `syft` (release SBOMs) -- never `latest`. Run `mise install` once per clone (or after pulling a `mise.toml` change) to fetch them.

The repo is a Go workspace (`go.work`) with three modules: `freshbooks/`, `mcp/`, `cli/`. Run `go` commands from inside a module directory, or with `-C <module>`.

## The gate

```sh
mise run check                # all three modules
mise run check -- freshbooks  # one module
```

This runs, per module: `fmt-check` (gofmt), `vet` (go vet), `lint` (golangci-lint), `test` (`go test -race -coverprofile -covermode=atomic ./...`, plus a pass with `-tags integration,docsgen` -- `docsgen` is a no-op tag outside `cli/`, where it compiles the hidden `docs` command and its tests, see "Docs generation" below), `cover` (the 90% coverage floor, via `scripts/coverage-gate.sh`), and `inventory-check` (freshbooks only -- the `// inventory:` parity check). It then cross-compiles `freshbooks-mcp` and the `freshbooks` CLI for the modules actually in scope (both, unless the invocation filtered to a module that isn't `mcp`/`cli`) across `{linux,darwin,windows} x {amd64,arm64}` into `dist/` (gitignored), runs the module-independent steps, and finishes with a `DIRTY TREE:` banner plus `git status --porcelain` if the working tree isn't clean -- a passing gate on a dirty tree is not a passing gate.

The module-independent steps are `actionlint` (`.github/workflows/*.yml`), `shellcheck` (`scripts/*.sh` at `-S warning`), `redaction-selftest`, `release-selftest`, and `readme-drift-check` (README.md's Status column against the newest tags -- read-only: it renders into a temp file and diffs). A bare `mise run check` runs them; `mise run check -- <module>` skips them and says so, because CI invokes the gate once per module and would otherwise pay for the same work three times. CI runs them exactly once, in its own `repo-wide` job (`mise run repo-wide`), which is a required check on `main`.

The dirty-tree check excludes `docs/phases/*/reports/`: the QA lane writes its report while this gate is still running, and that in-flight write isn't the kind of dirty tree the banner exists to catch.

Each step is also its own task (`mise run fmt-check`, `mise run vet`, `mise run lint`, `mise run test`, `mise run cover`, `mise run inventory-check`, `mise run vuln`, `mise run build`, `mise run actionlint`, `mise run shellcheck`, `mise run readme-drift-check`, `mise run repo-wide`), all backed by `scripts/check.sh`. `scripts/build.sh [modules...]` does the cross-compile alone (default both `mcp` and `cli`; `mise run check -- freshbooks` therefore builds nothing); `scripts/coverage-gate.sh <threshold> <coverprofile>` is the standalone coverage check.

`cmd/<binary>/main.go` entry points are excluded from the gated coverage number -- scoped to that path shape specifically, not any file named `main.go` and not by directory. They're thin flag-parsing + `os.Exit` wiring that a test process cannot exercise (`os.Exit` would kill the test binary), so the substantive logic is required to live in a tested `run()`-style function instead. `internal/cmd/` (cobra wiring, fully tested) and `freshbooks/internal/inventory/main.go` (the inventory CLI's flag parsing and report rendering, also fully tested, and not under a `cmd/` directory) both stay counted. A coverage profile with no measurable statements left after that filter is a hard failure, not a vacuous pass -- a module that's supposed to have real code and doesn't measure any is exactly the situation the gate exists to catch.

## Lint

`.golangci.yml` (v2 config schema) enables `errcheck`, `govet`, `staticcheck`, `revive` (with the exported-identifier rule), `gosec`, and `misspell`, with no default exclusions -- every finding is a real finding. A justified `gosec` suppression uses `//nolint:gosec // <reason>`, never a blanket disable.

## Vulnerability scanning

`mise run vuln` runs `govulncheck` (pinned at `golang.org/x/vuln/cmd/govulncheck`) per module, as a step in `mise run check` right after `cover`. `mise.toml`'s exact `go` pin has no dependabot lane watching it (dependabot only understands `go.mod`/`go.sum` and GitHub Actions), so this is the check that would have caught the toolchain shipping with known stdlib vulnerabilities.

## Branch protection

`scripts/branch-protection.sh` applies required-status-checks protection to `main` via `gh api` (job names `lib`, `mcp`, `cli` from `.github/workflows/ci.yml` are the required checks), plus `required_linear_history` and no force-pushes. GitHub does not read this from any file in the repo -- it has to be applied once, out of band, by someone with admin access.

## Release flow

`mise run release -- <subcommand> [args] [flags]` (`scripts/release.sh`) automates the release sequence -- v0.1.0 through v0.3.0/0.1.2 were all done by hand, one `git`/`gh` command at a time; this runs the same sequence. Subcommands:

| Subcommand | Does |
|---|---|
| `preflight` | main branch, clean tree, `gh auth status` scopes, `main`'s CI is green, toolchain resolvable, and prints (never applies) the tag-ruleset `gh api` call |
| `cut <module> <version>` | for `freshbooks`: changelog cut, commit, push, CI watch, tag, tag push, Release watch, verify. For `mcp`/`cli`: CI watch, tag, tag push, Release watch, verify (run `bump` first so the module's own changelog/go.mod are already committed) |
| `bump <lib-version> [--binary-version A.B.C]` | in `mcp/` and `cli/`: `go get`+`go mod tidy` against the new `freshbooks` tag, a `### Changed` changelog line, the changelog cut, `fmt-check`/`vet`/`lint`/`test`/`cover`, one shared commit, push, CI watch |
| `verify <tag>` | release view (not draft, named, asset count), and for `mcp`/`cli` the download+checksum+extract+run, `go install`+run, cli-only `md2man`/`blackfriday` absence check, and a dogfood copy into `$RELEASE_LOCAL_BIN` (default `~/.local/bin`) |
| `docs` | rewrites the README Status column from `git tag --list '<module>/v*'` |
| `all <lib-version> [--binary-version A.B.C]` | preflight -> cut freshbooks -> bump -> cut mcp -> cut cli -> docs -> a `docs: ship vX.Y.Z` commit |

`--dry-run` performs zero writes (no git/gh mutation, no changelog edit, no dogfood copy) and echoes every command it would have run as `dry-run: <command>`; `--yes` skips the TTY confirmation before the first push; `--version auto` (on `cut`/`all`) derives the bump kind from the module's own `[Unreleased]` section (`**Breaking:**`/`### Changed` -> minor while `0.x`, `### Added` -> minor, only `### Fixed` -> patch) and prints the proposal before accepting it. Every mutating step checks its own postcondition first and prints `SKIP` when it is already done, so re-running `all`/`cut`/`bump` after a partial failure is safe -- verification itself never skips. Output is one `release: OK/SKIP/FAIL <step>` line per step, plus `release: NOTE <text>` lines for things that are not a step outcome (the version proposal, the tag-ruleset call `preflight` reports but never makes, and the `docs/progress.md` ledger row and `GOAL.md` retarget that `all` deliberately leaves to you). `cut`, `bump` and `all` all require `main`, a clean tree, and a strict `X.Y.Z` version, and no tag is ever pushed before CI is green for the exact commit it points at. The script never reads `FRESHBOOKS_*`/`GITHUB_TOKEN` and never touches branch protection or tag rulesets. `scripts/release-selftest.sh` (`mise run release-selftest`, also wired into the gate) is its regression test, run entirely against scratch repos and fake `gh`/`go`. `--timeout <seconds>` caps each CI and Release watch (discovery plus completion; default 1200).

Each module tags and releases independently: `freshbooks/vX.Y.Z`, `mcp/vX.Y.Z`, `cli/vX.Y.Z`. Pushing a matching tag (which `release.sh cut` does) runs `.github/workflows/release.yml`:

1. **guard** -- parses the module and version out of the tag, rejects anything that isn't strict semver, confirms the tag commit is an ancestor of `origin/main`, and fails fast if the module's `## [X.Y.Z]` changelog section (`scripts/changelog-section.sh`) doesn't exist.
2. **ci** -- re-runs the full CI workflow via `workflow_call`; a red gate blocks the release.
3. **release** -- extracts the changelog section again as the release notes body, then publishes.

goreleaser OSS cannot release a prefixed tag: it validates with `git describe --exact-match --match <tag>` against the *current* tag it is told about, and its GitHub-release pipe would create the release on that same tag -- neither works against `mcp/v0.1.0` or `cli/v0.1.0`. So goreleaser only builds; `gh` publishes. For `mcp`/`cli`, the release job runs, from the module directory:

```sh
GORELEASER_CURRENT_TAG=v<version> mise exec -- goreleaser release --skip=publish,validate --clean
```

`GORELEASER_CURRENT_TAG` strips the `<module>/` prefix so `{{.Version}}` and archive names come out as plain semver. `--skip=validate` skips both the exact-match tag check (which would fail against a prefixed tag) and goreleaser's dirty-tree check -- moot here, since the CI checkout is pristine and `guard` already enforced semver plus on-`main`. `--skip=publish` stops goreleaser from creating its own GitHub release. Each `.goreleaser.yaml` sets `project_name` explicitly (`freshbooks-mcp`, `freshbooks`) so the two modules' archives never collide, and builds with `GOWORK=off` so the binary is built against the module's own `go.mod` -- the same lib pseudo-version a `go install .../<module>/...@<tag>` user gets, not the workspace's sibling checkout. SBOMs (SPDX 2.3 JSON via [syft](https://github.com/anchore/syft), mise-pinned) are generated per archive.

Then `gh release create "$TAG_NAME" --verify-tag --notes-file <notes> <module>/dist/*.tar.gz <module>/dist/*.zip <module>/dist/checksums.txt <module>/dist/*.sbom.json` publishes the archives, `checksums.txt`, and SBOMs onto the real prefixed tag. `freshbooks` has no binary to build, so it skips the goreleaser step and goes straight to `gh release create --verify-tag` (the Go module proxy picks up the tag on its own).

## Docs generation

`mise run docs` (`scripts/docs.sh`) regenerates `docs/cli.md` from the CLI's cobra command tree: `go run -tags docsgen ./cli/cmd/freshbooks docs docs/cli.md`. The `docsgen` build tag matters -- without it, `cli/internal/cmd`'s hidden `docs` command (and the `cli/internal/docsgen` package it calls into, the module's only non-test importer of `github.com/spf13/cobra/doc`) is not even compiled in, so a plain `go build ./cli/cmd/freshbooks` never links `cobra/doc`, `go-md2man`, or `blackfriday` into the release `freshbooks` binary. Generation is deterministic (`DisableAutoGenTag`, children sorted by name at every level), so running it twice with an unchanged command tree produces a byte-identical file; `cli/internal/cmd/docs_drift_test.go` (untagged, so it runs in every gate) fails naming the exact command to fix if `docs/cli.md` has drifted from the current tree.
