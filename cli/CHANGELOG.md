# Changelog

All notable changes to the `freshbooks` CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-09-02

### Added

- The full 168-command registry (`internal/cmd`, data-driven, one `Command` per `freshbooks` client-library method) built into a cobra tree grouped by resource, plus the non-registry commands: `auth login|status|logout|token`, `config view|contexts|use-context|set-context`, `api <METHOD> <path>` (the escape hatch for endpoints not yet modeled as their own command), and `version` (falls back to the Go module version via `debug.ReadBuildInfo` when built without `-ldflags -X main.version=...`, so a `go install .../freshbooks@<tag>` build reports the tag instead of a placeholder).
- `auth login`: loopback PKCE login over an ephemeral, in-process, never-written-to-disk self-signed certificate on `https://localhost:8765/callback`, or `--no-browser` to print the URL and read the redirect (or a bare code) from stdin. `--login-timeout` bounds how long it waits for the browser callback -- named distinctly from the global `--timeout` (per-request) so the two can never shadow each other.
- `--account`/`--business`/`--business-uuid`/`--context`/`--config` scope and context resolution (flag > env > the current context in `config.yaml` > default); credentials live one `FileStore`-backed JSON file per context, never in `config.yaml`. `config.File`/`config.Context` carry `json` tags matching their `yaml` tags, so `freshbooks config view -o json` prints `current-context`/`account`/`business`/`business_uuid` rather than Go's default field names.
- `-o/--output json|yaml|table|name` (`internal/output`), with `table`/`json` defaulting on whether stdout is a terminal; `--no-headers`; `-q/--quiet`. A Binary command's local `-o <file>` (`invoices pdf`, `reports download-invoice-details-csv`) is a distinct flag that shadows the global one only on those two commands, ignores `FRESHBOOKS_OUTPUT`, refuses to overwrite an existing file without `--force`, and refuses `-o -` when stdout is a terminal.
- List commands: `--page`/`--per-page`, `--search key=value` (repeatable), `--all` (walks every page, rejecting `--page`/`--per-page`), and `--sort field[:asc|desc]` on the List commands whose lib method takes `extra ...RequestOption`; `--include` (repeatable) on every List and single-resource `get`/`create`/`update` command whose lib method supports it.
- `-f/--file <path>|-` supplies a write command's JSON request body (a file path or stdin); the 13 report commands take it as an optional filter-options body instead of a required one.
- `--dry-run`: prints the request's method, URL, and body (never a header) and sends nothing; rejected outright (exit 2) on `auth` and `config` commands, which have no request to preview.
- `--yes` gates every destructive command when stdin is a terminal; the command reference marks each one with a trailing " (destructive: requires --yes on a TTY)" on its `Short` help line.
- Exit codes 0 (success) / 1 (API or other runtime error) / 2 (usage error -- a bad flag, a malformed `--file` body, `--all` with `--page`/`--per-page`, a destructive command without `--yes` on a TTY, and about a dozen more) / 3 (auth error: no stored credentials, or a 401) / 4 (a 404), with a JSON error object on stderr under `-o json`.
- The hidden `docs` command (`scripts/docs.sh`/`mise run docs` regenerates `docs/cli.md` from the cobra tree) lives behind a `docsgen` build tag in its own `internal/docsgen` package, the module's only non-test importer of `github.com/spf13/cobra/doc` -- a plain `go build ./cmd/freshbooks` never links `cobra/doc`, `go-md2man`, or `blackfriday` into the release binary.
- Release: goreleaser builds per-platform archives (`{linux,darwin,windows} x {amd64,arm64}`) with `GOWORK=off` (so the binary embeds the same lib pseudo-version a `go install` user gets), a shared `checksums.txt`, and an SPDX 2.3 JSON SBOM per archive; `gh release create --verify-tag` publishes them onto the module's prefixed tag, since goreleaser OSS cannot release a `cli/vX.Y.Z`-shaped tag itself.
