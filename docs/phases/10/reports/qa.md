# Phase 10 gate -- QA / reality-check lane

Subject: branch `phase-10/docs-site` @ `b573526` (14 commits ahead of `main`; the last four are the F1-F14 review-gate fix). Read-only lane: no source modified, nothing committed, no push, no Pages enablement, no deploy. `<repo root>` = the freshbooks-tools working copy.

## Verdict: PASS

Every mandatory probe passed on evidence. Three findings, all **ADVISORY**; none blocks the merge.

## Probe 1 -- the gate

```
$ /usr/bin/time -f "GATE_WALL=%e" mise run check > /tmp/qa-gate.log 2>&1; echo $?
0
GATE_WALL=60.26
```

The three coverage lines:

```
coverage-gate: <repo root>/freshbooks/coverage.out total = 91.9% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo root>/mcp/coverage.out total = 92.1% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo root>/cli/coverage.out total = 91.6% (floor 90%)
coverage-gate: PASS
```

The `site-build` block:

```
== site-build ==
site-sync: regenerated site/docs/ (8 pages)
Already up to date
Done in 543ms using pnpm v11.15.1
$ docusaurus build
[INFO] [en] Creating an optimized production build...
[webpackbar] i Compiling Client
[webpackbar] i Compiling Server
[webpackbar] v Server: Compiled successfully in 1.50s
[webpackbar] v Client: Compiled successfully in 1.87s
[SUCCESS] Generated static files in "build".
site-build: OK
check.sh: all OK
```

Repo-wide steps run **once each** -- `grep -n "^== "` shows exactly one `actionlint`, `shellcheck`, `redaction-selftest`, `release-selftest`, `readme-drift-check`, `site-build` header. `git status --porcelain` after the run: empty (this report is written last and is the only dirty file).

`actionlint .github/workflows/*.yml` -> exit 0, no output.

`shellcheck -S warning scripts/*.sh` as literally specified exits 1, but only on SC1008 (the `#!/usr/bin/env -S usage bash` shebang shellcheck cannot parse) and SC2154 (`$usage_*` variables the `usage` runtime sets). Those two are excluded by the gate's own invocation with a comment saying why (`scripts/check.sh:89-93`). The gate's contract, `shellcheck -S warning -e SC1008,SC2154 scripts/*.sh`, exits **0** with no output. Not a finding.

## Probe 2 -- clean-clone build and link crawl

```
$ git clone <repo root> /tmp/qa-site && git -C /tmp/qa-site checkout phase-10/docs-site
b573526 fix(site): apply the review-gate findings (F12-F14)
$ mise trust /tmp/qa-site/mise.toml
$ cd /tmp/qa-site && /usr/bin/time -f "SITE_BUILD_WALL=%e" mise run site-build
Progress: resolved 1149, reused 1149, downloaded 0, added 1149, done
Done in 2.4s using pnpm v11.15.1
[webpackbar] v Server: Compiled successfully in 9.90s
[webpackbar] v Client: Compiled successfully in 17.88s
[SUCCESS] Generated static files in "build".
site-build: OK
SITE_BUILD_WALL=27.45
```

**27.45s, inside the 60s budget.** Cold webpack cache and cold `node_modules`, but a warm pnpm content-addressed store on this machine -- see Q2 for what CI actually pays.

Served with `pnpm --dir site exec docusaurus serve --no-open --port 3999` and crawled under `baseUrl` `/freshbooks-tools/`:

| page | status |
|---|---|
| `/freshbooks-tools/` (root) | 200 |
| `/freshbooks-tools/getting-started` | 200 |
| `/freshbooks-tools/authentication` | 200 |
| `/freshbooks-tools/library` | 200 |
| `/freshbooks-tools/mcp` | 200 |
| `/freshbooks-tools/cli` | 200 |
| `/freshbooks-tools/building` | 200 |
| `/freshbooks-tools/agentic-transformation` | 200 |
| `/freshbooks-tools/assets/css/styles.369b3b94.css` (only crawled internal href not already a seed) | 200 |

**9 URLs fetched, 0 non-200.** Every `href` starting with `/freshbooks-tools/` in those pages resolves to a seed page (navbar, sidebar and footer all point into the seven guides plus the home page), so the crawl closes after one hop. The rendered sidebar order matches `site/sidebars.js` exactly and in order: Getting started, Authentication, Library, MCP server, CLI reference, Building, Agentic transformation.

`site/build/` contains 9 HTML, 18 JS, 1 CSS, `sitemap.xml`, `.nojekyll`, 2 txt. `404.html` is generated.

**Negative test (the broken-link guard is real).** Appending `[nope](docs/does-not-exist.md)` to `docs/getting-started.md` in the clone:

```
broken_build_exit=1
Error: Markdown link with URL `docs/does-not-exist.md` in source file "docs/getting-started.md" (129:20) couldn't be resolved.
[site-build] ERROR task failed
```

The error names the **real** source path, not the generated `site/docs/` copy -- which is exactly what F2's design intent claimed. Restored, rebuild exit 0.

## Probe 3 -- supply-chain policy, live

```
$ pnpm --dir site config get minimumReleaseAge
10080
```

Gate fires. Scratch dir holding only a bare `package.json` and a copy of the repo's `site/pnpm-workspace.yaml`:

```
$ pnpm add --lockfile-only @types/node@26.4.1
[ERR_PNPM_NO_MATURE_MATCHING_VERSION] 1 version does not meet the minimumReleaseAge constraint:
  @types/node@26.4.1 was published at 2026-09-01T20:11:22.146Z, within the minimumReleaseAge cutoff (2026-08-28T02:28:17.004Z)
exit=1
```

Frozen install succeeds and prints the policy line:

```
$ pnpm --dir site install --frozen-lockfile
* Lockfile passes supply-chain policies (verified 27m ago)
Lockfile is up to date, resolution step is skipped
Progress: resolved 1149, reused 1149, downloaded 0, added 1149, done
Done in 2.3s using pnpm v11.15.1
```

Lockfile `settings:` block:

```yaml
lockfileVersion: '9.0'

settings:
  autoInstallPeers: false
  excludeLinksFromLockfile: false

overrides:
  fastq: 1.20.1
```

**Independent age sweep.** Lockfile commit is `60f692c`, `2026-09-03T22:06:45-04:00` = `2026-09-04T02:06:45Z`; the 7-day cutoff is therefore `2026-08-28T02:06:45Z`. Eleven named versions via `pnpm view <pkg>@<ver> time --json`:

| package@version | published | margin before the cutoff |
|---|---|---|
| `webpack@5.110.1` | 2026-08-27T20:04:50Z | 6h 2m (7.3d old) |
| `memfs@4.68.1` | 2026-08-10T13:07:45Z | 24.5d old |
| `fastq@1.20.1` | 2025-12-23T07:58:58Z | 254d old |
| `@types/node@26.4.0` | 2026-08-27T00:15:56Z | 8.1d old |
| `@types/node@17.0.45` (2nd locked copy) | 2022-06-15T23:02:27Z | 1542d old |
| `joi@17.13.6` | 2026-08-19T15:13:21Z | 15.5d old |
| `cssdb@8.10.0` | 2026-08-15T15:05:55Z | 19.5d old |
| `electron-to-chromium@1.5.415` | 2026-08-25T22:09:50Z | 9.2d old |
| `fast-uri@3.1.6` | 2026-08-23T01:42:00Z | 12.0d old |
| `baseline-browser-mapping@2.11.20` | 2026-08-27T23:41:49Z | 2h 25m (7.1d old) |
| `core-js@3.50.0` | 2026-08-05T08:54:45Z | 29.7d old |

A wider independent sweep (those 11 plus 60 randomly sampled locked pairs, seed 1010, fetched straight from `registry.npmjs.org`): **71 checked, 0 younger than 7 days, 0 fetch errors, 0 missing time entries.** The two tightest are `baseline-browser-mapping` and `webpack`, both aged out with hours, not days, to spare.

**Stronger still -- pnpm verified all 1150 itself.** pnpm caches its verdict in `~/.cache/pnpm/lockfile-verified.jsonl`, which is why the routine install prints `(verified 27m ago)` rather than re-checking. With that cache entry removed (and restored afterwards), a frozen install performs the full check:

```
? Verifying lockfile against supply-chain policies (1150 entries)...
* Lockfile passes supply-chain policies (1150 entries in 11.5s)
Done in 12.4s using pnpm v11.15.1
```

That is every locked entry, not a sample, validated live against the registry under `minimumReleaseAge` + `trustPolicy` + integrity. It also independently confirms the count.

**`fastq` is 1.20.1.** Why, in one line: 1.20.3 is inside the 7-day window and the newest mature release, 1.20.2, is the only release in that line carrying no trust evidence at all (1.20.0/1.20.1/1.20.3 all have provenance + a trusted publisher), so the fix pinned back to a signed release rather than adding a trust exclusion.

**Exotic specifiers: none.** All 1150 resolutions are registry tarballs:

```
$ grep -oE 'resolution: \{[a-z]+:' site/pnpm-lock.yaml | sort | uniq -c
   1150 resolution: {integrity:
$ grep -nE "specifier: (git\+|file:|link:|https?:)" site/pnpm-lock.yaml
NONE
```

The 52 `http` matches in the lockfile are all package **names** (`http-proxy`, `http2-wrapper`, `@types/http-errors`, `@algolia/requester-node-http`, ...); there is not a single `http(s)://` URL in the file. `resolution:` lines: **1150** -- matching 1150 `integrity:` lines and 1150 `snapshots:` entries, i.e. the fix report's count is right. (`pnpm install` reports `+1149` installed, one fewer than the lockfile holds; a cosmetic accounting difference, not a discrepancy in the file.)

**Cold-store install** (simulating a CI cache miss, `--store-dir /tmp/qa-store` empty): `resolved 1149, reused 0, downloaded 1149, added 1149, done` in 8.07s, exit 0, no `ERR_PNPM_TRUST_DOWNGRADE`. The lockfile is reproducible with no store history.

## Probe 4 -- built-site hygiene

Against the clean-clone `site/build`:

- **External `<script src=`: none.** The only two script tags are `/freshbooks-tools/assets/js/main.*.js` and `runtime~main.*.js`.
- **External `<link href=`: none loading a resource.** Every absolute `<link>` href is a self-referential canonical/alternate to `https://infiniteroomlabs.github.io/freshbooks-tools/...`.
- **`gtag|googletagmanager|algolia\.net|plausible|posthog`: zero matches** across `*.html` and `*.js`. Also zero for `fonts.googleapis`, `fonts.gstatic`, `cdn.jsdelivr`, `unpkg.com`.
- **No `<img>`, `<iframe>`, `<video>`, `<audio>`, `<source>` tags at all.** Every CSS `url()` is an inline `data:image/svg+xml` (the `www.w3.org` hits inside them are the SVG namespace, not a fetch).
- Build artefacts: 1 css, 9 html, 18 js, 1 `.nojekyll`, 2 txt, 1 xml. No font files, no images.
- `site/src/css/custom.css` declares a system font stack only, no `@import`, no `url(http`.

Distinct external hosts appearing anywhere in the built HTML, classified:

| host | occurrences | classification |
|---|---|---|
| `infiniteroomlabs.github.io` | 47 | self (canonical/alternate + the README's own site link) -- content |
| `github.com` | 41 | content link (repo, releases, edit-this-page) |
| `www.freshbooks.com` | 15 | content link (API docs) |
| `pkg.go.dev` | 10 | content link (navbar + prose) |
| `schema.org` | 7 | JSON-LD `@context` string, not fetched |
| `localhost` | 7 | prose/example URLs in the guides |
| `mcp.example.com` | 3 | prose/example URL in `docs/mcp.md` |
| `auth.freshbooks.com` | 3 | prose (OAuth endpoint) |
| `mise.jdx.dev` | 2 | content link |
| `docusaurus.io` | 2 | content link |
| `api.freshbooks.com` | 2 | prose (API endpoint) |
| `www.conventionalcommits.org` | 1 | content link |
| `documenter.gw.postman.com` | 1 | content link |

**Zero resource hosts.** D7 holds in the built artefact, not just in the config.

## Probe 5 -- F1-F14 verification

Spot-checked against `git diff 5b0504f..b573526` and the live tree.

| F | Verified how | Result |
|---|---|---|
| F1 | `site/pnpm-workspace.yaml` read in full; `pnpm config get minimumReleaseAge` -> `10080`; the `ERR_PNPM_NO_MATURE_MATCHING_VERSION` scratch probe; lockfile `settings:`; `onlyBuiltDependencies` and `packageManagerStrict` absent as keys (`onlyBuiltDependencies` survives only inside the comment explaining it is inert); `blockExoticSubdeps`, `trustPolicy: no-downgrade`, `trustPolicyExclude: [semver@6.3.1]`, `allowBuilds: {core-js: true}`, one comment block | **landed** |
| F2 | Ran `scripts/site-sync.sh` in a scratch tree seeded with a README containing every case. `(docs/cli.md#foo)` -> `(/cli#foo)`; `(docs/getting-started.md)` -> `(/getting-started)`; `(docs/progress.md)` and `(docs/progress.md#bar)` and `(docs/phases/10/plan.md)` left verbatim; inline `` `docs/cli.md` `` untouched | **landed** |
| F3 | `pages.yml:20-25` `concurrency: {group: pages, cancel-in-progress: false}` | **landed** |
| F4 | `pages.yml:6-14` paths include `scripts/site-build.sh`, `scripts/check.sh`, `mise.toml`, `.github/workflows/pages.yml` | **landed** |
| F5 | Store-cache steps in both `ci.yml:66-74` and `pages.yml:38-46`, path from `pnpm store path --silent`, key `pnpm-${{ hashFiles('site/pnpm-lock.yaml') }}`. Pin verified live: `gh api repos/actions/cache/git/ref/tags/v6.1.0` -> `55cc8345863c7cc4c66a329aec7e433d2d1c52a9` (commit), which is exactly what both files pin. `docs/building.md` carries the warm-vs-cold sentence | **landed** |
| F6 | `ci.yml:48-54` repo-wide comment lists `site-build` and explains why the job needs node/pnpm and the store cache | **landed** |
| F7 | `site/docusaurus.config.js` -- no `i18n`, no `docs.path`, no `colorMode.defaultMode`/`disableSwitch`, no `undefined` analytics keys; the `// No gtag/googleAnalytics keys: D7, no telemetry.` comment is present; `respectPrefersColorScheme`, `onBrokenLinks`, `markdown.hooks.onBrokenMarkdownLinks`, `trailingSlash`, `sidebarPath` all kept | **landed** |
| F8 | `docs/phases/10/reports/implementer.md:117` records dark mode as available and OS-following, no config change | **landed** |
| F9 | `mise.toml [tasks.site]` = `site-sync.sh && pnpm --dir site install --frozen-lockfile && pnpm --dir site start`; the `docs/building.md` block matches | **landed** |
| F10 | `scripts/site-sync.sh` `pages=()` table uses `\|`, three columns (name/title/source), no duplicate fifth | **landed** |
| F11 | `grep -rn sidebar_position site/docs/` -> no front-matter key anywhere (the single hit is prose in the synced `building.md` saying the script writes none). Rendered sidebar still the same 7 entries in the same order | **landed** (see Q1) |
| F12 | `grep -rc '^```sh$' site/docs/` -> none. Fence census across the synced pages: 13 `bash`, 10 `go`, 3 `json` -- exactly the three `additionalLanguages` and no `yaml` | **landed** |
| F13 | `scripts/site-build.sh` uses `mise exec -- pnpm --dir "$repo_root/site" install --frozen-lockfile` then `... build`, the same idiom as the mise task; verified by two successful builds | **landed** |
| F14 | `implementer.md:20` `> **STATE AS OF 2026-09-04**` corrects 690 -> 1151 first-committed / 1150 after the re-resolve. My independent count is 1150 | **landed** |

`scripts/redaction-check.sh` -> `redaction-check: clean`. `git grep` for operator strings (internal domains, `100.x` tailnet addresses, absolute home-directory paths) across `site/`, `scripts/site-*.sh`, `pages.yml` -> none.

## Probe 6 -- Pages readiness, by reading

- `pages.yml` triggers on push to `main` (plus `workflow_dispatch`), builds with `mise run site-build`, and uploads `path: site/build` via `actions/upload-pages-artifact@fc324d3...` -- the same directory the build actually produces.
- Permissions are correct and minimal: top-level `contents: read`; the `build` job inherits it (all `checkout` needs); the `deploy` job overrides with `pages: write` + `id-token: write` and `environment: github-pages`, which is what `actions/deploy-pages@368f8252...` requires.
- **No secrets.** `grep -n "secrets\." .github/workflows/pages.yml` -> no matches. Nothing in the workflow needs one.
- Every action is SHA-pinned with a version comment.
- `docs/building.md` "Docs site" matches reality: the three documented `mise run site-sync` / `site-build` / `site` commands match `mise.toml` and the scripts exactly, including the `--frozen-lockfile` flag and the `pnpm --dir site start` dev server; it correctly states that the site-build step joined `run_repo_wide` in `scripts/check.sh`; and it correctly warns that GitHub Pages itself must be enabled once out of band with `build_type=workflow`.
- `README.md:51` links the site: `- Rendered as a site: <https://infiniteroomlabs.github.io/freshbooks-tools/>`.
- Only 7 files under `site/` are tracked (`docusaurus.config.js`, `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, `sidebars.js`, `src/css/custom.css`, `static/.nojekyll`) -- the policy file and the lockfile do travel to CI, which is the whole point of F1. There is no `.npmrc` or `.pnpmfile.cjs` anywhere in the repo that could shadow them.

## Findings

### Q1 -- ADVISORY -- `site/sidebars.js:1-2`: comment still describes the `sidebar_position` front matter F11 deleted

**Expected:** after F11 removed `sidebar_position`, no file claims the sidebar order mirrors it.
**Observed:** `site/sidebars.js` opens with

```js
// Explicit order (mirrors each doc's front matter `sidebar_position`, set
// by scripts/site-sync.sh). ...
```

`sidebars.js` was not touched by any of the four fix commits (`git diff --stat 5b0504f..b573526` does not list it). The comment now contradicts both `scripts/site-sync.sh` ("a `sidebar_position` front matter key is inert next to an explicit sidebar, so this table deliberately does not carry one") and `docs/building.md` ("the sync script writes no `sidebar_position`"). Cosmetic -- the sidebar itself is correct and verified rendered -- but it is the one place a future editor would look to learn where order comes from, and it points at a key that no longer exists. One-line comment fix.

### Q2 -- ADVISORY -- the 60s site-build budget has less CI headroom than the local number suggests

**Expected:** the clean-clone 27.45s figure represents what CI pays.
**Observed:** it does not, on two counts, both measured:

1. My clean clone had a **warm pnpm store** (`reused 1149, downloaded 0`, install 2.4s). With a genuinely empty store the install is 8.07s. CI hits this whenever the `actions/cache` key misses.
2. pnpm caches its supply-chain verdict in `~/.cache/pnpm/lockfile-verified.jsonl`, which is why local installs print `(verified 27m ago)` in milliseconds. A CI runner has no such cache, so **every** CI run performs the full check: `Verifying lockfile against supply-chain policies (1150 entries)` -> `1150 entries in 11.5s`.

Cold-path CI estimate: sync + install (~8s store + ~11.5s verification) + cold webpack build (~24s) = **~44s**, roughly 73% of the 60s budget rather than the 46% the local figure implies. Still inside it, and `docs/building.md` already says the right thing directionally ("CI is never warm ... expect tens of seconds there even with the pnpm store cache ... the cache saves the download, not the build"). Worth recording that the per-run verification, not the download, is the part the cache cannot save -- and that it is a feature, since CI is where the policy gets re-proven against the live registry on every run.

### Q3 -- ADVISORY -- `.github/workflows/pages.yml:6-14`: `docs/**` over-triggers on paths the site never publishes

**Expected:** a Pages deploy when something the site renders changes.
**Observed:** `paths` includes `docs/**`, which covers `docs/phases/**`, `docs/progress.md` and `docs/superpowers/**` -- all three explicitly excluded from the sync (`scripts/site-sync.sh` header, `docs/building.md`). A commit to `main` that only adds a gate report under `docs/phases/` therefore runs a full build and republishes a byte-identical site. Cost and noise only, never wrong output, and the safe direction to err in. If the lead wants it tightened, `paths-ignore` cannot be combined with `paths` in one trigger -- it would need the seven guide paths listed explicitly instead of `docs/**`, which trades this over-trigger for the risk of forgetting a future guide. Recommend leaving as is and recording the reasoning.

## Cleanup

`/tmp/qa-site`, `/tmp/qa-store`, `/tmp/qa-policy`, `/tmp/qa-sync` and `/tmp/qa-cache` removed; the `docusaurus serve` process on port 3999 stopped (port confirmed closed). `~/.cache/pnpm/lockfile-verified.jsonl` was copied aside for the fresh-verification probe and restored byte-for-byte. Only log files remain under `/tmp/qa-*.log`. `git status --porcelain` in `<repo root>` is clean apart from this report.

## Commands run

```bash
/usr/bin/time -f "GATE_WALL=%e" mise run check > /tmp/qa-gate.log 2>&1; echo $?
actionlint .github/workflows/*.yml
shellcheck -S warning scripts/*.sh
shellcheck -S warning -e SC1008,SC2154 scripts/*.sh
scripts/redaction-check.sh
git clone <repo root> /tmp/qa-site && git -C /tmp/qa-site checkout phase-10/docs-site
mise trust /tmp/qa-site/mise.toml
cd /tmp/qa-site && /usr/bin/time -f "SITE_BUILD_WALL=%e" mise run site-build
mise exec -- pnpm --dir site exec docusaurus serve --no-open --port 3999
curl ... http://localhost:3999/freshbooks-tools/<page>          # + a python crawler over every /freshbooks-tools/ href
mise exec -- pnpm --dir site config get minimumReleaseAge
mise exec -- pnpm add --lockfile-only @types/node@26.4.1        # in a scratch dir holding only package.json + pnpm-workspace.yaml
mise exec -- pnpm --dir site install --frozen-lockfile
mise exec -- pnpm --dir site install --frozen-lockfile --store-dir /tmp/qa-store   # cold store
rm ~/.cache/pnpm/lockfile-verified.jsonl && mise exec -- pnpm --dir site install --frozen-lockfile   # forced full verification, cache restored after
mise exec -- pnpm view <pkg>@<ver> time --json                  # x11
uv run --no-project python /tmp/qa-age.py                       # 60 random + 11 named, straight from registry.npmjs.org
gh api repos/actions/cache/git/ref/tags/v6.1.0 --jq '.object.sha'
grep/find sweeps over site/build/**/*.{html,js,css}
bash /tmp/qa-sync/scripts/site-sync.sh                          # F2 scratch test
# negative test: append a broken link to docs/getting-started.md in the clone, rebuild, restore
```
