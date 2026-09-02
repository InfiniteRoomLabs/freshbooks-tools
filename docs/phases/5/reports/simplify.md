# Phase 5 review gate -- simplification lane

Scope: `git diff main...phase-5/release` (11 commits, `1f0abbd..8b13939`) in `<repo root>`. Propose-only: nothing in this report was applied, no file outside this one was touched, and no gate/test/build was run.

Verdict up front: **the diff is already close to minimal.** Most of what looks like duplication is either load-bearing (the two `resolveVersion` copies, the guard/release double changelog extraction, the three near-identical CI jobs) or is evidence-carrying comment text that the hard constraint and the repo's own conventions say to keep. Three small proposals are worth applying; four more are cosmetic; nine ideas were considered and rejected, with reasons, because several of them are the obvious "cleanups" a later reader will be tempted by.

---

## APPLY-RECOMMENDED

### S1 -- `cli/internal/auth/paths_test.go:117-131`: hand-rolled containment loop that `slices.Contains` replaces

Before:

```go
want := []string{"user:clients:read", "user:invoices:write"}
for _, w := range want {
	found := false
	for _, s := range DefaultScopes {
		if s == w {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DefaultScopes missing %q", w)
	}
}
```

After (add `"slices"` to the import block):

```go
for _, w := range []string{"user:clients:read", "user:invoices:write"} {
	if !slices.Contains(DefaultScopes, w) {
		t.Errorf("DefaultScopes missing %q", w)
	}
}
```

Behaviour-preserving: same two assertions, same `t.Errorf` message and format, same non-fatal semantics (both scopes still get reported in one run). `slices.Contains` is exactly this loop. The `auth` package's tests already import `slices` (`cli/internal/auth/status_test.go:12`), and `cli/go.mod` is `go 1.26`, so this is not a new dependency or a version bump.

Risk: **very low.**

### S2 -- `cli/internal/cmd/version_test.go` and `mcp/cmd/freshbooks-mcp/version_test.go`: four subtests that table-drive cleanly

The two files are byte-identical apart from the package clause (verified with a `diff` of the two with package lines normalized). Each contains one plain subtest plus four that are the same seven lines with two values changed: swap `readBuildInfo`, defer the restore, assert. That is 4 x 8 lines of save/restore boilerplate per file, x2 files.

Before (x4 per file):

```go
t.Run("[edge] a (devel) Main.Version leaves the placeholder", func(t *testing.T) {
	orig := readBuildInfo
	defer func() { readBuildInfo = orig }()
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true
	}
	if got := resolveVersion(devVersion); got != devVersion {
		t.Errorf("resolveVersion(devVersion) = %q, want unchanged %q", got, devVersion)
	}
})
```

After:

```go
t.Run("[happy] a real ldflags version is returned unchanged", func(t *testing.T) {
	if got := resolveVersion("v1.2.3"); got != "v1.2.3" {
		t.Errorf("resolveVersion(%q) = %q, want unchanged", "v1.2.3", got)
	}
})

buildInfo := []struct {
	name    string
	info    *debug.BuildInfo
	ok      bool
	want    string
}{
	{"[happy] falls back to the module pseudo-version when unbuilt",
		&debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, true, "v0.1.0"},
	{"[sad] ReadBuildInfo returning false leaves the placeholder",
		nil, false, devVersion},
	{"[edge] a (devel) Main.Version leaves the placeholder",
		&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true, devVersion},
	{"[edge] an empty Main.Version leaves the placeholder",
		&debug.BuildInfo{Main: debug.Module{Version: ""}}, true, devVersion},
}
for _, tt := range buildInfo {
	t.Run(tt.name, func(t *testing.T) {
		orig := readBuildInfo
		defer func() { readBuildInfo = orig }()
		readBuildInfo = func() (*debug.BuildInfo, bool) { return tt.info, tt.ok }
		if got := resolveVersion(devVersion); got != tt.want {
			t.Errorf("resolveVersion(devVersion) = %q, want %q", got, tt.want)
		}
	})
}
```

Behaviour-preserving: identical subtest names (so the `[happy]/[sad]/[edge]` triage tags survive verbatim), identical inputs, identical assertions, identical save/restore discipline, same coverage of `resolveVersion`'s four branches. The first subtest stays standalone because it is the only one that does not touch `readBuildInfo`. No `t.Parallel()` is introduced -- the seam comment in `version.go` says the package-level var is only safe without it.

Risk: **low.** Apply the same edit to both files (they should stay identical -- that is the only thing keeping the duplication cheap; see S12).

### S3 -- `.github/workflows/release.yml:100-110`: four repeats of the same expression where `working-directory` already exists one step above

Before:

```yaml
      - name: Publish the module release
        if: needs.guard.outputs.module == 'mcp' || needs.guard.outputs.module == 'cli'
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          TAG_NAME: ${{ github.ref_name }}
        run: |
          gh release create "$TAG_NAME" --verify-tag --notes-file /tmp/release-notes.md \
            ${{ needs.guard.outputs.module }}/dist/*.tar.gz \
            ${{ needs.guard.outputs.module }}/dist/*.zip \
            ${{ needs.guard.outputs.module }}/dist/checksums.txt \
            ${{ needs.guard.outputs.module }}/dist/*.sbom.json
```

After:

```yaml
      - name: Publish the module release
        if: needs.guard.outputs.module == 'mcp' || needs.guard.outputs.module == 'cli'
        working-directory: ${{ needs.guard.outputs.module }}
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          TAG_NAME: ${{ github.ref_name }}
        run: |
          gh release create "$TAG_NAME" --verify-tag --notes-file /tmp/release-notes.md \
            dist/*.tar.gz dist/*.zip dist/checksums.txt dist/*.sbom.json
```

Behaviour-preserving: the globs resolve to the same four sets of files (they were already `<module>/dist/...` from the repo root; now they are `dist/...` from `<module>/`), the notes path is absolute, and the step matches the shape of the `Build with goreleaser` step directly above it, which already uses `working-directory` for the same value. The `if:` guard is unchanged, so nothing about which module publishes changes. It also removes the last four raw `${{ }}` interpolations into a shell line in this file, leaving the repo's existing convention (`TAG_NAME` via `env:`) as the only pattern.

One caveat to verify rather than assume: `gh` resolves the target repository from the git remote by walking up from the working directory, so running it from `<repo>/mcp` inside the same checkout resolves the same repo. That is standard `gh` behaviour, but it is the one thing this change depends on, so QA should see the release job succeed once before this is treated as settled -- or the change can simply be deferred to after the 0.1.0 tags ship, since it buys clarity, not correctness.

Risk: **low**, with the `gh`-repo-detection caveat above. If the lane wants zero risk on the release path this cycle, defer S3 rather than reject it.

---

## OPTIONAL

### S4 -- `cli/internal/docsgen/docsgen_test.go:47-70`: five sequential `strings.Contains` checks

`TestGenerate` is five `if strings.Contains(...) { t.Error(...) }` blocks, four positive and one negative. Two small tables read better and make adding a case a one-line edit:

```go
for _, want := range []string{
	"# freshbooks CLI reference",
	"## freshbooks clients list",
	"## freshbooks clients",
} {
	if !strings.Contains(s, want) {
		t.Errorf("missing %q", want)
	}
}
for _, bad := range []struct{ substr, why string }{
	{"Auto generated by spf13/cobra", "DisableAutoGenTag did not suppress the date-stamped footer (would break idempotency)"},
	{"hidden docs command", "a Hidden command must not appear in the generated reference"},
} {
	if strings.Contains(s, bad.substr) {
		t.Error(bad.why)
	}
}
```

Behaviour-preserving: same five assertions, same failure conditions. The negative cases keep their prose messages because those messages carry the *why* (idempotency; hidden-command suppression) and are worth more than the uniformity. Marginal win -- five checks is not yet enough repetition to demand a table.

Risk: **very low.** Skip without regret if the lane is trimming.

### S5 -- `freshbooks/CHANGELOG.md`: `Page[T]` is introduced twice

The Types bullet ends "`Page[T]` and `identity.go`'s `User`/`Membership` carry `json` tags, so marshaling either ... produces the documented snake_case wire shape", and the Pagination bullet two lines later opens "`Page[T]`, `PageMeta`, and the `All` iterator (`iter.Seq2`) over every paginated list." A reader of the 0.1.0 release notes meets the same type as if it were new, twice.

Suggested: leave the Pagination bullet as the place `Page[T]` is introduced, and in the Types bullet reduce the clause to the fact that is actually about types -- the `json` tags on `identity.go`'s `User`/`Membership` and on `Page[T]` -- e.g. "... `Page[T]` (see Pagination) and `identity.go`'s `User`/`Membership` carry `json` tags ...". Content-preserving; this is release-notes prose, not behaviour.

Risk: **none** (prose). Value: small.

### S6 -- `docs/building.md:16`: the gate paragraph is now one ~10-line sentence-chain

F4 appended a `docsgen` parenthetical mid-sentence and F6 appended the dirty-tree exclusion sentence to the end of an already long paragraph. Both facts are correct and belong on the page; they are just hard to find inside one block. Splitting the final "The dirty-tree check excludes `docs/phases/*/reports/`: ..." sentence into its own paragraph (no rewording) would make the D8 note findable without changing a word of content, and keeps the house no-hard-wrap rule (one paragraph per line).

Risk: **none** (docs formatting only). Value: readability.

### S7 -- `docs/getting-started.md`: the two `go install ... # once v0.1.0 tags ship; mise run build until then` lines

The same caveat is spelled out inline twice (CLI section, MCP section) and a third time in `README.md`'s Install section. This is deliberate-looking -- each section should stand alone for someone who lands on it from a search -- so I am not recommending a cut. Flagging it only because it becomes stale in three places at once at tag time; whoever runs the attended tag step (`docs/progress.md` release checklist step 5) should treat `grep -rn 'once.*v0.1.0 tags ship' README.md docs/` as part of that step rather than trusting memory.

Risk: **none.** This is a note for the tag runbook, not a code change.

---

## DO-NOT-APPLY (considered and rejected)

### S8 -- collapse `ci.yml`'s three jobs into one matrix job

`lib`, `mcp`, and `cli` are byte-identical apart from the trailing `mise run check -- <module>` and `needs: lib`, so a `strategy.matrix.module` job looks like an obvious four-lines-each win. **Rejected: it changes observable behaviour.** `scripts/branch-protection.sh:12-14` pins the required status-check contexts on `main` to the literal names `lib`, `mcp`, `cli`; a matrix job publishes contexts named `<job> (<module>)` instead, so branch protection would silently stop requiring any check that exists. It also loses the `needs: lib` fan-out (lib gates the other two), which a matrix would have to re-express as two jobs anyway. Twelve duplicated lines is the correct price.

### S9 -- drop the `guard` job's changelog extraction (the prompt's direct question)

The double extraction is **worth its cost; keep it.** `guard` runs `scripts/changelog-section.sh ... > /dev/null` purely to fail fast, and `release` runs it again for real. The cost is one script invocation plus the `jdx/mise-action` install in `guard` (the script's `#!/usr/bin/env -S usage bash` shebang needs the mise-pinned `usage` binary, so `guard` cannot skip the install while keeping this step). The benefit is that a missing `## [X.Y.Z]` section fails before the `ci` job runs the whole three-module gate -- minutes, on a tag push that is attended and hard to redo cleanly.

I also considered replacing the guard-side call with a mise-free `grep -q "^## \[$version\]" "$module/CHANGELOG.md"`, which would let `guard` drop its `mise-action` step entirely and become a seconds-long pure-bash job. **Rejected:** it re-implements `changelog-section.sh`'s awk heading matcher in a second place, so the two can drift -- and the drift failure mode is precisely the one `guard` exists to prevent (guard green, release red, on a tag that is already pushed). One real invocation of the real script is the correct early fail.

### S10 -- share `resolveVersion` between `cli` and `mcp`

`cli/internal/cmd/version.go` and `mcp/cmd/freshbooks-mcp/version.go` are the same 30 lines (differing only in the `go install` path named in a comment), and their tests are byte-identical. **Rejected: there is no cheaper shape.** The two modules must not import each other, and the only module both already depend on is `freshbooks`, which is a stdlib-only FreshBooks API client -- adding an exported `ResolveVersion` there would put binary-versioning helper API into the library's permanent public surface (a locked-design concern: the lib's exported API is what pkg.go.dev ships and semver protects), to save 30 lines of test-covered, dependency-free code. A third shared module or a `go:generate` copy step costs more than the duplication does. Keep both copies, and keep them identical (S2 should be applied to both or neither).

### S11 -- replace the `extraCommands` hook slice with a tagged/untagged stub pair

`cli/internal/cmd/hooks.go` (untagged `var extraCommands []func(*cobra.Command) *cobra.Command`) + `docs_cmd.go`'s `init()` append + the `for` loop in `root.go:44-46` is the D6 registration mechanism. The alternative -- `docs_hook.go` (`//go:build docsgen`, real `addDocsCmd`) plus `docs_hook_stub.go` (`//go:build !docsgen`, no-op) -- would drop the slice and the `init()` but add a file and a build-tag pair that must be kept mutually exclusive by hand. **Rejected as a wash:** same line count, same number of moving parts, and the current shape is the one D6 specified, is already tested, and generalizes to a second tagged command for free. It is also the version that keeps `docs_drift_test.go` untagged, which is the actual constraint.

### S12 -- merge the two `gh release create` steps in `release.yml`

`Publish the module release` (mcp/cli, with assets) and `Create the lib release` (freshbooks, no assets) share the `env:` block and the `gh release create "$TAG_NAME" --verify-tag --notes-file /tmp/release-notes.md` prefix. **Rejected:** merging needs a bash array built behind an `if`, replacing two flat, greppable, individually-`if`-guarded steps with one step containing conditional logic. That is fewer lines and more thinking -- the wrong trade on a release path that runs unattended on a pushed tag.

### S13 -- `sort.Slice` -> `slices.SortFunc` in `cli/internal/docsgen/docsgen.go:141`

Technically the modern stdlib call. **Rejected:** `sort` is the house spelling for comparator sorts across this repo (`cli/internal/cmd/registry.go`, `cli/internal/output/output.go`, `mcp/internal/tools/registry.go`, `freshbooks/internal/inventory/*`), so this would make the one new file the odd one out. Pure churn.

### S14 -- make `docsgen.Header` a raw string literal

The header is ~40 lines of `... + "`" + ...` concatenation, which reads badly. **Rejected: it is not possible.** Go has no escape for a backtick inside a raw string literal, and the header is Markdown full of inline code spans. Concatenation is the only shape. Worth recording so the next reader does not spend the same ten minutes on it.

### S15 -- `if: needs.guard.outputs.module != 'freshbooks'` instead of `== 'mcp' || == 'cli'`

Shorter, and `guard` already rejects any module outside the three. **Rejected:** the explicit form states which modules build binaries rather than which one does not, and it stays correct on the day a fourth module lands (the negated form would silently try to goreleaser it).

### S16 -- `mise exec -- goreleaser` -> bare `goreleaser` in the release job

`jdx/mise-action` with `install: true` puts the mise shim directory on `PATH`, so the prefix is arguably redundant. **Rejected:** it buys nothing and stakes the release job on a `PATH` side effect of a third-party action rather than an explicit invocation. The explicit form also matches `scripts/docs.sh`, which must work when run directly.

---

## Deliberately left alone

- **The goreleaser rationale comments** (`release.yml:86-91`, both `.goreleaser.yaml` headers, the `docs/building.md` release-flow paragraph). These are evidence, not restatement: each records *why* `--skip=publish,validate` and `project_name` are set, which is the single least-guessable thing in the diff. Keep all three; they serve three different readers (the workflow editor, the goreleaser-config editor, the contributor reading the guide).
- **The Q22 seam comments** (`state.go:308`, `state.go:410`, `auth_cmd.go:77`). One line each stating a package-level mutable var is only safe because no test in the package calls `t.Parallel()`. That is a real invariant a future `t.Parallel()` would break silently; it is not a comment restating code.
- **`redaction_sweep_test.go`'s variadic `assertNoLeak`.** Passing the secret list explicitly at each call site looks repetitive, but that explicitness *is* the Q20 fix -- the point is that a scenario asserts only the secrets it could actually leak. Collapsing back to a default set would restore the vacuous assertion the fold removed.
- **The two overlapping length checks in `TestDefaultScopes`** (`len(DefaultScopes) != 44` and `!= len(scopeObjects)*2`). Redundant-looking, deliberately so: Q4's point is that the derived check is mutation-blind and the literal one is not. Both stay.
- **The near-identical "Release:" bullets in `mcp/CHANGELOG.md` and `cli/CHANGELOG.md`** (and the version-fallback bullets). `scripts/changelog-section.sh` extracts each module's section verbatim as that release's GitHub notes, so each has to stand alone for a reader who will never see the other module's page. Cross-referencing would produce worse release notes.
- **`README.md` vs `docs/getting-started.md` install overlap.** Different jobs: the README carries the honest pre-release status plus the `sha256sum -c` verification recipe; getting-started carries the two-line "get a binary and make your first call" path. Neither is a restatement of the other. (See S7 for the one staleness hazard.)
- **The `docsgen` explanation appearing in `scripts/docs.sh`, `cli/internal/docsgen/docsgen.go`'s package doc, `cli/internal/cmd/docs_cmd.go`, and `docs/building.md`.** Four places, but each is the point of use for a different actor, and each states a different slice of it (how to run it / what the package is / why the file is tagged / what the tag means for the gate). Trimming any one of them would leave a reader in that file without the constraint that explains it.
- **`docs/cli.md`'s generated body.** Untouched by design; the header is only reachable through `docsgen.Header`, and the hard constraint forbids changing its bytes.
