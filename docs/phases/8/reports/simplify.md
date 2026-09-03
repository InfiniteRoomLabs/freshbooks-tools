# Phase 8 gate -- simplification lane

Branch `phase-8/converge`, `git diff main...phase-8/converge` (5 commits `ccccafc..abbe407`, 39 files, +806/-58). Read-only lane: nothing below was applied, no gate/test/build was run.

Constraint honored throughout: no proposal changes observable behaviour or wire encoding (JSON tags, `omitempty`, exported API, tool/command names, script exit codes, generated `docs/cli.md`), and none adds a dependency. Two findings that *would* cross that line are recorded as DO-NOT-APPLY with an escalation note instead.

Verdict: the diff is in good shape. Six proposals worth applying, all local; the largest single win is collapsing six hardcoded registry-size literals to two, so the next new lib method is a one-line change per module.

| ID | Where | Tag | One line |
|---|---|---|---|
| S1 | `mcp/internal/tools/*_test.go`, `mcp/cmd/freshbooks-mcp/run_test.go`, `cli/internal/cmd/*_test.go` | APPLY-RECOMMENDED | 6 hardcoded `169`s -> 2, one per module |
| S2 | `mcp/internal/tools/parity_test.go:187`, `cli/internal/cmd/parity_test.go:187` | DO-NOT-APPLY | keyless sets cannot be shared: separate Go modules |
| S3 | `freshbooks/time_entries.go:53-61,92-101,130-136,164-172` | APPLY-RECOMMENDED | one list-response struct and one decoder, not two |
| S4 | `scripts/redaction-check.sh:47-55` | APPLY-RECOMMENDED | allowlist as one anchored regex |
| S5 | `scripts/redaction-check.sh:104-111` | APPLY-RECOMMENDED | digit sweep as one `grep -oE` pass |
| S6 | `scripts/redaction-check.sh:57-84,140-149` | APPLY-RECOMMENDED | one added-line reader instead of two |
| S7 | `scripts/redaction-check.sh` (4 sites) | OPTIONAL | hoist `${usage_range:-}` once |
| S8 | `scripts/redaction-check.sh:88-94` | APPLY-RECOMMENDED | `local` the loop vars; `content` currently shadows the caller's |
| S9 | `freshbooks/time_entries.go:81-86` | OPTIONAL | two field comments restate the type doc |
| S10 | `freshbooks/expenses.go:92-145` | DO-NOT-APPLY | the trailing block is the clearer arrangement; keep it |
| S11 | `freshbooks/time_entries_test.go:81-155` | DO-NOT-APPLY | subtests are right here; a table would be longer |
| S12 | `freshbooks/time_entries.go:91`, `cli/internal/output/output.go:187-193` | DO-NOT-APPLY (escalate) | `Totals` has no json tag; table output drops it entirely |

---

## S1 -- one registry-size literal per module (APPLY-RECOMMENDED)

The diff edits `168 -> 169` at six code sites and six comment sites. Two of the code sites are load-bearing anchors (the frozen doc row count); the other four re-state a number that is already derivable.

Sites:

- `mcp/internal/tools/parity_test.go:66-67` -- `len(rows) != 169` against `docs/phases/3/tools.md`. **The anchor.** `TestParityAgainstToolsMD` already asserts registry-name set == tools.md-name set in both directions (`parity_test.go:88-113`), so this one literal pins the whole mcp package.
- `mcp/internal/tools/roundtrip_test.go:548-549` -- `len(All) != 169`. Redundant with the above; keep the guard but point it at a name.
- `mcp/internal/tools/unit_test.go:143-144` -- `len(Manifest()) != 169`. The invariant actually under test is "Manifest emits one tool per registry entry, sorted".
- `mcp/cmd/freshbooks-mcp/run_test.go:33-34` -- `len(manifest) != 169` on the `tools` subcommand's JSON. The invariant under test is "the subcommand prints every registered tool".
- `cli/internal/cmd/parity_test.go:65-66` -- `len(rows) != 169` against `docs/phases/4/commands.md`. **The anchor** for the cli package (same bidirectional name check at `parity_test.go:150-176`).
- `cli/internal/cmd/roundtrip_test.go:658-659` -- `len(All) != 169`. Redundant with the above.

Before / after sketch (mcp; cli is the same shape minus the `run_test.go` step):

```go
// parity_test.go -- the one number, with the rationale that is today
// split across four files.
//
// wantRegistrySize is the frozen tool surface docs/phases/3/tools.md
// documents. It is the single number to bump when a lib method is added:
// every other size assertion in this package derives from it or from All.
const wantRegistrySize = 169

func parseToolsMD(t *testing.T) []mdRow {
	...
	if len(rows) != wantRegistrySize {
		t.Fatalf("parsed %d rows from %s, want %d", len(rows), toolsMDPath, wantRegistrySize)
	}
	return rows
}

// roundtrip_test.go
if len(All) != wantRegistrySize {
	t.Fatalf("registry has %d tools, want %d", len(All), wantRegistrySize)
}

// unit_test.go -- assert the real invariant, no literal at all
if len(m) != len(All) {
	t.Fatalf("len(Manifest()) = %d, want one entry per registry Spec (%d)", len(m), len(All))
}

// mcp/cmd/freshbooks-mcp/run_test.go -- likewise
import "github.com/InfiniteRoomLabs/freshbooks-tools/mcp/internal/tools"
...
if len(manifest) != len(tools.All) {
	t.Fatalf("got %d tools, want one per registry Spec (%d)", len(manifest), len(tools.All))
}
```

Why behaviour-preserving: the same assertions fire on the same conditions. `run_test.go` and `unit_test.go` get strictly *stronger* -- today a registry that grew while the printer did not would only be caught because both literals happened to be 169; after the change each test catches its own seam, and `parity_test.go` remains the one place doc-vs-registry drift is detected.

Risk: low. `mcp/cmd/freshbooks-mcp` importing `mcp/internal/tools` is legal (`internal/` is module-scoped, same module) and adds no dependency -- `mcp/cmd/freshbooks-mcp/main.go` already imports it for the real `tools` subcommand. Net effect: adding a 170th method touches one `const` and one doc table per module instead of three or four test files.

### S1b -- the comment sites (OPTIONAL)

`mcp/internal/tools/doc.go:3,6`, `mcp/internal/tools/registry.go:220`, `mcp/internal/tools/roundtrip_test.go:41`, `cli/internal/cmd/root.go:2,24`, `cli/internal/cmd/registry.go:390`, `cli/internal/cmd/parity_test.go:16,247` all carry the number in prose. They cannot derive it. Either accept the churn or drop the number from the four package/type doc comments (`doc.go`, `registry.go`, `root.go`, `roundtrip_test.go:41`) and let them say "one per exported service method" with a pointer to `docs/phases/3/tools.md` for the count. `parity_test.go`'s two mentions are next to the assertion and read better with the number. Purely editorial; no behaviour either way.

## S2 -- the keyless allowed-set is not shareable (DO-NOT-APPLY)

`keylessTools` (`mcp/internal/tools/parity_test.go:181-190`) and `keylessCommands` (`cli/internal/cmd/parity_test.go:181-190`) look like duplication but are not extractable:

- `cli/go.mod` and `mcp/go.mod` are separate modules; neither `require`s the other. Their only shared dependency is `freshbooks` -- which is stdlib-only by locked design (spec section 2) and has no business knowing MCP tool names or CLI verb paths. Sharing would mean a new inter-module dependency or a naming leak into the lib.
- The two sets are not the same data anyway: `"identity whoami"` / `"time-entries list-with-totals"` are `Group + " " + Verb` paths; `"identity_whoami"` / `"time_entries_list_with_totals"` are snake tool names. Any shared table would need a per-surface translation, which is more code than the six lines it replaces.

Keep both. Correct call by the implementer.

### S2b -- print the set deterministically (OPTIONAL)

Both use `map[string]bool` and print it with `%v` in three failure messages (`parity_test.go:216,218,234,236` in each package). Go map iteration order is randomized, so a failing test's message ordering varies run to run -- annoying when diffing CI logs. A sorted slice reads simpler and prints stably:

```go
// before
var keylessTools = map[string]bool{"identity_whoami": true, "time_entries_list_with_totals": true}
if !keylessTools[spec.Name] { ... }

// after
var keylessTools = []string{"identity_whoami", "time_entries_list_with_totals"} // sorted
if !slices.Contains(keylessTools, spec.Name) { ... }
```

Behaviour-preserving: two entries, membership test, `len()` used identically. `slices` is stdlib. Risk: none.

## S3 -- one time-entries list-response struct, one decoder (APPLY-RECOMMENDED)

Two private structs now decode the identical wire body, differing only in whether `meta`'s totals survive:

- `timeEntriesListResponse` (`time_entries.go:53-61`) -- `Meta PageMeta` + `TimeEntries`, used only by `list` (`:130-136`).
- `timeEntriesListWithTotalsResponse` (`time_entries.go:92-101`) -- `Meta struct{PageMeta; TimeEntryTotals}` + `TimeEntries`, used only by `ListWithTotals` (`:164-172`).

`ListWithTotals` does *not* duplicate `List`'s request building -- `List` folds `*TimeEntryListOptions` into opts, `ListWithTotals` takes `RequestOption` directly, which is a deliberate API choice (D1) and fine. What it duplicates is the `client.do` + decode + `newPage` triple. Fold the narrower struct into the wider one and make `list` a projection:

```go
// after -- timeEntriesListResponse and its 6-line comment are deleted.

// timeEntriesListResponse decodes a time-entries list body. Meta embeds
// both PageMeta and TimeEntryTotals so their fields (no keys collide)
// land flat under "meta", matching the wire shape in one decode: the
// totals are the aggregate fields Page[TimeEntry] has no room for, which
// List and Search drop and ListWithTotals keeps.
type timeEntriesListResponse struct {
	Meta struct {
		PageMeta
		TimeEntryTotals
	} `json:"meta"`
	TimeEntries []TimeEntry `json:"time_entries"`
}

func (s *TimeEntriesService) listWithTotals(ctx context.Context, path string, opts []RequestOption) (*TimeEntriesPage, error) {
	var resp timeEntriesListResponse
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return &TimeEntriesPage{Page: *newPage(resp.TimeEntries, resp.Meta.PageMeta), Totals: resp.Meta.TimeEntryTotals}, nil
}

func (s *TimeEntriesService) list(ctx context.Context, path string, opts []RequestOption) (*Page[TimeEntry], error) {
	p, err := s.listWithTotals(ctx, path, opts)
	if err != nil {
		return nil, err
	}
	return &p.Page, nil
}

// ListWithTotals body becomes one line:
return s.listWithTotals(ctx, timeEntriesPath(businessID), opts)
```

Why behaviour-preserving:

- Same single HTTP request, same method, path, family, and opts on every one of the three callers (`List`, `Search`, `ListWithTotals`).
- Decoding a superset struct from the same body cannot change what lands in `PageMeta`: `encoding/json` fills the embedded `PageMeta` fields from the same `meta` keys either way, and `TimeEntryTotals`'s four keys do not collide with `PageMeta`'s five (`page`, `pages`, `per_page`, `total`, `sort`). `newPage(items, resp.Meta.PageMeta)` therefore receives a byte-identical `PageMeta` to today's `newPage(items, resp.Meta)`.
- `List` and `Search` still return `*Page[TimeEntry]`; the extra allocation is one struct copy of a slice header plus five ints.
- `timeEntriesListResponse` has no other referent (grep: 4 hits, all in `freshbooks/time_entries.go`); it is unexported, so nothing outside the module can see the change.

Risk: low. Net: -1 type, -1 duplicated `do` call, and the comment about which callers drop the totals lives in one place instead of being split across `:53-57` and `:64-73`.

## S4 -- seed-number allowlist as one anchored regex (APPLY-RECOMMENDED)

`scripts/redaction-check.sh:47-55` spells the allowlist three ways -- a `case` with six literals, then two separate `[[ =~ ]]` tests, then a hand-rolled `return 1`:

```bash
# before
seed_number_allowed() {
  local n="$1"
  case "$n" in
  8675309 | 4242424 | 5555550100 | 5550100100 | 999999999 | 1111111) return 0 ;;
  esac
  [[ "$n" =~ ^700[0-9]{2}$ ]] && return 0
  [[ "$n" =~ ^0+$ ]] && return 0
  return 1
}

# after (the 12-line evidence comment above it stays verbatim)
seed_number_allowed() {
  [[ "$1" =~ ^(700[0-9]{2}|8675309|4242424|5555550100|5550100100|999999999|1111111|0+)$ ]]
}
```

Why behaviour-preserving: `case` patterns match the whole word, so each of the six literals was already effectively anchored; the two regex tests carried explicit `^...$`. The accept set is unchanged, and the function's exit status is now the `[[ ]]` result -- 0 on match, 1 otherwise, exactly as before. No `local` needed for a single positional read.

Risk: low. The one thing to preserve is the evidence comment naming each number's provenance (Jenny's number, the Stripe test card, QA Q4's pre-existing values) -- that is the part a future reader cannot reconstruct.

## S5 -- digit sweep as one grep pass (APPLY-RECOMMENDED)

`scripts/redaction-check.sh:104-111` enumerates 6+-digit runs by matching one, reporting it, then destructively deleting it from `content` and re-matching. That is a hand-rolled `grep -o`, and the destructive edit makes the loop hard to reason about:

```bash
# before
while [[ "$content" =~ ([0-9]{6,}) ]]; do
  local n="${BASH_REMATCH[1]}"
  if ! seed_number_allowed "$n"; then
    echo "redaction-check: unallowlisted 6+-digit number $n in $file:$lineno" >&2
    found=1
  fi
  content="${content/$n/}"
done

# after
while IFS= read -r n; do
  seed_number_allowed "$n" && continue
  echo "redaction-check: unallowlisted 6+-digit number $n in $file:$lineno" >&2
  found=1
done < <(printf '%s\n' "$content" | grep -oE '[0-9]{6,}')
```

Why behaviour-preserving: bash's `[[ =~ ]]` and `grep -oE` both take leftmost, longest, non-overlapping matches of `[0-9]{6,}`, so the sequence of values yielded is identical -- including the same value twice when it appears twice on one line (today's loop also reports it twice, since it deletes only the first occurrence per pass). `found=1` still lands in the calling shell: the `while` is fed by process substitution, not a pipe, so no subshell. The UUID `sed` strip at `:103` is untouched and still runs first.

Risk: low-medium. Two things to keep in mind: use `-oE`, not `-oP` (PCRE is not present in every `grep` build and nothing here needs it); and `grep` exiting 1 on no-match cannot trip `set -e`, because a process substitution's exit status is not the enclosing command's.

## S6 -- one added-line reader instead of two (APPLY-RECOMMENDED, verify carefully)

Range mode currently extracts added lines twice, in two different styles, and runs `git diff <range> -U0 -- <file>` twice for every seed file:

- `scripts/redaction-check.sh:143` -- ad hoc `git diff "$usage_range" -U0 -- "$file" | grep -E '^\+' | grep -v '^\+\+\+' | sed 's/^\+//'`, feeding `scan_terms`.
- `scripts/redaction-check.sh:57-84` -- `added_lines_with_numbers` / `numbered_lines`, a pure-bash hunk-header walker, feeding `scan_seed_numbers`.

`numbered_lines` is the more capable of the two (it carries line numbers and it handles both modes), so make it the only reader:

```bash
# after -- lines 140-149
while IFS= read -r file; do
  [ -z "$file" ] && continue
  # In range mode this is the diff's added lines; in staged mode the
  # staged file's full content. Deleted-in-this-commit files fail the
  # read and are skipped.
  content=$(numbered_lines "$file" | cut -f2-) || continue
  scan_terms "$file" "$content"
  scan_seed_numbers "$file"
done < <("${files_cmd[@]}")
```

Why behaviour-preserving:

- **Range mode:** `added_lines_with_numbers` already skips the `+++ b/...` header (`:66-67`) and emits `${dline:1}` for every other `+` line -- exactly the set the `grep '^\+' | grep -v '^\+\+\+' | sed 's/^\+//'` chain produces. `cut -f2-` strips the `lineno<TAB>` prefix `printf` added.
- **Staged mode:** `git show ":$file" | cat -n | sed -E 's/^ *([0-9]+)\t/\1\t/'` then `cut -f2-` round-trips the content. `cat -n`'s separator is a TAB and `cut -f2-` keeps every field from the second on, so embedded tabs survive; blank lines come back blank.
- **The deleted-file skip survives.** `git show ":$file"` failing makes the pipeline non-zero under `pipefail`, so the function returns non-zero and `|| continue` fires -- the same skip today's `|| continue` on the bare `git show` provides. The `|| continue` also suppresses `errexit` for that command.

Risk: **medium** -- this is a security control and its failure mode is silent (a file scanned as empty rather than an error). Verification recipe for whoever applies it:

1. Before and after, run `scripts/redaction-check.sh --range main..phase-8/converge` and a staged-mode run over the same staged tree; diff both stderr and stdout. Expect byte-identical output.
2. Add `123456789` to a `freshbooks/testdata/seed/` fixture and confirm both modes still exit 1 naming the right `file:line`.
3. Delete a staged file and confirm staged mode still skips it silently.

Follow-through once applied: `added_lines_with_numbers` is then only ever called with `$usage_range`, so its `range` parameter can go (`:61-62,72`), and `scan_seed_numbers` no longer needs its own reader call -- it can take the already-read numbered lines. That second step is a bigger rewrite; leave it unless the lead wants it.

## S7 -- hoist `usage_range` once (OPTIONAL)

`${usage_range:-}` appears at `:79`, `:134`, and `:142` (and shaped the line S6 deletes) purely to satisfy `set -u` when `usage` did not set the flag. One assignment after `set -euo pipefail` removes the noise:

```bash
set -euo pipefail
usage_range="${usage_range:-}" # unset when --range was not passed
```

Every later test then reads `[ -n "$usage_range" ]`. Behaviour-preserving and `set -u`-safe. Risk: none. Pairs naturally with S6.

## S8 -- `local` the scan_seed_numbers loop variables (APPLY-RECOMMENDED, small)

`scripts/redaction-check.sh:88-94` declares only `local file="$1"`. `lineno`, `content`, and `n` are globals -- and `content` is the *same name* the main loop at `:143-145` uses for the term-scan text. It is harmless today only because `scan_terms` is called before `scan_seed_numbers` (`:147-148`); reorder those two lines, or add a third scanner, and the second one silently scans a clobbered string.

```bash
scan_seed_numbers() {
  local file="$1" lineno content n
```

Behaviour identical today (`n` also stops being re-`local`ed on every loop iteration, which is legal but odd). Risk: none. Cheap removal of a real trap in a security-relevant script.

## S9 -- two comments restate the type doc (OPTIONAL)

`freshbooks/time_entries.go:78-85`. The `TimeEntryTotals` type doc (`:64-75`) already explains at length why the two breakdown lists are raw JSON -- zero time entries on the capture, no populated example in Postman or on the docs page, INFERRED, spec callout. The two per-field comments then say the same thing and point back at it:

```go
// PerTeamMember is the meta.total_logged_per_team_member array,
// undecoded (see the type doc comment).
PerTeamMember json.RawMessage `json:"total_logged_per_team_member,omitempty"`
```

The json tag already names the wire key and `json.RawMessage` already says "undecoded". Dropping both field comments keeps the evidence in exactly one place. Purely editorial.

Everything else in this diff's comments is evidence, not restatement, and should stay as written: the 8-line provenance block at `expenses.go:92-99`, every `null on the capture -- INFERRED` note, the `Version` fractional-seconds rationale (`expenses.go:139-145`), the `jea_id`/`jesa_id` "at most one of the two non-null, confirming int64" note (`ledger_accounts.go:43-52`), the allowlist provenance in the script (`:35-46`), and the UUID-strip rationale (`:96-102`). That is the part of this phase that could not be reconstructed later.

## S10 -- the 14 Expense fields' grouping (DO-NOT-APPLY)

`freshbooks/expenses.go:92-145`. The rest of `Expense` is alphabetical by Go field name; the new fields are a trailing block ordered by JSON tag, with two deliberate adjacency pairs (`ConverseProjectID`/`ModernProjectID` sharing one comment, `ExtInvoiceID`/`ExtSystemID` likewise).

Merging them into the alphabetical body would either lose the 8-line provenance comment or force its substance to be repeated per field, and would scatter fourteen INFERRED fields among the CONFIRMED ones. The block, with one provenance header and per-field evidence, is the clearer arrangement. Leave it.

One thing a reader might trip on: `LegacyAccountID` (tag `accountid`) sits between `AccountName` and `BackgroundJobID`, i.e. sorted by tag rather than by field name. Its comment already explains that it is a second, distinct identifier from `AccountingSystemID`, which is the confusing part; the ordering is fine.

## S11 -- subtests, not tables (DO-NOT-APPLY)

`freshbooks/time_entries_test.go:81-155`: the four subtests need four different servers -- a path-capturing handler over a fixture, an inline JSON body, a query-capturing handler, and an error fixture. A table would need a handler func per row plus per-row assertion funcs, which is longer and less readable than what is there.

`freshbooks/expenses_test.go:27-52` and `freshbooks/ledger_accounts_test.go:68-88` assert named fields against one fixture each, not one shape over many inputs -- also not table material. The `[happy]`/`[sad]`/`[edge]` tags are applied consistently. No change.

## S12 -- escalation, not a simplification: `Totals` has no json tag (DO-NOT-APPLY here)

Out of this lane's remit because both halves are observable-behaviour changes, but both are cheap now (`freshbooks`, `mcp`, and `cli` all carry this under `[Unreleased]`) and expensive after a tag. Handing to code review / QA for a decision:

1. **Casing.** `TimeEntriesPage.Totals` (`freshbooks/time_entries.go:91`) carries no struct tag, so it marshals as `"Totals"` while every field promoted from the embedded `Page[TimeEntry]` is lowercase snake (`items`, `page`, `pages`, `per_page`, `total`, `sort`). `freshbooks time-entries list-with-totals -o json` therefore emits `{"items":[...],"page":1,...,"Totals":{...}}`, and the MCP tool returns the same mixed-case object.
2. **Table and name output drop the totals entirely.** `cli/internal/output/output.go:187-193` unwraps any JSON object carrying an `"items"` key down to that array before rendering a table. `TimeEntriesPage` embeds `Page[TimeEntry]`, so `items` is present at the top level and the unwrap fires -- meaning `time-entries list-with-totals -o table` (and `-o name`) prints byte-identical output to `time-entries list`, with the totals silently discarded. The default output format decides whether the new command does anything at all.

Both are one-line-ish fixes (`json:"totals"`; and either a table-mode note or a totals footer), but each changes emitted bytes, so this lane only reports them.

---

## Not proposed, and why

- **Sharing the `169` literal across modules.** Same wall as S2 -- `cli` and `mcp` are separate modules. Two constants is the floor.
- **Deriving the doc-table row count from the registry.** That would defeat the point: `docs/phases/3/tools.md` and `docs/phases/4/commands.md` are the frozen external surface the registry is checked *against*. The literal belongs there.
- **Touching `docs/cli.md`.** Generated (`-tags docsgen`); off limits and correctly regenerated in the diff.
- **Renumbering the doc tables so row 169 sits beside `time_entries_list`.** Appending is right: renumbering churns 165-168 and every future reference to a row number for no gain, and the totals lines already explain why row 169 is keyless.
- **`TimeEntryListOptions` for `ListWithTotals`.** The plain-`RequestOption` signature is D1's decision and is documented at the method; changing it is an exported-API change.
- **Fixture consolidation.** `freshbooks/testdata/time_entries/list_with_totals.json` is a distinct wire shape (populated `meta` totals and breakdown lists) from `time_entries/list.json`; a shared fixture would make both tests assert less.
