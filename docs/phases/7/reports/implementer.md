# Phase 7 implementer report -- live conformance (2026-09-03)

Branch `phase-7/live`, resumed from `fb5302c` with the previous run's uncommitted work in the tree (expenses + gateways code, one re-seeded fixture, seven new capture directories). None of it was discarded; all of it is committed below.

Attended, read-only. The authorized account is IRL's real books with a 45-scope token; there is no sandbox, so every write fact stays DEFERRED per `lead-sandbox.md`. Nothing was created, updated, or deleted.

## Fact table

| Id | Fact | Verdict | Capture | Observation |
|---|---|---|---|---|
| B | Business-family filter encoding is bare `field=value` | **CONFIRMED** | `seed/time_entries/{error_422_bare_filter,list_bracket_filter_ignored,list}.json` | Proved through the validator, not through results (the account has zero time entries): `updated_since=notadate` answers **422** `{"errno": 2001, "error": {"updated_since": "Input should be a valid datetime or date, input is too short"}}`; the same value as `search[updated_since]=notadate` answers **200** unfiltered. The accounting spelling is ignored there, not rejected. |
| C | Business-family sort direction (`-field` vs `field_desc`) | **UNRESOLVED** (documented) | `seed/projects/{list,list_sort_minus,list_sort_suffix}.json` | The API validates `sort` not at all: 200 for `-updated_at`, for `updated_at_desc`, and for `no_such_field_desc`, each echoed back verbatim in `meta.sort`. With zero projects, orderings cannot be compared either. `Sort()` left unchanged; backlog item 1 re-cut with what would close it. |
| D | `/events/` callbacks use the accounting envelope | **CONFIRMED** | `seed/callbacks/list.json` | Raw body is `{"response":{"result":{"callbacks":[],"page":1,"pages":0,"per_page":3,"total":0}}}`, `Content-Type: application/json`. The Phase 2 classification was right. |
| E | `/payments/` gateways answer flat | **CONFIRMED + CORRECTED** | `seed/gateways/get.json` | Flat (`application/vnd.api+json`, no envelope) as inferred. But the connection shape was wrong: this account answers `"stripe": null`, **no `fbpay` key at all**, and the whole connection under **`stripe_unified`** -- a key set absent from the Postman example. `GatewayConnection` decoded a fully connected account as three nil pointers. |
| F | Ledger family flat `{"data": ...}`; taxonomy shapes | **CONFIRMED + CORRECTED** | `seed/ledger_accounts/{list,types,sub_types,sub_type}.json` | Family flat, confirmed. Both taxonomy endpoints (no Postman example, no docs page, so Phase 2 returned them raw) were guessed wrong: `types` answers five one-key **objects** (`{"name": "asset"}`), not bare strings, and spells the income type `income`, not `revenue`; `sub_types` answers objects with a **numeric** `id` (the fixture quoted it) plus a `base_number` key the fixture lacked. |
| O | `PageMeta` drops `meta.sort` | **CONFIRMED + CORRECTED** | same as C | `meta.sort` is present on every business-family list. `Page[T]`/`PageMeta` now carry `Sort []string`. A second drop found in the same block is deferred as backlog item 12 (see below). |
| P | `StaffService.List` discards sibling business fields | **CONFIRMED + PARTLY CORRECTED** | `seed/staff/list.json` | Two *member* keys were being lost silently: `identity_uuid` and `language`. Both now decode. The seventeen sibling *business* keys stay unreturned on purpose; the reason is now written next to `staffListResponse`. |
| Q | `DateTime` zoneless-format producers | **PARTLY CONFIRMED** | `seed/expenses/list.json`, `seed/ledger_accounts/list.json`, `seed/gateways/get.json` | Three of four layouts have named producers (see the spec 5.1 callout). The zoneless `YYYY-MM-DDTHH:MM:SS` has **none on this account**: projects and time entries, the only endpoints whose examples show it, are both empty. It stays INFERRED. A fifth format nothing models appears on the wire: space-separated with fractional seconds on `invoices[].version` / `expenses[].version`. |
| J1 | `Expenses.Vendors` returns a bare string array | **CORRECTED** | `seed/expenses/vendors.json` | It returns the paginated accounting result with one-key objects: `{"page","pages","per_page","total","vendors":[{"vendor":"..."}]}`, default page size 15. The Phase 2 struct decoded to an error, not to a short list. |
| S1 | Real `auth login` through the self-signed loopback | **CONFIRMED** (by the lead, 2026-09-02) | -- | `xdg-open` -> browser rejects the ephemeral cert on the first attempt -> user accepts -> callback lands -> `Login succeeded.`; credentials at `$XDG_CONFIG_HOME/freshbooks/credentials/default.json`, `auth status` `valid: true`, expiry 12h out. The raw `tls: bad certificate` stderr line it produced is fixed in `5411fce`. |
| S2 | CLI end to end | **CONFIRMED** (by the lead, 2026-09-02) | -- | `identity me -o json` returns the membership list (`account_id`, `business_id`, `business_uuid`, `name`, `role`); `clients list --per-page 2 -o json` returns the `{items,page,pages,per_page,total}` envelope with 20 keys per client. |
| S3 | MCP end to end | **CONFIRMED** (by the lead, 2026-09-02) | -- | `freshbooks-mcp serve --transport http` on loopback: `initialize` 200 with `serverInfo.name` `freshbooks-mcp`; `tools/call identity_whoami` with the bearer 200, no `isError`. The server log contains no bearer or JWT fragment. |
| G, H, I, J2, K, L, M, R | write facts | **DEFERRED** | -- | Production books, no sandbox. Not attempted. Backlog item 7 lists each with what it needs. |
| A | PKCE enforcement with a wrong verifier | **DEFERRED** | -- | Needs a second attended consent. |
| N | Tokenization shapes | **DEFERRED** | -- | Needs a test card and a connected gateway. |

## Library changes

Each its own commit, each with a re-seeded fixture, a `TestLive*`, a `freshbooks/CHANGELOG.md` `[Unreleased]` entry, and a spec `STATE AS OF 2026-09-03 (Phase 7, live)` callout.

- `f095cde` **expense vendors**: decode the object entries and walk every page. Silently returning the first 15 would have been a trap, so `Vendors` paginates rather than truncating.
- `21e431b` **unified Stripe gateway**: new `StripeUnifiedConnection` + `StripeCapability`, `GatewayConnection.StripeUnified`. The older `Stripe` field stays -- an account onboarded before the change still answers there. The two null-in-every-capture members (`stripe_requirements_current_deadline`, `configuration`) are `json.RawMessage` rather than guessed.
- `a96e9f7` **ledger taxonomy**: `Types` -> `[]LedgerAccountType`, `SubTypes` -> `[]LedgerAccountSubType`, `SubType` -> `*LedgerAccountSubType`. **This is a breaking library API change** (they returned `json.RawMessage`) and needs a release-note line. It is the change the Phase 2 comment invited once a shape was confirmed live; both the MCP and CLI wrappers return `any`, so nothing downstream broke.
- `70d38a8` **staff member fields**: `IdentityUUID` and `Language` on `BusinessGroupMember`.
- `04bd77a` **`meta.sort`**: `Sort []string` on `Page[T]` and `PageMeta`, `omitempty` so accounting-family output is unchanged. The doc comment says outright that the value is an echo of the request, not proof the sort was understood.
- `c4f05d8` **no code change**: live tests and captures for D and Q.

## CLI change

- `701cdcf` **`auth token` printed expired tokens.** Found while bridging credentials into the live suite: `auth status` reported `valid: false` while `auth token` printed the stale value and exited 0, so the documented `TOKEN=$(freshbooks auth token)` idiom handed callers a credential that 401s on first use. Only an actual API call refreshed it. `cliauth.Token` now refreshes through the store -- the same rotation-persisting path `--refresh` uses -- whenever the stored token is expired or expires within the lib's `DefaultExpirySkew`, and takes a clock so the seam is testable. Four new unit tests; `--refresh` still forces a rotation on a valid token.

## Deferred, with reasons

- **Fact C** (`Sort()` encoding): no evidence can distinguish the spellings on this account. Changing it would trade one unverified spelling for another.
- **Time-entry `meta` totals** (new backlog item 12): `total_logged`, `total_unbilled`, `total_logged_per_team_member`, `total_logged_per_client` are on the wire and dropped. Surfacing them needs a time-entry-specific list result instead of `Page[TimeEntry]` -- a design decision, not a conformance fix.
- **All write facts**: production books.

## Verification

Gate tail (`mise run check`, exit 0, clean tree):

```
== cover: freshbooks ==   coverage-gate: total = 91.8% (floor 90%)  PASS
== cover: mcp ==          coverage-gate: total = 92.1% (floor 90%)  PASS
== cover: cli ==          coverage-gate: total = 91.6% (floor 90%)  PASS
== actionlint ==
== build ==               12 artifacts
check.sh: all OK
```

Live suite: every `TestLive*` in `freshbooks/live_conformance_test.go` plus the Phase 1 `TestLiveIdentity` run green against the real account. No `-run` filter is committed.

Redaction: `scripts/redaction-check.sh` clean before every commit. Separately, the whole working tree was grepped for the real account id, business id, business uuid, business name, identity uuid, member email, and the live Stripe account id and publishable key seen in the probes. The only hit is the company name, which is already public in this repo by design.

`git status --porcelain`: empty.
