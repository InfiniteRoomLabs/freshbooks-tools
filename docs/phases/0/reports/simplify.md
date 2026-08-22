# Phase 0 (scaffold) -- simplification lane report

Branch `phase-0/scaffold`, diff `main...phase-0/scaffold` (`b3d69ac..9545506`). Propose only: nothing was modified, nothing was run beyond `git`, `grep`, and file reads.

**Headline:** the code is in good shape for a first phase. There is no duplicated business logic and no speculative abstraction of consequence. What I found is mechanical: three multi-map/nested-loop spots that collapse to one map, one regex pair that collapses to one regex, a struct with two fields nobody reads, a shell step-list written twice, and a meaningful amount of test-fixture boilerplate. Eight APPLY-RECOMMENDED items, seven OPTIONAL, seven considered-and-rejected.

All proposals were checked against the frozen surface: `testdata/inventory.json` byte layout, the inventory key format, the `-check` report line, exit codes, script CLI contracts, mise task names. Anything that touched those is under DO-NOT-APPLY with the reason stated.

---

## APPLY-RECOMMENDED

### 1. `dedupe` carries three lookup structures where two suffice, and one nested loop that a map replaces

`freshbooks/internal/inventory/inventory.go:396-430`

Before:

```go
result := make([]Entry, 0, len(entries))
sigIndex := make(map[string]int, len(entries))
baseKeyCount := make(map[string]int, len(entries))
...
    finalKey := baseKey
    if baseKeyCount[baseKey] > 0 {
        finalKey = fmt.Sprintf("%s (%s)", baseKey, e.Method)
    }
    for _, existing := range result {          // O(n) scan per entry
        if existing.Key == finalKey {
            return nil, fmt.Errorf(...)
        }
    }
    e.Key = finalKey
    result = append(result, e)
    sigIndex[sigKey] = len(result) - 1
    baseKeyCount[baseKey]++
```

After:

```go
result := make([]Entry, 0, len(entries))
sigIndex := make(map[string]int, len(entries))
takenKey := make(map[string]bool, len(entries))
...
    finalKey := baseKey
    if takenKey[baseKey] {
        finalKey = fmt.Sprintf("%s (%s)", baseKey, e.Method)
    }
    if takenKey[finalKey] {
        return nil, fmt.Errorf(...)            // same message, same trigger
    }
    e.Key = finalKey
    result = append(result, e)
    sigIndex[sigKey] = len(result) - 1
    takenKey[finalKey] = true
```

Behaviour-preserving: `baseKeyCount[B] > 0` and `takenKey[B]` are the same predicate. The first entry with base key B always has count 0, so it takes `finalKey == B` and sets `takenKey[B]`; every later entry with base B sees both as true. The nested `for _, existing := range result` loop tests exactly "is `finalKey` already a key in `result`", which is what `takenKey` now records. I walked the pathological case (a Postman request literally named `Single Tax (DELETE)` sharing a folder with `Single Tax`) in both orders -- verdicts identical. Error message and text unchanged, so the committed `inventory.json` and the golden test are untouched.

Risk: **low**. Net: -6 lines, one fewer map, removes the only nested loop in the package.

### 2. `loadIgnoreList` has two near-identical directive branches that one alternation regex collapses

`freshbooks/internal/inventory/check.go:232-235, 261-281`

Before: `ignoreDirective` and `todoDirective` regexes, then two ~10-line blocks that differ only in which map is `into` and which is `other`.

After:

```go
var directive = regexp.MustCompile(`^//go:inventory-(ignore|todo)\s+(.+)$`)
...
    m := directive.FindStringSubmatch(line)
    if m == nil {
        return nil, fmt.Errorf("%s:%d: unrecognized directive: %q", path, lineNo, line)
    }
    key, reason, err := splitKeyReason(m[2])
    if err != nil {
        return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
    }
    into, other := list.Ignore, list.Todo
    if m[1] == "todo" {
        into, other = list.Todo, list.Ignore
    }
    if err := addUnique(into, other, key, reason, path, lineNo); err != nil {
        return nil, err
    }
```

Behaviour-preserving: `ignore` and `todo` are disjoint literal prefixes, so trying them in sequence and matching them by alternation accept exactly the same lines. Every error string is byte-identical, including the `unrecognized directive` fallthrough (which now happens on the same set of lines: those matching neither prefix). Verified against all six `TestLoadIgnoreList*` cases plus the committed 217-line `testdata/ignore.list`.

Risk: **low**. Net: -11 lines, one regex instead of two.

### 3. `implementation.File` and `.Line` are populated and never read

`freshbooks/internal/inventory/check.go:144-148, 206-225`, consumed at `check.go:100-102`

`scanFile` computes `fset.Position(c.Pos())` for every match and stores `File`/`Line`; `Check` reads only `im.Key`. The obvious use for them -- naming the file and line in the `double-covered:` / `unknown:` report lines -- is off the table this phase, because the `-check` report format is frozen observable behaviour.

Before/after:

```go
type implementation struct { Key string; File string; Line int }
func scanFile(path string) ([]implementation, error)
func scanPackages(pkgs []string, dir string) ([]implementation, error)
```
becomes
```go
func scanFile(path string) ([]string, error)      // keys only
func scanPackages(pkgs []string, dir string) ([]string, error)
```

and `Check`'s `for _, im := range impls { implCount[im.Key]++ }` becomes `for _, k := range impls { implCount[k]++ }`.

Behaviour-preserving: nothing reads the dropped fields; the `token.FileSet` is still required by `parser.ParseFile` and stays.

Risk: **low**. Net: -8 lines, one type gone. Caveat for the lead: Phase 2 will want file:line back when real `// inventory:` comments start colliding -- reintroduce them then, together with the richer report line that will need re-specifying anyway. Carrying dead fields for one phase to save a five-line re-add is not worth it.

### 4. `scanPackages` shadows its own `dir` parameter

`freshbooks/internal/inventory/check.go:152` (param) vs `:159` (loop variable)

```go
func scanPackages(pkgs []string, dir string) ([]implementation, error) {
    dirs, err := packageDirs(pkgs, dir)
    ...
    for _, dir := range dirs {        // shadows the parameter
```

Rename the loop variable to `pkgDir`. Not a bug today (the parameter is consumed on the line before), but it is a live trap for the next edit -- anyone adding a use of the original `dir` inside the loop silently gets the wrong one.

Behaviour-preserving: pure rename. Risk: **none**.

### 5. `coverage-gate.sh`: the head/tail/grep dance is one grep

`scripts/coverage-gate.sh:20-25`

Before:

```bash
filtered=$(mktemp)
trap 'rm -f "$filtered"' EXIT
{
  head -n 1 "$usage_coverprofile"
  tail -n +2 "$usage_coverprofile" | grep -v '/main\.go:' || true
} >"$filtered"
```

After:

```bash
filtered=$(mktemp)
trap 'rm -f "$filtered"' EXIT
grep -v '/main\.go:' "$usage_coverprofile" >"$filtered" || true
```

Behaviour-preserving: the `mode: atomic` header line cannot contain `/main.go:`, so `grep -v` passes it through unchanged -- the head/tail split exists only to protect a line grep was never going to drop. `|| true` is retained so an empty profile still yields the existing vacuous-pass path at `:27`.

**On the lead's specific question -- is there a simpler honest way to do the main.go exclusion?** Not within this phase. I checked the two alternatives:

- Filter `go tool cover -func` *output* instead of the profile: does not work. `-func` prints per-function percentages, not statement counts, so recomputing a weighted total from it means re-implementing the weighting. Strictly worse.
- Measure `main()` for real via `go build -cover` / `GOCOVERDIR` and drop the filter entirely: this is the honest fix, and it is the one the implementer already flagged (report item 4). It is new machinery, not a simplification, and it belongs to whichever phase gives `mcp`/`cli` enough surface to be worth exec'ing a built binary.

So: filtering the profile is the right shape, the comment at `:12-19` earns its keep (it carries the by-filename-not-by-directory rationale, which is the non-obvious part), and the only cut available is the mechanical one above.

Risk: **low**. Net: -3 lines.

### 6. `build.sh` recomputes an identical version string inside the loop, behind a `cd` that does nothing

`scripts/build.sh:24-26`

```bash
for binary in "${binaries[@]}"; do
  read -r module name pkg <<<"$binary"
  version=$(cd "$repo_root/$module" && git -C "$repo_root" describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")
```

The `cd "$repo_root/$module"` is inert -- `git -C "$repo_root"` overrides the working directory -- so the expression does not depend on `$module` and produces the same string on both iterations. Hoist it above the loop and drop the `cd`:

```bash
version=$(git -C "$repo_root" describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")

for binary in "${binaries[@]}"; do
  read -r module name pkg <<<"$binary"
```

Behaviour-preserving: same value, stamped into the same 12 binaries. Risk: **low**.

### 7. `check.sh` writes the ordered step list twice

`scripts/check.sh:75-89` (the `run_step` dispatcher) and `:94-104` (the `all` branch spelling the same six steps out as direct calls)

Before, `all` hardcodes:

```bash
all)
  for module in "${modules[@]}"; do
    run_fmt_check "$module"
    run_vet "$module"
    run_lint "$module"
    run_test "$module"
    run_cover "$module"
    run_inventory_check "$module"
  done
  run_build
  ;;
```

After, one source of truth:

```bash
steps=(fmt-check vet lint test cover inventory-check)
...
all)
  for module in "${modules[@]}"; do
    for step in "${steps[@]}"; do
      run_step "$step" "$module"
    done
  done
  run_build
  ;;
```

Behaviour-preserving: identical step order, identical per-step banners, identical exit semantics under `set -e`. Adding a seventh step now means touching one array instead of two lists that can drift apart silently.

Risk: **low**. Net: -5 lines and removes a real drift hazard between the gate's `all` path and its single-step path.

### 8. `redaction-check.sh` calls git twice to answer one question

`scripts/redaction-check.sh:26-29`

```bash
if ! git cat-file -e ":$file" 2>/dev/null; then
  continue # deleted in this commit; nothing staged to scan
fi
content=$(git show ":$file" 2>/dev/null || true)
```

becomes

```bash
# skip paths deleted in this commit; nothing staged to scan
content=$(git show ":$file" 2>/dev/null) || continue
```

Behaviour-preserving: `git show ":$file"` fails on exactly the paths `git cat-file -e ":$file"` rejects (absent from the index). Today a failure already falls through to an empty `content` that matches no term; the merged form skips explicitly, which is the same outcome one branch earlier. The `|| continue` keeps `set -e` from killing the script.

Risk: **low**. Net: -3 lines, one git process per staged file instead of two.

---

## OPTIONAL

### 9. Table-drive the four ignore-list error tests

`freshbooks/internal/inventory/check_test.go:212-256`

`TestLoadIgnoreListMalformedLine`, `TestLoadIgnoreListDuplicateKey`, `TestLoadIgnoreListKeyInBothListsFails`, `TestLoadIgnoreListUnknownKeyFails` are four functions with an identical body (`newFixtureModule` with an empty `impl.go`, `writeInventoryFixture(t, "Clients/List Clients")`, an ignore fixture, one `Check` call) and an identical assertion (`err != nil`). Only the ignore-list lines and the message differ.

```go
func TestLoadIgnoreListErrors(t *testing.T) {
    tests := []struct{ name string; lines []string }{
        {"[sad] no ' -- ' separator", []string{"//go:inventory-ignore Clients/List Clients no separator here"}},
        {"[sad] key listed twice", []string{"//go:inventory-ignore Clients/List Clients -- first", "//go:inventory-ignore Clients/List Clients -- second"}},
        {"[sad] key in both lists", []string{"//go:inventory-ignore Clients/List Clients -- reason", "//go:inventory-todo Clients/List Clients -- phase-2"}},
        {"[sad] key absent from inventory", []string{"//go:inventory-ignore Nonexistent/Key -- reason"}},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) { ... one Check call, want err ... })
    }
}
```

Behaviour-preserving: same four scenarios, same assertion, same coverage of `loadIgnoreList`/`validateIgnoreList`. Net roughly -25 lines, and the four cases become readable side by side. Risk: **low**. Optional rather than recommended only because it renames test functions, which shows up in any `go test -v` diff a reviewer is eyeballing.

### 10. A `runCheck` test helper for the 15 repeated `Check(CheckOptions{...})` calls

`freshbooks/internal/inventory/check_test.go`, every test function

`Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir})` appears verbatim 15 times at ~110 characters. A helper alongside the existing `newFixtureModule` / `writeInventoryFixture` / `writeIgnoreFixture` trio:

```go
func mustCheck(t *testing.T, dir, inv, ignore string) CheckReport {
    t.Helper()
    report, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir})
    if err != nil {
        t.Fatalf("Check() error = %v", err)
    }
    return report
}
```

collapses the four-line call-plus-error-check at eleven sites to one line each. The four error-path tests keep the raw call (they want the error). Behaviour-preserving. Net ~-35 lines. Risk: **low**.

### 11. A collection-builder helper for the repeated `&Collection{Item: ...}` nesting

`freshbooks/internal/inventory/inventory_test.go:117-125, 160-174, 188-198, 206-216, 238-243, 258-265, 385-401, 413-418, 425-430, 443-448`

Ten tests hand-build the same folder/subfolder/request nesting. Three of them (`TestNormalizeNoBodyIsNil`, `TestEntryJSONHasNoNullSlices`, `TestWriteAndReadJSONRoundTrip`) build a *byte-identical* `Clients / List Clients` collection.

```go
// oneRequest builds a collection holding a single request at trail[0]/.../trail[n-1]/name.
func oneRequest(req *Request, name string, trail ...string) *Collection { ... }

// clientsListRequest is the shared do-nothing fixture used by the body/JSON/round-trip tests.
func clientsList() *Collection { ... }
```

Behaviour-preserving: same structs, same assertions. Net ~-40 lines and the interesting field of each test (the URL, the method, the folder name) stops being buried in five lines of braces. Risk: **low**. This is the single largest cut available in the diff.

### 12. `mustReadFile` instead of `readFile` + four hand-written error checks

`freshbooks/internal/inventory/inventory_test.go:11-17`, used at `:495-502` and `:563-570`

The file already has `mustLoad` and `mustNormalize` with `t.Helper()`; `readFile` is the odd one out, and every one of its four call sites is immediately followed by `if err != nil { t.Fatal(err) }`. Converting it to `mustReadFile(t *testing.T, path string) string` matches the file's own convention and cuts twelve lines. Behaviour-preserving. Risk: **none**.

### 13. `runCheck`'s four print loops table-drive

`freshbooks/internal/inventory/main.go:166-177`

```go
for _, bucket := range []struct {
    label string
    keys  []string
}{
    {"uncovered", report.Uncovered},
    {"double-covered", report.DoubleCovered},
    {"stale", report.Stale},
    {"unknown", report.Unknown},
} {
    for _, k := range bucket.keys {
        _, _ = fmt.Fprintf(stdout, "  %s: %s\n", bucket.label, k)
    }
}
```

Behaviour-preserving: same bucket order, same `"  %s: %s\n"` format, so the frozen `-check` output is untouched. Roughly line-neutral; the gain is that the four labels sit together and cannot drift from `CheckReport.String()`'s wording. Risk: **low**. Take it or leave it.

### 14. `mcp`'s `run` takes an `args` parameter nothing uses

`mcp/cmd/freshbooks-mcp/main.go:120`

```go
func run(w io.Writer, args []string, v string) error
```

`args` is never read; `main` passes `os.Args[1:]` and the test passes `nil`. The Phase 0 work order asked for a `run(w io.Writer) error`-style seam, so this is plumbing added beyond spec for a Phase 3 that will restructure this into cobra anyway. Dropping it gives `run(w io.Writer, v string) error` and one fewer argument at both call sites.

Behaviour-preserving: the parameter has no effect. Risk: **low**. Optional rather than recommended because it is genuinely a coin-flip whether re-adding it in Phase 3 costs more than carrying it.

### 15. Float-compare the coverage percentage in one awk call

`scripts/coverage-gate.sh:42-49`

```bash
scaled_percent=$(awk -v p="$percent" 'BEGIN { printf "%d", p * 10 }')
scaled_threshold=$(awk -v t="$usage_threshold" 'BEGIN { printf "%d", t * 10 }')
if [ "$scaled_percent" -lt "$scaled_threshold" ]; then
```

becomes

```bash
if ! awk -v p="$percent" -v t="$usage_threshold" 'BEGIN { exit !(p + 0 >= t + 0) }'; then
```

Two awk processes and a scaling hack become one awk process. Behaviour: identical for every input this script can actually receive -- `go tool cover -func` prints one decimal place, so multiplying by ten is exact and the two comparisons agree. They could diverge only for a percentage with two or more decimals, which the tool never emits. Risk: **low-medium** (it is a numeric comparison in a gate; the equivalence argument depends on `go tool cover`'s output precision, so leave it if the lead wants zero movement on the gate's arithmetic).

---

## DO-NOT-APPLY (considered and rejected -- recorded so the lead does not re-derive them)

### 16. Collapse `ci.yml`'s three jobs into a matrix

`.github/workflows/ci.yml:10-43` -- three jobs whose bodies differ only in the module argument. Rejected for two independent reasons: (a) the job names `lib`, `mcp`, `cli` are the required status-check contexts wired into `scripts/branch-protection.sh:12-15`, and a matrix changes the context string unless every leg overrides `name:`, which is exactly the kind of implicit coupling that breaks a protected branch silently; (b) `mcp` and `cli` both `needs: lib`, and matrix legs cannot depend on each other, so it collapses to two jobs at best. Ten lines saved against a chance of silently unprotecting `main`. Not worth it.

### 17. Share the two `.goreleaser.yaml` files

`mcp/.goreleaser.yaml` and `cli/.goreleaser.yaml` are near-identical. Rejected: the implementer already documented (report item 3) that goreleaser OSS lacks the monorepo config, that the current arrangement is unverified against a real tag push, and that Phase 5 will do a live dry run. Deduplicating a config whose correctness is not yet established just makes the Phase 5 fix harder to localise. Revisit after Phase 5.

### 18. Replace `stripWhitespace` with a stdlib call

`freshbooks/internal/inventory/inventory.go:225-235`. `strings.Fields`/`unicode.IsSpace` cover more code points than the six ASCII characters this strips (NBSP, U+2028, and friends), so swapping them in is a behaviour change on URLs containing those bytes -- out of bounds for this lane. The exactly-equivalent version, `strings.Map` with a `strings.ContainsRune(" \t\n\r\v\f", r)` predicate, is the same length as the explicit loop and no clearer. No win either way.

### 19. Replace `normalizeQueryString` with `url.ParseQuery`

`freshbooks/internal/inventory/inventory.go:237-258`. `url.ParseQuery` returns a `map[string][]string`, which loses parameter order and merges repeats. Query order is serialised into `testdata/inventory.json`, so this would change the committed bytes. The hand-rolled splitter is correct and required.

### 20. Unexport the `inventory` package's API

`freshbooks/internal/inventory/*.go`. `Load`, `Normalize`, `Entry`, `Check`, `WriteJSON`, `ReadJSON`, `CheckOptions`, `CheckReport`, and the `Family*` constants are exported inside a `package main`, so nothing can ever import them, and `revive`'s `exported` rule then demands a doc comment on each. Tempting to unexport. Rejected: the Phase 0 work order (section B) names `Load`, `Normalize`, `Entry`, and `Check` as exported, and every one of the resulting doc comments carries real information (the dedupe/disambiguation contract, the ignore-list directive grammar, the byte-stability guarantee). This is documentation the next phase needs, not ceremony.

### 21. Drop `scanPackages`' `filepath.Base(dir) == "testdata"` guard

`freshbooks/internal/inventory/check.go:160`. `go list ./...` never returns a directory named `testdata` -- cmd/go excludes them -- so the guard never fires and `TestCheckIgnoresTestFilesAndTestdata` would pass without it. Rejected: it is one line, it states the exclusion the test asserts, and removing it makes the test pass for a reason the reader cannot see. Worth naming to the code-review lane, though: that test currently proves less than its name claims, because the `testdata/` half is enforced by the Go toolchain rather than by this code.

### 22. Fold `buildEntry`'s key assembly into a single trimmed-segment slice

`freshbooks/internal/inventory/inventory.go:124-133`. A single `segs` slice trimmed once, with `folder`, `path`, and `name` sliced back out of it, saves about four lines. Rejected: `Entry.Path` would then alias the key's backing array, which is safe today only because nothing mutates it -- a footgun disproportionate to four lines. The current version's one visible cost (trimming in two places) is cheaper to read.

### 23. Remove the redundant sort in `WriteJSON`

`freshbooks/internal/inventory/inventory.go:436-438` re-sorts entries that `dedupe` already sorted at `:428`. Rejected in both directions: dropping `WriteJSON`'s sort makes the byte-stability guarantee depend on caller discipline (and `writeInventoryFixture` in `check_test.go` is a caller that does not sort); dropping `dedupe`'s sort changes `Normalize`'s documented "sorted by Key" return contract. The duplicated `sort.Slice` over 213 entries costs nothing measurable and each one guards a different invariant. Leave both.

---

## Two findings I am routing rather than proposing

**A. `mise run check -- <module>` cross-builds all six targets for both binaries regardless of the module filter.** `scripts/check.sh:94-104` -- the `all` branch honours `${modules[@]}` for the six per-module steps, then calls `run_build` unconditionally, and `scripts/build.sh` always builds both `mcp` and `cli`. In CI that means all 12 cross-compiled binaries are produced three times (once per job in `ci.yml`), for 36 builds where 12 would do. Making `run_build` respect the module filter is straightforward, but it changes what `mise run check -- freshbooks` prints, which is inside the frozen surface I was told not to touch. Flagging it for the lead to route -- it is a CI wall-clock item, not a readability one.

**B. `acronyms` currently encodes nothing that `titleCaseWord` does not.** `freshbooks/internal/inventory/inventory.go:269-272` maps `"id" -> "Id"` and `"uuid" -> "Uuid"`; `titleCaseWord` produces exactly those strings from the same inputs, so the branch at `:362-366` is a no-op and the map could shrink to a `map[string]bool` set used only by `peelAcronymSuffix`. I am not proposing the cut, because the work order's normalization rule 2 explicitly asks for "a small fixed map for known acronyms", and the map is the documented extension point for the first acronym that needs non-title casing (an `"api" -> "API"` would make it load-bearing immediately). Recording it so a future reader does not spend the same ten minutes deciding it is dead code.

---

## Deliberately left alone

- **`inventory.go` normalization rules** (`normalizePathSegments`, `classifyFamily`, `normalizeVarName`, `peelAcronymSuffix`). Each maps one-to-one onto a numbered spec rule, each has a table-driven test, and the `switch` in `classifyFamily` is already the flattest form. The one true nit is a redundant `continue` at `:299` as the last statement of the loop body -- below the threshold of worth changing.
- **`Entry`'s field set and JSON tags.** Field order is the frozen byte layout. `Duplicates` is 1 for every entry in the real collection today but is spec-mandated and load-bearing for the golden test's 213-leaf assertion.
- **`normalizeURL`'s five return values.** A struct would be tidier in the abstract, but there is one caller and the values are consumed immediately into an `Entry` literal. Introducing a type to avoid five returns is the abstraction this lane is supposed to be arguing against.
- **`addUnique` / `splitKeyReason`.** Small, single-purpose, good error messages, no duplication.
- **`.golangci.yml`.** `exclusions.presets: []` looks like it could be deleted but is the golangci-lint v2 spelling of the work order's `issues.exclude-use-default: false`, and `max-issues-per-linter: 0` / `max-same-issues: 0` are required to stop truncation. Nothing to cut.
- **`scripts/build.sh` and `scripts/docs.sh` as separate scripts.** The lead asked whether they earn their existence versus inline mise task bodies. `build.sh` clearly does -- 43 lines, a two-dimensional matrix, and version stamping. `docs.sh` is a three-line echo that could be a `run =` string today, but it is the Phase 4 seam for `cobra/doc` generation and inlining it now just means un-inlining it in two phases. Keep both. Worth noting though: `check.sh`'s `build)` and `docs)` cases (`:92-93`) are unreachable -- mise's `build` and `docs` tasks call `scripts/build.sh` and `scripts/docs.sh` directly, and the `all` branch calls `run_build` without going through the dispatcher. Cutting them would trim check.sh's advertised subcommand list, which is a CLI contract I was told to treat as frozen, so I am leaving them and noting them here instead.
- **`cli/internal/cmd/root.go:24`**, `root.CompletionOptions.DisableDefaultCmd = false`, which sets cobra's own default. Deletable, but it documents that the completion command is deliberate rather than accidental, and `TestCompletionCommand` depends on the behaviour. Net zero.
- **The seven `doc.go` placeholder packages.** Each carries a real "Phase N fills this in, see docs/X.md" pointer. That is the kind of comment the brief says to keep.
