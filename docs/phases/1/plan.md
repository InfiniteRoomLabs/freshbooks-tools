# Work order: implementer -- Phase 1 (lib core)

Dispatch: `Agent(subagent_type: "general-purpose", model: "opus", name: "phase-1-impl")`.

---

You are implementing **Phase 1 (lib core)** of `freshbooks-tools`, a public MIT Go monorepo. Work ONLY inside `<repo root>` on branch `phase-1/lib-core` (already created, clean). Do not touch other branches or worktrees.

## Read first (pointers, not pasted)

1. The oracle: `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` sections 3 (especially the `STATE AS OF 2026-08-23` live OAuth callout -- those facts are CONFIRMED and win over prose), 5.1, 8.1. Section 2 is locked; do not redesign.
2. Conventions: `CLAUDE.md` (toolchain, commits, green rule, parity contract, public-repo hygiene), `GOAL.md` stage 2 (the deliverable list and acceptance criteria -- it is the authoritative checklist for this phase).
3. **FreshBooks docs before coding:** https://www.freshbooks.com/api/start and https://www.freshbooks.com/api/authentication.
4. **Live captures (sanitized, synthetic IDs): `freshbooks/testdata/seed/*.json`** -- real envelope, meta, users/me, and error shapes captured 2026-08-23. Seed your httptest fixtures from these; keep IDs synthetic. `users_me.json` shows the auth-family envelope (`{"response": {...}}`, no `result` layer).
5. Inventory entries for this phase: `freshbooks/internal/inventory/testdata/inventory.json`, folder `Authorization` (4 entries). Each maps to a `// inventory: Authorization/<Request name>` comment on exactly one method; flip their `phase-1 todo` lines in `freshbooks/internal/inventory/testdata/ignore.list` by removing them (implemented entries must not be listed). `mise run inventory-check` is the parity gate.
6. Exemplar for style: `freshbooks/internal/inventory/` (Phase 0's shipped code + tests).

## Deliverables (all in `freshbooks/`, stdlib only; testify in tests)

GOAL.md stage 2 is the authoritative list. Summary:

- `client.go`: `Client`, `NewClient(opts...)`, every service struct type from spec 5.1 pre-declared as an exported field on `Client` (empty structs holding `*Client`) so Phase 2 batches only add files.
- `options.go`: `WithTokenSource`, `WithHTTPClient`, `WithBaseURL`, `WithUserAgent`, `WithLogger`, `WithRetry`, `WithClock`.
- `transport.go`: single `do()` path -- headers, JSON body, accounting envelope unwrap, family-specific error decoding, retry on 429/502/503/504 with jittered exponential backoff honoring `Retry-After`, context cancellation, bounded response bodies, no `Authorization` on cross-host redirects.
- `errors.go`: `*Error`, sentinels, `RetryAfter()`. `page.go`: `Page[T]`, `All` -> `iter.Seq2[T, error]`. `types.go`: `AccountID`, `BusinessID`, `BusinessUUID`, `Money` + `Rat()`, `Date`, `DateTime` (all three wire formats), `VisState`; request options (`Include`, `Search`, `Sort`, `PerPage`, `Page`) with per-family query encoding.
- `identity.go`: `Identity.Me` -> `[]Membership`; carries the 4 `// inventory: Authorization/...` comments.
- `(*Client).Do` escape hatch.
- `auth/`: `Config`, `Endpoints` (both sets as named vars; default = the RFC 8414 set per the spec callout), `AuthCodeURL` with PKCE (`crypto/rand` verifier, S256), `Exchange`, `Refresh`, `Revoke`, `Token`, `TokenSource`, `StaticTokenSource`, `NewTokenSource(cfg, store)` with expiry skew, single-flight refresh, rotation write-back through `TokenStore` **before** returning; `FileStore` (0600, 0700 dir, temp+rename, `XDG_CONFIG_HOME`), `MemoryStore`. Token expiry computes from `created_at + expires_in` (see spec callout).
- Tests per GOAL.md stage 2: table-driven with httptest fixtures under `freshbooks/testdata/<area>/` seeded from `testdata/seed/`; envelope unwrap, both error shapes, 401/404/422/429 (+`Retry-After`), malformed JSON, cancelled context, retry exhaustion, `All` over multiple pages with mid-stream 429, every `Date`/`DateTime` format, PKCE URL, exchange, refresh rotation persisted before return, concurrent refresh single-flight (`-race`), `FileStore` permissions + atomicity. Integration-tagged (`//go:build integration`) seam test: expiry -> refresh -> store write-back -> retried request. `live`-tagged read-only smoke behind `FRESHBOOKS_LIVE=1`. Coverage >= 90%; `WithClock` everywhere time matters.
- `doc.go` overview + runnable `Example`; `freshbooks/CHANGELOG.md` `[Unreleased]`; `docs/authentication.md` and `docs/library.md` written for real; spec `STATE AS OF` callouts for anything the API does differently than the spec says.
- Acceptance: `mise run check` green on a clean tree; `mise run inventory-check` shows `implemented 4` (Authorization) with no uncovered/stale entries; no token or secret in any log, error string, fixture, or doc.

## Gotchas (these cost prior runs time)

- Two ID families (`AccountID` string vs `BusinessID` int64); accounting responses are enveloped, business-scoped ones are flat, auth family is `{"response": {...}}` with no `result`; refresh tokens are one-time-use -- persist through `TokenStore` before returning. See `CLAUDE.md` Gotchas.
- Accounting error objects carry a `value` field (spec callout 2026-08-23); business 404 is a bare `{"error": "..."}` string; 401 is `{"error": "unauthenticated", "error_description": ...}`.
- Fixture IDs and names are synthetic. Never paste real account IDs, tokens, or names into fixtures or tests. `scripts/redaction-check.sh` runs on staged files; keep it green.
- Docs are ASCII-only, no hard wraps (`--`, `->`).
- Run everything through `mise run ...` from the repo root; `go` commands from inside `freshbooks/` or with `-C freshbooks`.
- The changelog/version guard hooks: stage and commit in separate Bash calls, never `-a`/`-am`, never `--no-verify`.
- If the spec is wrong about something the API does, implement what the API does, add a `> **STATE AS OF 2026-08-23**` callout in the affected spec section in the same commit, and list the discrepancy in your report.

## Reporting (both channels)

When done (gate green, committed, `git status --porcelain` empty): write the report to `docs/phases/1/reports/implementer.md` (commit it), send the same report with `SendMessage` to `team-lead` (report in `message`, not `summary`), AND return it as your final text. Report: files created/changed, test counts per package, coverage per module, the exact `mise run check` tail, `git log --oneline main..phase-1/lib-core`, `git status --porcelain` output, inventory entries covered, and every spec discrepancy or ambiguity you hit and how you resolved it. If you are genuinely blocked, report the blocker the same way instead of guessing.
