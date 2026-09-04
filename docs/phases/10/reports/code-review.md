# Phase 10 code-review lane

Branch `phase-10/docs-site`, 9 commits `d73e122..eb25cc7` (`git diff main...phase-10/docs-site`). Read-only lane: no edits, no gate, no build, no network. Findings below were verified against the source and, where empirically checkable, against the already-built `site/build/` tree.

## Verdict: APPROVE

No blocking findings. The three things most likely to be wrong in a phase like this -- the blanket `sed` link rewrite, the MDX-vs-CommonMark opt-out, and the edit-link/baseUrl plumbing -- are all correct, and I confirmed the last two against rendered HTML rather than taking the config's word for it. Eleven advisories follow; none need to block the merge, but R1, R2 and R9 are worth folding in while the branch is open.

## What I verified (evidence)

**Link rewriting is safe and complete, for today's content.** The `sed -E 's#\(docs/([A-Za-z0-9_-]+)\.md\)#(/\1)#g'` at `scripts/site-sync.sh:59` requires a literal `(` immediately before `docs/`. Grepping all eight synced sources for `(docs/`, the only matches are README.md:52-58 -- the seven guide links, exactly the ones that must be rewritten. Every cross-guide reference inside `docs/*.md` is backticked prose (`docs/authentication.md` at `docs/getting-started.md:114`, etc.), not a markdown link, so nothing in a code block or inline-code span is touched. The nearest miss is `docs/cli.md:674` and 21 sibling lines: `(default asc; ... -- see docs/progress.md)`. That string ends in `)` and contains `docs/progress.md`, but the char before `docs/` is a space, so the regex correctly does not fire. If it had, every one of those 22 generated flag descriptions would have been silently corrupted -- worth stating explicitly since the guides are 8k lines of generated text nobody reads by eye.

**`docs/cli.md` as CommonMark.** `mdx.format: md` is emitted nested (`scripts/site-sync.sh:53-54`), which matches what `@docusaurus/mdx-loader` actually reads in 3.10.2. The implementer's writeup of why the bare top-level `format:` key failed is accurate.

**baseUrl and slugs resolve correctly.** `site/build/index.html` renders the rewritten `(/getting-started)` links as `href="/freshbooks-tools/getting-started"` -- baseUrl is applied, so the `/<slug>` rewrite is right and `onBrokenLinks: 'throw'` had real routes to match against. All seven guides plus the README landing page at `/` exist as built HTML.

**Edit links point at real source.** Every built page's "Edit this page" resolves through `custom_edit_url`, not the config fallback: `.../edit/main/docs/<guide>.md` for the seven guides and `.../edit/main/README.md` for the index. No page points into the gitignored `site/docs/`.

**D7 hygiene holds, empirically.** Grepping every external host in the built HTML returns only prose links (freshbooks.com, github.com, pkg.go.dev, mise.jdx.dev, schema.org JSON-LD, and the localhost/mcp.example.com synthetic examples already in the guides). No font, CDN, or analytics origin is loaded. No `algolia`/`gtag`/`googletagmanager` string in any built page -- `@docusaurus/theme-search-algolia` really is inert. ASCII sweep over every file this phase touched: clean. Operator-string sweep over tracked `site/` files and the whole branch diff (Tailscale `100.x`, `*.lab.*`, `*.internal.*`, `IRL/`, home paths): clean.

**Sidebar matches.** `site/sidebars.js:8-16` lists exactly the seven guides in `sidebar_position` order; `index` is deliberately absent and reachable at `/`. Matches D2 and the implementer's rendered list.

**`site-build` runs once per gate.** `run_site_build` is called from `run_repo_wide` only (`scripts/check.sh:169`), and `all` invokes `run_repo_wide` once, guarded by `module_filter` (`scripts/check.sh:211-215`). `mise run check -- <module>` skips it, matching how the other repo-wide steps behave. `.gitignore:24-27` covers `site/node_modules/`, `site/docs/`, `site/build/`, `site/.docusaurus/`, so the gate's dirty-tree banner stays quiet.

**The `ForceTerminatePlugin` note in `docs/building.md` is true and worth keeping.** I read the installed `site/node_modules/@docusaurus/core/lib/webpack/plugins/ForceTerminatePlugin.js`: it is a single `logger.error(\`Client bundle compiled with errors therefore further build is impossible.\n${formatStatsErrorMessage(...)}\`)` followed synchronously by `process.exit(1)`, wired into the client config at `webpack/client.js:111`. The generic sentence is the head of that one string and the useful detail is the tail, so a truncated non-TTY write drops exactly the half you need. That is a genuinely non-obvious trap and the note will save the next person an hour.

**Gate budget: the site step is within budget.** The plan's 60s applies to the site step, not the whole gate. Warm `site-build` is ~5-7s including sync and `pnpm install --frozen-lockfile`; the reported ~72s wall clock is the full three-module gate, of which the site is under 10%. Comfortably inside 60s locally. See R9 for the CI caveat.

**No unauthorized doc edits.** The diff touches `README.md` (one line) and `docs/building.md` (one new section) only; no other `docs/*.md` changed, and `docs/cli.md` is untouched. `docs/progress.md` is correctly not updated here -- this repo ships that at merge time (`2ebd467`, `3cec68d`), not on the phase branch. Commit scopes are conventional and correct throughout.

## Findings

### R1 -- ADVISORY -- link rewrite drops anchors, `scripts/site-sync.sh:59`

The regex only matches `(docs/<x>.md)` with the closing paren immediately after `.md`. A perfectly ordinary link like `[the invoices commands](docs/cli.md#freshbooks-invoices)` does not match, survives into `site/docs/getting-started.md` as a relative `docs/cli.md#...`, and resolves against `site/docs/` to a nonexistent `site/docs/docs/cli.md`. `markdown.hooks.onBrokenMarkdownLinks: 'throw'` catches it at gate time, so it fails loudly rather than shipping broken -- but the failure will read as a mysterious missing file, and anchors are the single most likely link form to be added next. Fix: `sed -E 's#\(docs/([A-Za-z0-9_-]+)\.md(#[^)]*)?\)#(/\1\2)#g'`.

### R2 -- ADVISORY -- rewrite does not validate the target is a published page, `scripts/site-sync.sh:59`

The substitution is unconditional over any `docs/<name>.md`. `[progress](docs/progress.md)` added to README would become `(/progress)` -- a link to a route that does not exist, since `docs/progress.md`, `docs/phases/` and `docs/superpowers/` are deliberately unpublished. `onBrokenLinks: 'throw'` is the backstop, so again this fails the gate rather than shipping, but the error would point at a rewritten slug with no obvious relationship to the source line. Cheap fix: restrict the alternation to the seven published names, or emit a warning when a rewritten target is not in `pages`.

### R3 -- ADVISORY -- no `concurrency` group on the Pages workflow, `.github/workflows/pages.yml:1-16`

GitHub's own Pages starter workflow carries `concurrency: {group: "pages", cancel-in-progress: false}` precisely because two overlapping runs make `actions/deploy-pages` fail on the loser. Two pushes to `main` in quick succession (a `--no-ff` phase merge followed by a docs touch-up is exactly that shape) can produce a red deploy that needs a manual `workflow_dispatch` to recover. Add the block at the top level.

### R4 -- ADVISORY -- `paths` filter misses two files that feed the site, `.github/workflows/pages.yml:6-10`

The filter lists `docs/**`, `README.md`, `site/**`, `scripts/site-sync.sh` -- which is what D5 specified -- but `mise run site-build` also depends on `scripts/site-build.sh` and on `mise.toml`'s task and tool pins. A change to either (say, bumping the pinned node) will not redeploy. Low impact, since neither changes rendered content on its own, but the filter should name everything the build actually reads. Inverse nit, not worth fixing: `docs/**` also fires on `docs/phases/**` and `docs/progress.md`, which are never published, so some deploys will be no-ops.

### R5 -- ADVISORY -- `ci.yml`'s repo-wide comment is now stale, `.github/workflows/ci.yml:48-52`

The comment enumerates the module-independent steps as "actionlint, shellcheck, the redaction and release selftests, readme-drift-check" and does not mention `site-build`, which this branch added to that job. `mise.toml`'s `repo-wide` and `check` descriptions were both updated correctly; this one was missed. It matters slightly more than usual because that comment is where someone looks to understand why the required CI job got slower.

### R6 -- ADVISORY -- `site/pnpm-workspace.yaml` carries two overlapping mechanisms and two comments that disagree

`allowBuilds: {core-js: true}` (lines 5-6) is what actually unblocked the install -- the implementer's own report says so. `onlyBuiltDependencies: [core-js]` (lines 17-18) is the thing that turned out *not* to be the mechanism, kept alongside it, under a comment block (lines 7-16) that presents it as the mechanism and re-explains the core-js postinstall a second time. A reader hitting this file cannot tell which setting is load-bearing. Keep `allowBuilds` with its comment, drop the rest, or keep both with one comment that says plainly which one pnpm 11 honors and why the other is retained.

### R7 -- ADVISORY -- `gtag: undefined` / `googleAnalytics: undefined` are no-op keys, `site/docusaurus.config.js:50-51`

preset-classic only instantiates those plugins on a truthy option, so omitting the keys and passing `undefined` are identical. As written they read like they are switching something off, which invites a future reader to "fix" them by filling them in. The file's header comment already states the no-telemetry intent; the keys add nothing to it.

### R8 -- ADVISORY -- `defaultMode: 'light'` versus D2's "dark mode on", `site/docusaurus.config.js:59-63`

D2 says "dark mode on". The config ships `defaultMode: 'light'` with `disableSwitch: false` and `respectPrefersColorScheme: true`, so a reader whose OS prefers dark gets dark and everyone can toggle. I think that is the better behavior and would not change it -- but it is a deviation from a written decision, and unlike the two `format`/`onBrokenMarkdownLinks` deviations it is not listed in the implementer report's "reality disagreed" section. Lead should confirm the intent was "dark mode available" rather than "dark by default".

### R9 -- ADVISORY -- the 60s budget was measured warm and local; CI is always cold and uncached

The ~5s figure quoted in `docs/building.md` and in `scripts/check.sh`'s new comment is a warm webpack cache on this laptop. Every CI run of the *required* `repo-wide` job is cold (~26s per the implementer's own measurement) plus a `pnpm install --frozen-lockfile` of 690 packages with no pnpm-store cache in either `ci.yml` or `pages.yml` (`mise-action`'s `cache: true` caches mise tools, not the pnpm store). Neither workflow will be anywhere near 5s. Nothing here is wrong -- the site step still fits 60s -- but the docs currently imply a cost that only holds locally, and adding a store cache keyed on `site/pnpm-lock.yaml` is a few lines. Worth a sentence in `docs/building.md` distinguishing the warm-local number from the CI number.

### R10 -- ADVISORY -- `mise run site` can fail on a fresh clone, `mise.toml` `[tasks.site]`

`scripts/site-sync.sh && mise exec -- pnpm --dir site start` never installs. On a clone that has never run `site-build`, `site/node_modules` does not exist and the dev server fails (loudly, under `verify-deps-before-run=error`, so no silent breakage). The documented first-run path in `docs/building.md` lists `site-sync`, `site`, `site-build` in that order, which reads as though `site` is runnable second. Either add an install to the task or reorder the doc block so `site-build` is the stated first run.

### R11 -- ADVISORY -- `pages` table parsing is colon-fragile and carries a redundant column, `scripts/site-sync.sh:28-41`

`IFS=':' read -r name title position src edit_path` splits a five-field record on `:`, and `title` is the field most likely to acquire a colon ("MCP server: transports" is a plausible future rename) -- at which point the remainder silently shifts into `edit_path` and the edit links break with no error. Separately, `src` and `edit_path` are byte-identical in all eight rows; the fifth column exists but never differs. Either drop it and use `$src` for both, or, if the intent was to allow them to diverge, say so in the comment. A different delimiter (`|`) removes the fragility for free.

## Convention check

Conventional, imperative, correctly scoped commits (`feat(site)`, `chore(site)`, `chore(mise)`, `ci(pages)`, `docs`); `CHANGELOG.md` `[Unreleased] ### Added` updated with a Phase 10 entry above the Phase 9 one; ASCII-only and unwrapped throughout; no operator strings; `.gitignore` complete. Two plan deviations are documented in the implementer report (`mdx.format`, `markdown.hooks.onBrokenMarkdownLinks`) and are correct. One is not documented: D1 specified a `usage` shebang for `scripts/site-sync.sh` and both new scripts use plain `#!/usr/bin/env bash`. That is the right call -- neither takes arguments, `scripts/docs.sh` (the repo's other no-arg script) does the same, and the house rule reserves `usage` for argument-taking scripts -- but it should have been named as a deviation. Note also that "usage lint-clean" is vacuously true here: the gate has no `usage lint` step at all. Both new scripts are inside `shellcheck -S warning`'s `scripts/*.sh` glob and so are covered by the gate.
