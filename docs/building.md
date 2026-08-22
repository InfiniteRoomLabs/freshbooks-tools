# Building freshbooks-tools

## Toolchain

Everything runs through [mise](https://mise.jdx.dev/). `mise.toml` pins exact versions for `go`, `golangci-lint`, `goreleaser`, `actionlint`, and `usage` -- never `latest`. Run `mise install` once per clone (or after pulling a `mise.toml` change) to fetch them.

The repo is a Go workspace (`go.work`) with three modules: `freshbooks/`, `mcp/`, `cli/`. Run `go` commands from inside a module directory, or with `-C <module>`.

## The gate

```sh
mise run check                # all three modules
mise run check -- freshbooks  # one module
```

This runs, per module: `fmt-check` (gofmt), `vet` (go vet), `lint` (golangci-lint), `test` (`go test -race -coverprofile -covermode=atomic ./...`, plus a pass with `-tags integration`), `cover` (the 90% coverage floor, via `scripts/coverage-gate.sh`), and `inventory-check` (freshbooks only -- the `// inventory:` parity check). It then runs `actionlint` against `.github/workflows/*.yml` once (not per module), cross-compiles `freshbooks-mcp` and the `freshbooks` CLI for the modules actually in scope (both, unless the invocation filtered to a module that isn't `mcp`/`cli`) across `{linux,darwin,windows} x {amd64,arm64}` into `dist/` (gitignored), and finishes with a `DIRTY TREE:` banner plus `git status --porcelain` if the working tree isn't clean -- a passing gate on a dirty tree is not a passing gate.

Each step is also its own task (`mise run fmt-check`, `mise run vet`, `mise run lint`, `mise run test`, `mise run cover`, `mise run inventory-check`, `mise run vuln`, `mise run build`, `mise run actionlint`), all backed by `scripts/check.sh`. `scripts/build.sh [modules...]` does the cross-compile alone (default both `mcp` and `cli`; `mise run check -- freshbooks` therefore builds nothing); `scripts/coverage-gate.sh <threshold> <coverprofile>` is the standalone coverage check.

`cmd/<binary>/main.go` entry points are excluded from the gated coverage number -- scoped to that path shape specifically, not any file named `main.go` and not by directory. They're thin flag-parsing + `os.Exit` wiring that a test process cannot exercise (`os.Exit` would kill the test binary), so the substantive logic is required to live in a tested `run()`-style function instead. `internal/cmd/` (cobra wiring, fully tested) and `freshbooks/internal/inventory/main.go` (the inventory CLI's flag parsing and report rendering, also fully tested, and not under a `cmd/` directory) both stay counted. A coverage profile with no measurable statements left after that filter is a hard failure, not a vacuous pass -- a module that's supposed to have real code and doesn't measure any is exactly the situation the gate exists to catch.

## Lint

`.golangci.yml` (v2 config schema) enables `errcheck`, `govet`, `staticcheck`, `revive` (with the exported-identifier rule), `gosec`, and `misspell`, with no default exclusions -- every finding is a real finding. A justified `gosec` suppression uses `//nolint:gosec // <reason>`, never a blanket disable.

## Vulnerability scanning

`mise run vuln` runs `govulncheck` (pinned at `golang.org/x/vuln/cmd/govulncheck`) per module, as a step in `mise run check` right after `cover`. `mise.toml`'s exact `go` pin has no dependabot lane watching it (dependabot only understands `go.mod`/`go.sum` and GitHub Actions), so this is the check that would have caught the toolchain shipping with known stdlib vulnerabilities.

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
