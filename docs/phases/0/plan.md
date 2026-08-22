# Phase 0 (scaffold) -- implementer work order

Dispatch: `Agent(subagent_type: "general-purpose", model: "sonnet", name: "phase-0-impl")`. Generated 2026-08-22 from `docs/phases/_templates/implementer.md`.

---

You are implementing **Phase 0 (scaffold)** of `freshbooks-tools`, a public MIT Go monorepo. Work ONLY inside `<repo root>` on branch `phase-0/scaffold` (already checked out, clean). Do not touch other branches.

## Read first (pointers, not pasted)

1. The oracle: `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` sections 2 (locked), 3 (read the `STATE AS OF` callout about 213 requests), 4, 5.2, 8, 9.5, 10. Do not redesign.
2. Conventions: `CLAUDE.md` (toolchain, commits, green rule, parity contract, public-repo hygiene), `GOAL.md` stage 2 (the deliverable list).
3. Global rules that also apply: every new argument-taking shell script uses the `usage` shebang (`#!/usr/bin/env -S usage bash` + `# USAGE` directives); pin `usage` in `mise.toml` so CI has it. No hard-wrapped prose in markdown, ASCII only.
4. The collection: `docs/freshbooks.postman_collection.json` (you move it). Postman v2.1 schema; requests may have `url` as a string or an object with `raw`; folders nest one level deep (22 subfolders); one folder name has a trailing space (`My Team `); one URL embeds a newline; `{{accountId}}` vs `{{accountid}}` vs hard-coded example IDs; ~6 `my.freshbooks.com/service/api/...` requests.

## Pinned versions (verified resolvable 2026-08-22; never `latest`)

- `mise.toml` tools: `go = "1.26.5"`, `golangci-lint = "2.13.1"`, `goreleaser = "2.17.1"`, `actionlint = "1.7.12"`, `usage = "6.0.0"`.
- Go modules: `github.com/stretchr/testify v1.12.1` (tests only), `github.com/spf13/cobra v1.10.2` (cli only). The mcp module does NOT pull go-sdk in Phase 0 (a placeholder `main` that prints the version has no use for it; Phase 3 adds it). Keep every `go.mod` tidy.
- `go 1.26` directive in every `go.mod` and `go.work`.

## Deliverables

### A. Workspace and modules

- `go.work` listing `./freshbooks`, `./mcp`, `./cli`.
- `freshbooks/`: module `github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks`. `doc.go` with the package overview (what the lib is, the two API families, the ID types, pointer to docs) and `const Version = "0.0.0-dev"`. `version_test.go` asserting `Version` is non-empty (so coverage is measurable). `CHANGELOG.md` (Keep a Changelog, `## [Unreleased]` with an `### Added` line for the scaffold).
- `mcp/`: module `.../mcp`. `cmd/freshbooks-mcp/main.go` prints `freshbooks-mcp <version>` (version var settable via `-ldflags -X`), exits 0. `internal/config/doc.go`, `internal/server/doc.go`, `internal/tools/doc.go` placeholder packages with a package comment each. A test for the version-print path (move the printing into a `run(w io.Writer) error`-style function so it is testable). `CHANGELOG.md`.
- `cli/`: module `.../cli`. `cmd/freshbooks/main.go` builds a cobra root `freshbooks` with `version` and `completion` subcommands (cobra provides completion). Root command lives in `internal/cmd` (`NewRootCmd() *cobra.Command`), with tests executing `version` and `completion bash` via `cmd.SetArgs` + `cmd.SetOut`. `internal/{config,output,auth}/doc.go` placeholders. `CHANGELOG.md`.
- Every module >= 90% coverage of the statements it has. Every exported identifier documented.

### B. Inventory tool `freshbooks/internal/inventory` (the only real code)

Layout: `freshbooks/internal/inventory/main.go` (CLI), `inventory.go` (parse + normalize, exported `Load`, `Normalize`, `Entry`, `Check`), `check.go` (source scan + ignore list), `*_test.go`, `testdata/freshbooks.postman_collection.json` (moved from `docs/` with `git mv`), `testdata/inventory.json` (generated, committed, must be byte-stable across runs: sorted by key, 2-space indent, trailing newline), `testdata/ignore.list`, plus small hand-written fixtures under `testdata/fixtures/` for the unit tests (synthetic names and IDs only).

Commands:

- `go run ./internal/inventory -in <postman.json> -out <inventory.json>` -- emits the inventory.
- `go run ./internal/inventory -check <pkg...> [-inventory <inventory.json>] [-ignore <ignore.list>]` -- parity check, exit 1 on any failure, prints a report (`implemented N, ignored N, todo N, uncovered N, double-covered N, stale N, unknown N`). Defaults: inventory and ignore paths relative to the module (`internal/inventory/testdata/...`).

Entry shape (JSON keys in this order): `key`, `folder` (top-level folder), `path` (array of subfolder names, may be empty), `name`, `method`, `pathTemplate` (path only, no host, leading `/`), `host` (original host, after rewrite), `query` (array of `{name, value, description}`), `body` (raw string or null), `responses` (array of `{name, status, body}`), `family` (one of `accounting`, `business`, `auth`, `events`, `uploads`, `payments`, `ledger`, `internal`, `unknown`), `duplicates` (int, number of identical source entries collapsed; 1 normally).

Key = `<Folder>/<Subfolder>/.../<Request name>`, every segment `strings.TrimSpace`d. Exact duplicates (same key, method, and normalized path) collapse into one entry with `duplicates` incremented; same key with a different method/path is an error (the tool must fail loudly, not silently pick one).

Normalization rules (each one gets a table-driven test):

1. Strip all whitespace including embedded newlines from the raw URL; trim folder/request names.
2. Postman variables `{{name}}` -> `{name}` lower-camel-cased: `{{accountid}}`/`{{accountId}}`/`{{AccountID}}` -> `{accountId}`, `{{businessId}}` -> `{businessId}`, `{{invoiceid}}` -> `{invoiceId}`; generic rule: split on non-alphanumerics, lowercase first word, TitleCase the rest, and a small fixed map for known acronyms (`id` -> `Id`, `uuid` -> `Uuid`).
3. Hard-coded IDs: a segment right after `/account/` that is not a variable -> `{accountId}`; a numeric segment right after `/business/` -> `{businessId}`; any other purely numeric path segment -> `{id}`.
4. `https://my.freshbooks.com/service/api/<rest>` -> host `api.freshbooks.com`, path `/<rest>`, family `internal` (these are candidates for the ignore list, not silently dropped).
5. Family by path prefix: `/accounting/account/` -> `accounting`; `/accounting/businesses/` -> `ledger`; `/projects/business/`, `/timetracking/business/`, `/comments/business/`, `/auth/api/v1/businesses/` -> `business`; `/auth/` (other) -> `auth`; `/events/` -> `events`; `/uploads/` -> `uploads`; `/payments/` -> `payments`; else `unknown`.
6. Query params come from the Postman `url.query` array (keep raw `{{var}}` text in values after the same variable rewrite). Bodies and example responses are copied verbatim (strings), not parsed.

Check rules:

- Scan every `.go` file (excluding `_test.go` and `testdata/`) in the given packages for comments matching `^\s*//\s*inventory:\s*(.+?)\s*$`. Use `go/parser` with `ParseComments` on each package directory (`go list -f '{{.Dir}}' <pkg...>` via `os/exec` is acceptable; `golang.org/x/tools/go/packages` is NOT -- stdlib only).
- Ignore list file format, one directive per line, `#` comments and blank lines allowed:
  - `//go:inventory-ignore <key> -- <reason>` : deliberately not implemented (`my.freshbooks.com` internal endpoints, exact hard-coded-ID duplicates of another request, etc.).
  - `//go:inventory-todo <key> -- <phase>` : planned for a later phase; counts as covered for the check but is reported in the `todo` count.
  - Keys contain spaces, hence the ` -- ` separator. A key listed twice, or listed AND implemented in code (stale), or not present in the inventory (unknown) fails the check.
- Failure conditions: any key uncovered (not implemented, not ignored, not todo), double-covered (two `// inventory:` comments for one key), stale, unknown, or an `// inventory:` comment whose key is not in the inventory.
- Phase 0 state of `testdata/ignore.list`: every `internal`-family entry as `ignore` with reason `internal my.freshbooks.com endpoint`; every remaining key as `todo` with phase `phase-2` (Authorization folder entries -> `phase-1`). Generate this file with the tool (`-emit-todo <phase-map>` is NOT required; a one-off script or a test helper is fine, but the committed file must be sorted and the check must pass against it).

Tests: unit tests with small synthetic fixtures for parsing (string URL vs object URL, nested folders, trailing space, embedded newline, variables of all three spellings, hard-coded IDs, internal host rewrite, exact duplicates collapse, conflicting duplicates error, every family); a golden test that loads the real collection from `testdata/`, normalizes, and asserts 213 leaf requests / 211 unique keys / the per-folder counts in the spec callout and that re-emitting equals the committed `inventory.json` byte-for-byte; check tests using a temp package dir with `// inventory:` comments covering every failure class and the pass path. Coverage >= 90% for the `inventory` package (it is the bulk of the `freshbooks` module in this phase, so it dominates the module number).

### C. Build tooling

- `mise.toml`: `[tools]` pins above; `[tasks]` `fmt-check`, `vet`, `lint`, `test` (`go test -race -coverprofile=coverage.out -covermode=atomic ./...` per module, integration tag too: `-tags integration`), `cover` (`scripts/coverage-gate.sh 90 <module>/coverage.out` per module), `inventory-check`, `build` (`scripts/build.sh`: cross matrix `{linux,darwin,windows} x {amd64,arm64}` for `mcp/cmd/freshbooks-mcp` and `cli/cmd/freshbooks` into `dist/` which is gitignored), `check` (runs all of the above for every module, or for the modules named as args: `mise run check -- freshbooks`; afterwards prints a banner `DIRTY TREE:` plus `git status --porcelain` when non-empty and exits 1), `docs` (stub now: prints that cobra/doc generation lands in Phase 4 and exits 0). Implement the per-module loop in `scripts/check.sh` (usage shebang, args = module names, default all three); each task = `mise exec -- ` is unnecessary inside mise tasks (tools are already on PATH).
- `.golangci.yml` (v2 config format for golangci-lint 2.x): enable `errcheck`, `govet`, `staticcheck`, `revive` (with the `exported` rule on), `gosec`, `misspell`; `issues.exclude-use-default: false`; no warnings-as-non-errors.
- `scripts/coverage-gate.sh <threshold> <coverprofile>`: reads `go tool cover -func`, compares the `total:` line, exits 1 below threshold, prints the number either way.
- `scripts/changelog-section.sh <module-dir> <version>`: prints the `## [X.Y.Z]` section body of `<module-dir>/CHANGELOG.md`, exit 1 if absent.
- `scripts/branch-protection.sh`: `gh api -X PUT repos/InfiniteRoomLabs/freshbooks-tools/branches/main/protection` with required status checks `lib`, `mcp`, `cli` (strict), PRs required (0 approving reviews is fine for a solo repo), `required_linear_history: true`, `enforce_admins: false`, `allow_force_pushes: false`. Takes an optional `--repo` flag.
- `scripts/redaction-check.sh`: if `~/projects/infinite-room-labs/agent-ops/scripts/resolve-redaction-terms.py` exists, runs it to get the term list (one per line) and greps `git diff --cached` for each (case-insensitive, fixed strings); exit 1 on a hit, printing the file and the term's index (never the term itself); if the script is absent, prints `redaction-check: term list not available (optional for outside contributors)` and exits 0.
- `scripts/build.sh` as described.
- `.gitignore` additions: `dist/`, `coverage.out`, `.worktrees/` (check what exists first).

### D. CI / release / dependabot

- `.github/workflows/ci.yml`: triggers `pull_request`, `push` to `main`, `workflow_call`. Jobs `lib`, `mcp` (`needs: lib`), `cli` (`needs: lib`), each on `ubuntu-latest`: `actions/checkout@v4` (pin by tag is fine), `jdx/mise-action@v2` (`install: true`, `cache: true`), then `mise run check -- <module>` (`freshbooks` for `lib`). The job names are the required checks; keep them exactly `lib`, `mcp`, `cli`.
- `.github/workflows/release.yml`: `on: push: tags: ['freshbooks/v*', 'mcp/v*', 'cli/v*']`. Job `guard`: parse module + version from the tag; fail unless version matches `^v[0-9]+\.[0-9]+\.[0-9]+$`; `git fetch origin main` and `git merge-base --is-ancestor $GITHUB_SHA origin/main`; `scripts/changelog-section.sh <module> <version>` must succeed (write the body to a file artifact). Job `ci`: `uses: ./.github/workflows/ci.yml` with `needs: guard`. Job `release` (`needs: [guard, ci]`): for `mcp`/`cli` run goreleaser (`goreleaser/goreleaser-action@v6`, `workdir: <module>`, `args: release --clean --release-notes=<body file>`); for `freshbooks` create the GitHub release with `gh release create <tag> --notes-file <body file>`. `permissions: contents: write`.
- `mcp/.goreleaser.yaml` and `cli/.goreleaser.yaml` (goreleaser v2 schema): `builds` with the 6-target matrix, `main: ./cmd/<binary>`, `binary: <binary>`, `ldflags: -s -w -X main.version={{.Version}}`, `monorepo: {tag_prefix: "<module>/", dir: "<module>"}`, archives (tar.gz, zip for windows), `checksum`, `sbom` (`artifacts: archive`), `changelog: {disable: true}` (the body comes from the changelog section).
- `.github/dependabot.yml`: `gomod` for `/freshbooks`, `/mcp`, `/cli`, and `github-actions` for `/`, all weekly.
- Run `actionlint` (it is pinned in mise) on both workflows; must be clean.

### E. Docs

- `README.md`: what/why (one paragraph, no marketing), module table (`freshbooks` lib / `freshbooks-mcp` / `freshbooks` CLI with module paths, status "pre-release"), install placeholders (`go install` lines, "binaries on the Releases page"), quick links to every doc, pkg.go.dev badge/link for the lib, and a **Contributing** section: Go >= 1.26 via mise (`mise install`), `mise run check`, conventional commits, and the agent setup note: contributors using Claude Code install the agent-ops marketplace (`/plugin marketplace add InfiniteRoomLabs/agent-ops`) for the changelog guard; `scripts/redaction-check.sh` is optional for outside contributors. Not affiliated with FreshBooks; MIT.
- Stubs with real headings (3-6 headings each, one sentence under each saying what will go there and which phase fills it): `docs/getting-started.md`, `docs/building.md` (this one can be mostly real now: mise, tasks, the gate, branch protection script, release flow), `docs/authentication.md`, `docs/library.md`, `docs/mcp.md`, `docs/cli.md`.
- `docs/agentic-transformation.md` written for real: how the collection was pulled (`https://documenter.gw.postman.com/api/collections/<id>/<slug>?segregateAuth=true&versionTag=latest` -- use the real id `3322108` and slug `S1ERwwza`, that is a public FreshBooks URL), why no codegen, the inventory tool (keys, normalization, ignore/todo list, the `// inventory:` parity contract), the work orders and the four-lane gate (point to spec section 9 and `docs/phases/_templates/`), and the GOAL.md treadmill.
- Root `CHANGELOG.md` `[Unreleased]`: add the scaffold line(s). Do NOT touch `GOAL.md`.

## Acceptance

- `mise run check` green for all three modules on a clean tree (`git status --porcelain` empty).
- `cd freshbooks && go run ./internal/inventory -check ./...` passes (every key implemented/ignored/todo).
- `actionlint` clean on both workflows.
- `scripts/redaction-check.sh` passes on every commit you make.

## Gotchas

- Fixture IDs and names are synthetic. The real collection contains example account IDs and names; the inventory tool normalizes IDs away in `pathTemplate`, but example bodies/responses are copied verbatim into `inventory.json` -- that is FreshBooks' own public documentation data, acceptable. Do not add any of it to hand-written fixtures or tests.
- `revive`'s exported rule will flag every undocumented exported identifier, including in `main` packages and test helpers that are exported. Document them or keep them unexported.
- `gosec` flags `os/exec` with variable args (G204) and file permissions; use `//nolint:gosec // reason` only with a real reason, or restructure.
- The `usage` shebang needs `usage` on PATH at runtime: `mise` puts it there for `mise run` tasks and in CI via mise-action. Run scripts through `mise exec -- scripts/x.sh` or from mise tasks, never assume a bare shell has it. Validate with `usage lint <script>`.
- `go vet` and `gofmt -l` across the workspace: run them per module from inside the module dir (`go -C <module> ...`).
- Docs are ASCII-only, no hard wraps, `--` and `->` instead of dashes/arrows.
- If the spec is wrong about something, implement reality, add a `> **STATE AS OF 2026-08-22**` callout in the affected spec section in the same commit, and list it in your report.

## Reporting (both channels)

When done (gate green, committed, `git status --porcelain` empty): write the report to `docs/phases/0/reports/implementer.md` (commit it), send the same report with `SendMessage` to `team-lead` (report in `message`, not `summary`), AND return it as your final text. Report: files created/changed, test counts per package, coverage per module, the exact `mise run check` tail, `git log --oneline main..phase-0/scaffold`, `git status --porcelain` output, inventory counts (leaf/unique/ignored/todo), and every spec discrepancy or ambiguity you hit and how you resolved it. If you are genuinely blocked, report the blocker the same way instead of guessing.
