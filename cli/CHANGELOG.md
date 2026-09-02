# Changelog

All notable changes to the `freshbooks` CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- The full 168-command registry (`internal/cmd`, data-driven, one `Command`
  per `freshbooks` client-library method) built into a cobra tree grouped
  by resource, plus the non-registry commands: `auth login|status|logout|
  token` (loopback PKCE login over an ephemeral self-signed certificate,
  or `--no-browser`), `config view|contexts|use-context|set-context`,
  `api <METHOD> <path>` (the escape hatch for endpoints not yet modeled as
  their own command), `version`, and the hidden `docs` command
  `scripts/docs.sh`/`mise run docs` uses to regenerate `docs/cli.md`.
- `-o/--output json|yaml|table|name` (`internal/output`), with `table`/
  `json` defaulting on whether stdout is a terminal; `--no-headers`;
  `-q/--quiet`.
- `--account`/`--business`/`--business-uuid`/`--context`/`--config`
  scope and context resolution (flag > env > the current context in
  `config.yaml` > default); credentials live one `FileStore`-backed JSON
  file per context, never in `config.yaml`.
- `--dry-run`: prints the request's method, URL, and body (never a
  header) and sends nothing.
- `--yes` gates every destructive command when stdin is a terminal.
- Exit codes 0/1/2/3/4 per the design (usage/auth/not-found distinguished
  from a generic API or runtime error), with a JSON error object on
  stderr under `-o json`.
- Repository scaffold: cobra root command with `version` and `completion`
  subcommands, and placeholder `internal/{config,output,auth}` packages.

### Fixed

- `config.File`/`config.Context` now carry `json` tags matching their
  `yaml` tags, so `freshbooks config view -o json` prints
  `current-context`/`account`/`business`/`business_uuid` instead of Go's
  default `CurrentContext`/`Account`/... field names.
