# Phase 1 simplification review -- `phase-1/lib-core`

Lane: simplification (propose only). Scope: `git diff main...phase-1/lib-core`, everything under `freshbooks/` plus `docs/authentication.md`, `docs/library.md`. Constraint honoured: every proposal below is behaviour-preserving on the wire, in errors, in retry counts, in token persistence order, and in the exported API; anything that was not is listed under DO-NOT-APPLY.

Overall verdict up front: this branch is already lean. The code follows the exemplar's style, the tests are table-driven where tables help, comments carry API evidence rather than restating code, and the transport genuinely is the single request path. The list below is short because there is little to cut.

## APPLY-RECOMMENDED

1. **`freshbooks/transport.go:157` -- stop re-deriving the family inside `roundTrip`.** `do()` receives `fam` as a parameter, then `roundTrip` independently recomputes it with `familyForPath(req.URL.Path)` for error decoding. Two sources of truth for one value.
   - Before: `fam := familyForPath(req.URL.Path)` inside `roundTrip`.
   - After: add a `fam Family` parameter to `roundTrip` (called only from `do`), delete the recompute.
   - Behaviour: identical under `DefaultBaseURL` (no path prefix, so `req.URL.Path == path`). Honesty note: with `WithBaseURL(".../v1")` the current recompute classifies `/v1/accounting/...` as `FamilyBusiness` when decoding an error, disagreeing with the family `do()` used for the request; passing `fam` through makes the two agree. That is the latent inconsistency the duplication was hiding -- the code-review lane may want to log it as a bug; from this lane it is a dedup whose only observable effect is aligning `Error.Family` with the request's declared family in the prefixed-base edge case.
   - Risk: low.

2. **Delete `freshbooks/version_test.go`.** It asserts that the compile-time constant `Version` is not `""`. The only way it fails is someone editing the constant to the empty string, which the release tooling would also have to do deliberately. Zero behaviour, zero coverage contribution (constants are not counted).
   - Risk: none.

3. **`freshbooks/client_test.go:53-64` -- `serveFixture` should call `readFixture`.** `readFixture` (errors_test.go:13) and the body of `serveFixture` duplicate the same `os.ReadFile(filepath.Join("testdata", area, name+".json"))` + `t.Fatalf` block in the same package.
   - Before: `serveFixture` opens the file itself.
   - After: `body := readFixture(t, area, name)` as its first line.
   - Risk: none. (The `fixture` copies in `freshbooks/auth` and the `integration`-tagged package are separate packages -- see DO-NOT-APPLY 12.)

## OPTIONAL

4. **Delete `freshbooks/testdata/seed/` (6 files, ~210 lines).** Every seed file is byte-identical (md5-verified) to its copy under `testdata/{accounting,projects,auth}/`, and no Go code reads `testdata/seed/`. The duplication exists because spec line 54 designates `seed/` as "the fixture source of truth for Phase 1" -- but with the copies identical, one copy is the source of truth. Applying this means adding a `> **STATE AS OF ...**` callout to that spec sentence in the same commit (per the inferred-vs-confirmed convention), which is why this is the lead's call rather than a plain apply.
   - Risk: low mechanically; touches a spec-designated location.

5. **`freshbooks/errors.go:113-114` -- `Object` and `Value` fields of `accountingErrorEnvelope` are decoded and never read.** encoding/json ignores unknown fields, so deleting the two struct fields changes nothing. Counter-argument for keeping them: they document the observed wire shape of an accounting error entry, which is exactly the kind of API evidence the conventions say to keep. If cut, preserve the knowledge as a one-line comment.
   - Risk: none either way.

6. **`freshbooks/transport.go:66` -- `max(c.retry.MaxAttempts, 1)` is dead armor.** `c.retry` is unexported, set only by `DefaultRetryPolicy()` (3) or `WithRetry` (which rejects `MaxAttempts < 1`), and copied by value, so the guard is unreachable. `attempts := c.retry.MaxAttempts` suffices. Kept OPTIONAL because one cheap guard on a retry loop is defensible insurance against a future option that forgets to validate.
   - Risk: none today.

7. **`freshbooks/page_test.go:135-142` -- `TestPageMeta` asserts that struct field assignment works.** It exists (per its own comment) for the json tags, but never exercises them. Either delete it, or make it real at the same size: unmarshal `{"page":1,"pages":2,"per_page":15,"total":20}` into a `PageMeta` and compare -- that would actually fail if a tag is misspelled. The unmarshal version is the better trade.
   - Risk: none.

## DO-NOT-APPLY (considered and rejected, so the lead does not re-derive them)

8. **Replace `auth.DefaultTokenPath` with `os.UserConfigDir()`.** Looks like a textbook stdlib swap, but `os.UserConfigDir` returns `~/Library/Application Support` on macOS and `%AppData%` on Windows, where the current code always yields `~/.config/freshbooks/token.json`. That is an observable behaviour change (token file location), and pinning `~/.config` cross-platform is a legitimate CLI convention anyway. If the CLI phase wants platform-native dirs, decide it there.

9. **Reflection over `Client`'s service fields to collapse `registerServices`' 36 assignments.** Trades a boring, greppable, compiler-checked list for `reflect` cleverness. The triple listing (field decls, registration, `services.go` types) is the deliberate Phase-2 batching design: batches add files without touching `client.go`. The doc comments on both the fields and the types are mandated by the enabled `revive` exported rule, so they are not cuttable duplication either.

10. **Merging the auth family back into two `Family` values / collapsing `unwrap`'s cases.** The spec's own STATE callout (section 5.1) settled three families to match the live `/auth/api/v1` envelope and the inventory classifier. Locked.

11. **Tightening `Sort`'s direction handling to `dir == SortDesc`.** Today any `d`/`D`-prefixed string ("DESC", "descending") encodes `_desc`; a strict compare silently flips those callers to `_asc`. Observable query-string change.

12. **Deduplicating `redactPath` (freshbooks) with `endpointName` (auth), and the three per-package test fixture readers.** Same ~10 lines of logic, but they live in different packages; sharing needs a new export or an `internal/` package, which costs more surface than it saves. The lib is two packages by design; a little cross-package echo is cheaper than a shared-helpers package.

13. **Dropping the defensive copy of `body` into `Error.Raw` (`errors.go:135`).** `body` is uniquely owned by the caller today, so the copy is technically unnecessary -- but it is one line and guards `Raw` against aliasing if `roundTrip`'s buffer handling ever changes. Not worth it.

14. **Merging `Date` into `DateTime`** (DateTime already parses bare dates). `Date` always marshals `YYYY-MM-DD`; `DateTime` marshals in its remembered layout. Distinct wire contracts; merging changes marshal output for date-typed fields.

## Deliberately left alone

- **The 36 pre-declared services and the option/type names** (`PageNumber`, `Search`-as-type, etc.): Phase 2 contract per the work order and the spec's STATE callout; out of scope.
- **The test suites generally**: already table-driven where a table earns its keep (`TestDecodeError`, `TestRequestOptionValues`, `TestRetryPolicyDelay`, `TestTokenValid`); the non-tabular tests each assert genuinely different mechanics (redirect stripping, single-flight refresh, atomic rename). Forcing more of them into tables would obscure, not simplify.
- **Single-caller named helpers** (`isRetryableTransportError`, `jsonString`, `truncate`, `flatMessage`, `writeTokenFile`): each carries intent or a doc-comment worth of API knowledge; inlining saves lines and loses names.
- **Comments**: sampled across every file -- they consistently carry API evidence (one-time-use refresh tokens, created_at-based expiry, envelope shapes, the Postman double-listing) rather than restating code. Nothing to cut.
- **`docs/authentication.md` / `docs/library.md`**: tight, no restated reference material, tables where tables help.
- **`Do` + `do` split, `resolve`, `wait`**: each is the minimal seam its tests need; no over-abstraction found.

## Net effect if 1-3 apply

About -20 lines, one file deleted, one duplicated derivation removed. Items 4-7 add roughly another -230 lines (mostly fixture copies) at the lead's discretion.
