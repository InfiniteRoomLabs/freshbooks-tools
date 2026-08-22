# Work order: security lane

Dispatch: `Agent(subagent_type: "general-purpose", model: "<one tier above the implementer>", name: "phase-<n>-security")`. Runs in parallel with the code-review and simplification lanes. Read-only except for running `govulncheck` if it is installed (it does not write to the tree).

---

You are the **security lane** of a four-lane review gate for branch `<branch>` in `<absolute path>` (`git diff main...<branch>`). **READ-ONLY:** do not modify files, do not commit, and do NOT run `mise run check`, tests, or builds. You may run `git`, `grep`, `go list -m all`, `govulncheck ./...` (if present), and read files. This is a public repo that handles OAuth tokens and accounting data for real businesses.

## Context

- Spec: `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` sections 5.1 (auth, transport), 6 (MCP auth per transport), 7 (CLI credential handling), 9.2 (security checklist), 10 (public-repo hygiene). Conventions: `CLAUDE.md`.

## Check, with evidence

1. **Secrets never leak:** access/refresh tokens, client secrets, and `Authorization` headers must not appear in logs at any level, in error strings, panics, `--dry-run` output, test fixtures, golden files, or doc examples. Grep for `slog`, `fmt.Errorf`, `%v` on request/response structs, and `String()` methods.
2. **Credential storage:** files created `0600`, written atomically (temp + rename), directory `0700`; no world-readable defaults; path honours `XDG_CONFIG_HOME`; no credentials in `config.yaml`.
3. **OAuth flow:** PKCE verifier is random (crypto/rand) and never logged; `state` is random and validated; loopback listener binds `127.0.0.1` only, single-use, times out; redirect URI matches exactly; refresh rotation persists the new token before the old one is discarded; single-flight refresh (no concurrent refreshes racing a one-time-use token).
4. **Transport:** TLS never disabled; `http.Client` timeouts set; redirects do not forward `Authorization` to other hosts; response bodies bounded or streamed; `Retry-After` parsed defensively.
5. **Trust boundaries:** CLI args/flags, stdin JSON, MCP tool inputs, and API responses are validated or decoded into typed structs; no path traversal from user-supplied file names (`-f`, `-o`); no shell-outs with user input; no `unsafe`.
6. **Stateless MCP:** HTTP mode keeps no per-client state; bearer token from the request is never cached across requests; errors returned to the client do not echo the token; `/healthz` reveals nothing sensitive.
7. **Supply chain:** `go.sum` present and unchanged except for intended additions; new dependencies listed in the report with justification; `govulncheck` clean; GitHub workflows pin actions by SHA or trusted major tag and use least-privilege `permissions:`; release workflow cannot be triggered from a tag that is not on `main`.
8. **Public-repo hygiene:** no real account/business IDs, internal hostnames/IPs, vault item names, or personal names anywhere in the diff (fixtures included).

## Deliver

Verdict **PASS** or **BLOCK**, findings numbered and tagged **BLOCKING** (must fix before merge) / **ADVISORY**, each with `file:line`, the evidence, and the concrete fix. Write the report to `docs/phases/<n>/reports/security.md` (do not commit), send it with `SendMessage` to `team-lead` (full report in `message`, not `summary`), AND return it as your final text.
