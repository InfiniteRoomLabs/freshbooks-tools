# Building freshbooks-tools

## Toolchain

Everything runs through [mise](https://mise.jdx.dev/). `mise.toml` pins exact versions for `go`, `golangci-lint`, `goreleaser`, `actionlint`, and `usage` -- never `latest`. Run `mise install` once per clone (or after pulling a `mise.toml` change) to fetch them.

The repo is a Go workspace (`go.work`) with three modules: `freshbooks/`, `mcp/`, `cli/`. Run `go` commands from inside a module directory, or with `-C <module>`.

## The gate

```sh
mise run check                # all three modules
mise run check -- freshbooks  # one module
```

This runs, per module: `fmt-check` (gofmt), `vet` (go vet), `lint` (golangci-lint), `test` (`go test -race -coverprofile -covermode=atomic ./...`, plus a pass with `-tags integration`), `cover` (the 90% coverage floor, via `scripts/coverage-gate.sh`), and `inventory-check` (freshbooks only -- the `// inventory:` parity check). It then cross-compiles `freshbooks-mcp` and the `freshbooks` CLI for `{linux,darwin,windows} x {amd64,arm64}` into `dist/` (gitignored), and finishes with a `DIRTY TREE:` banner plus `git status --porcelain` if the working tree isn't clean -- a passing gate on a dirty tree is not a passing gate.

Each step is also its own task (`mise run fmt-check`, `mise run vet`, `mise run lint`, `mise run test`, `mise run cover`, `mise run inventory-check`, `mise run build`), all backed by `scripts/check.sh`. `scripts/build.sh` does the cross-compile alone; `scripts/coverage-gate.sh <threshold> <coverprofile>` is the standalone coverage check.

`main.go` entry points (`cmd/*/main.go`) are excluded from the gated coverage number by filename -- they're thin flag-parsing + `os.Exit` wiring that a test process cannot exercise (`os.Exit` would kill the test binary), so the substantive logic is required to live in a tested `run()`-style function instead. `internal/cmd/` and everything else still counts.

## Lint

`.golangci.yml` (v2 config schema) enables `errcheck`, `govet`, `staticcheck`, `revive` (with the exported-identifier rule), `gosec`, and `misspell`, with no default exclusions -- every finding is a real finding. A justified `gosec` suppression uses `//nolint:gosec // <reason>`, never a blanket disable.

## Branch protection

`scripts/branch-protection.sh` applies required-status-checks protection to `main` via `gh api` (job names `lib`, `mcp`, `cli` from `.github/workflows/ci.yml` are the required checks), plus `required_linear_history` and no force-pushes. GitHub does not read this from any file in the repo -- it has to be applied once, out of band, by someone with admin access.

## Release flow

Each module tags and releases independently: `freshbooks/vX.Y.Z`, `mcp/vX.Y.Z`, `cli/vX.Y.Z`. Pushing a matching tag runs `.github/workflows/release.yml`:

1. **guard** -- parses the module and version out of the tag, rejects anything that isn't strict semver, confirms the tag commit is an ancestor of `origin/main`, and extracts the module's `## [X.Y.Z]` changelog section (`scripts/changelog-section.sh`) as the release notes body.
2. **ci** -- re-runs the full CI workflow via `workflow_call`; a red gate blocks the release.
3. **release** -- `mcp`/`cli` build and publish via [goreleaser](https://goreleaser.com/) (`<module>/.goreleaser.yaml`); `freshbooks` gets a plain `gh release create` (the Go module proxy picks up the tag on its own).

goreleaser OSS has no monorepo tag-prefix support (that's a paid Pro feature), so the release job sets `GORELEASER_CURRENT_TAG` to the parsed plain-semver version before invoking it, stripping the `<module>/` prefix so `{{.Version}}` and archive names come out right. This hasn't been exercised against a real tag push yet -- Phase 5 (release hardening) does that dry run.

## Docs generation

`mise run docs` is currently a stub -- it just says cobra/doc generation for `docs/cli.md` lands in Phase 4, once the CLI has resource commands worth documenting.
