# Phase 10 gate -- simplification lane

Branch `phase-10/docs-site`, 9 commits `d73e122..eb25cc7`, reviewed against `git diff main...phase-10/docs-site`. Propose-only: nothing in this report was applied, no build, install, gate, or network call was run. Every claim below was verified by reading the pinned `@docusaurus/*@3.10.2` and `pnpm@11.15.1` sources already present on disk, the committed `site/build/` output, and the source guides.

Headline: this is a tight diff. There are **no scaffolder leftovers** -- no `site/src/pages/`, no sample images, no blog remnants, no `babel.config.js`, and `site/src/css/custom.css` is 27 hand-written lines (system font stack plus a two-mode primary palette), not the generated boilerplate. The findings below are about redundant/inert config and one duplicated data field, not about dead scaffolding.

Net if S1-S4 are applied: about 30 lines removed, one silently-ignored second source of truth for sidebar order eliminated, and one confirmed-inert pnpm setting removed. All four are byte-identical in rendered output.

---

## S1 -- `site/pnpm-workspace.yaml`: drop the inert `onlyBuiltDependencies` key and the duplicated core-js justification -- APPLY-RECOMMENDED

`site/pnpm-workspace.yaml:1-18`

The file carries the same core-js postinstall justification twice (lines 1-4 and lines 11-16, near-identical prose) and two build-allow-list settings where only one is live.

Before (18 lines):

```yaml
# core-js's postinstall (node_modules/core-js/postinstall.js) was read
# before setting this to true: it only prints a donation banner (checks
# ADBLOCK/CI/DISABLE_OPENCOLLECTIVE env vars and a /tmp rate-limit file,
# then console.log) -- no compilation, no native binary, no network call.
allowBuilds:
  core-js: true
# pnpm 11 moved settings like onlyBuiltDependencies out of package.json's
# "pnpm" field and into this file (see https://pnpm.io/settings). site/ is
# a single package, not a multi-package workspace -- no `packages:` list
# needed, this file exists purely to carry that setting.
#
# core-js is a transitive dep of @docusaurus/core (via
# @docusaurus/plugin-content-blog, a babel polyfill target). Its
# postinstall (node_modules/core-js/postinstall.js) only prints a
# donation banner -- no compilation, no native binary -- verified by
# reading the script before allow-listing it.
onlyBuiltDependencies:
  - core-js
```

After (about 9 lines):

```yaml
# site/ is a single package, not a multi-package workspace -- this file
# exists purely to carry pnpm 11's `allowBuilds` setting (pnpm 11 reads
# build allow-lists here, not from package.json's "pnpm" field).
#
# core-js is a transitive dep of @docusaurus/core (via
# @docusaurus/plugin-content-blog, a babel polyfill target). Its
# postinstall (node_modules/core-js/postinstall.js) was read before
# allow-listing: it only prints a donation banner (checks
# ADBLOCK/CI/DISABLE_OPENCOLLECTIVE and a /tmp rate-limit file, then
# console.log) -- no compilation, no native binary, no network call.
allowBuilds:
  core-js: true
```

Why behaviour-preserving: in `pnpm@11.15.1` (`~/.local/share/mise/installs/pnpm/11.15.1/dist/pnpm.mjs`) `createAllowBuildFunction()` reads only `opts.allowBuilds` and `opts.dangerouslyAllowAllBuilds`. `onlyBuiltDependencies` occurs exactly three times in that bundle: twice in the list of setting *names* accepted in `pnpm-workspace.yaml` (so it parses without error) and once inside an unrelated read-only-store error hint. There is no code path that folds it into `allowBuilds`. The implementer report says the same thing from the other direction: "`onlyBuiltDependencies` wasn't what unblocked it -- pnpm auto-wrote an `allowBuilds` stub ... that needed an explicit `true`". Keeping a setting that was empirically proven not to work invites the next reader to edit the wrong one.

Risk: very low. The pin is exact in two places (`site/package.json` `packageManager`, `mise.toml [tools]`), and the gate itself verifies the result -- `run_site_build` runs `pnpm install --frozen-lockfile` under the machine-wide `strict-dep-builds=true`, which fails loudly with `ERR_PNPM_IGNORED_BUILDS` if the allow-list stops working. No separate verification step needed beyond re-running `mise run check`.

Counter-argument if the lead prefers to keep it: it is harmless belt-and-braces should pnpm ever restore `onlyBuiltDependencies` as the primary. If kept, at minimum collapse the duplicated comment.

## S2 -- `scripts/site-sync.sh`: the `pages` table carries the same path twice -- APPLY-RECOMMENDED

`scripts/site-sync.sh:28-41`

The table is `name:title:position:source-path:edit-path`, and in all eight rows `source-path` and `edit-path` are byte-identical (`README.md:README.md`, `docs/library.md:docs/library.md`, ...). The fifth field carries no information.

Before:

```bash
# name:title:position:source-path:edit-path
pages=(
  "index:freshbooks-tools:0:README.md:README.md"
  "getting-started:Getting started:1:docs/getting-started.md:docs/getting-started.md"
  ...
)
...
  IFS=':' read -r name title position src edit_path <<<"$page"
...
    echo "custom_edit_url: \"$edit_base/$edit_path\""
```

After:

```bash
# name:title:source-path (also the path the "Edit this page" link points at)
pages=(
  "index:freshbooks-tools:README.md"
  "getting-started:Getting started:docs/getting-started.md"
  ...
)
...
  IFS=':' read -r name title src <<<"$page"
...
    echo "custom_edit_url: \"$edit_base/$src\""
```

(The `position` field drops out too -- see S3. If S3 is rejected, keep `position` and drop only `edit_path`.)

Why behaviour-preserving: pure substitution of one variable for another whose value is identical in every row. Verified by reading all eight rows.

Risk: very low. If a future page ever needed a divergent edit path, the fifth field can come back at that point -- speculative generality is what it is today.

## S3 -- `scripts/site-sync.sh`: `sidebar_position` front matter is inert with an explicit sidebar -- APPLY-RECOMMENDED

`scripts/site-sync.sh:29-38, 51-52`, and the sentence in `docs/building.md:77` that documents it.

Sidebar order is declared twice: as `sidebar_position` in every synced page's front matter, and as the ordered array in `site/sidebars.js`. Only the second one does anything.

Why behaviour-preserving: in `@docusaurus/plugin-content-docs@3.10.2`, `sidebar_position` is read in `lib/docs.js:67` into `sidebarPosition`, which is then consumed only by `lib/sidebars/generator.js` (the *autogenerated* sidebar generator, lines 94 and 142) after being picked into the generator's doc shape in `lib/sidebars/processor.js:17-24`. `site/sidebars.js` is a hand-written array of doc ids, so `sidebarItemsGenerator` is never invoked for it and `sidebarPosition` is never read. Front matter keys do not render, so removing it cannot change a byte of `site/build/`. Next/previous pagination follows the sidebar array, not the front matter, so that is unaffected too.

Why it is worth removing rather than keeping as documentation: it is a trap. Someone reordering the guides will edit the obvious-looking `sidebar_position` values in `site-sync.sh`, run the build, see no change, and have nothing tell them why. One source of truth (`site/sidebars.js`, which already carries a comment explaining the ordering) is better than one real plus one decorative.

Counter-argument: it keeps a future switch to an autogenerated sidebar cheap. That switch is blocked anyway (see S10), and the positions could be re-derived from the `pages` array order in minutes.

Risk: low. Verify with `mise run site-build` and a diff of `site/build/getting-started.html`'s `menu__link` list against the seven entries in the implementer report.

## S4 -- `site/docusaurus.config.js`: five settings are already the framework default -- APPLY-RECOMMENDED

`site/docusaurus.config.js:18-21, 38, 50-51, 59-63`

| Setting | Line | Default in 3.10.2 | Source |
|---|---|---|---|
| `i18n: {defaultLocale: 'en', locales: ['en']}` | 18-21 | identical (`DEFAULT_I18N_CONFIG`) | `@docusaurus/core/lib/server/configValidation.js:38-43` |
| `docs.path: 'docs'` | 38 | `'docs'` | `@docusaurus/plugin-content-docs/lib/options.js:18` |
| `gtag: undefined` | 50 | omitting the key is the same value | -- |
| `googleAnalytics: undefined` | 51 | omitting the key is the same value | -- |
| `colorMode.defaultMode: 'light'` | 60 | `'light'` | `@docusaurus/theme-classic/lib/options.js:41-45` |
| `colorMode.disableSwitch: false` | 61 | `false` | same |

`colorMode.respectPrefersColorScheme: true` is **not** the default (`false`) and must stay -- it is what D2's "dark mode on" actually rests on.

Before:

```js
  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },
  ...
        docs: {
          routeBasePath: '/',
          path: 'docs',
          sidebarPath: require.resolve('./sidebars.js'),
  ...
        gtag: undefined,
        googleAnalytics: undefined,
  ...
      colorMode: {
        defaultMode: 'light',
        disableSwitch: false,
        respectPrefersColorScheme: true,
      },
```

After:

```js
        docs: {
          routeBasePath: '/',
          sidebarPath: require.resolve('./sidebars.js'),
  ...
      colorMode: {
        // Not the default (false): follow the visitor's OS preference.
        respectPrefersColorScheme: true,
      },
```

Note on the two that stay even though they look removable: `onBrokenLinks: 'throw'` (line 16) *is* the 3.10.2 default, but the gate leans on it (`scripts/check.sh:129-135`), and a future Docusaurus default flip would silently weaken `mise run check` -- keep it explicit. `markdown.hooks.onBrokenMarkdownLinks: 'throw'` is genuinely load-bearing (the default is `'warn'`, `configValidation.js:93-96`). `trailingSlash: false` is load-bearing (there is no default; `undefined` means legacy behaviour). `sidebarPath` has no default at all (`options.js:76` -- absent means autogenerate), so it is required.

On `gtag`/`googleAnalytics: undefined`: these document D7's "no telemetry" intent, which is worth keeping *as a comment*. Suggest replacing both lines with `// No gtag/googleAnalytics keys: D7, no telemetry.` rather than deleting silently.

Why behaviour-preserving: each removed key is replaced by the identical value from the validation schema's own default. Verified against the installed package sources, not from memory.

Risk: low. `mise run site-build` plus a `diff -r` of `site/build/` before and after is a complete check.

## S5 -- `site/docusaurus.config.js`: the preset `editUrl` fallback is dead, and wrong if it ever fired -- OPTIONAL

`site/docusaurus.config.js:40-44`

Five lines (a four-line comment plus the key) exist to explain a fallback that can never be reached: `scripts/site-sync.sh` regenerates `site/docs/` from scratch on every run and writes `custom_edit_url` into all eight pages, and the built output confirms it -- every `edit/main/...` href in `site/build/*.html` points at `README.md` or a real `docs/*.md`, none at `site/docs/`. The comment itself explains that the fallback would produce a link into the gitignored generated copy, i.e. a wrong link.

After: delete lines 40-44 and keep a one-liner where the docs block is, e.g. `// Edit links come from each page's custom_edit_url front matter (site-sync.sh).`

Why behaviour-preserving: a preset-level `editUrl` only applies to docs without `custom_edit_url`; there are none, and there structurally cannot be any while `site/docs/` is generated. Confirmed against the eight distinct hrefs in the committed build.

Risk: low, but this is judgement, not defect -- a reader could reasonably prefer a fallback even a wrong one. Tagged OPTIONAL for that reason.

## S6 -- `site/docusaurus.config.js`: `additionalLanguages` lists languages the site cannot use -- OPTIONAL

`site/docusaurus.config.js:99-101`

`additionalLanguages: ['go', 'bash', 'yaml', 'json']`, but the published sources contain exactly three fence languages: 13 x ```` ```sh ````, 10 x ```` ```go ````, 3 x ```` ```json ````. There is no ```` ```bash ```` and no ```` ```yaml ```` anywhere in `README.md` or the seven synced guides.

Narrow proposal (zero risk): drop `'bash'` and `'yaml'`.

```js
      prism: {
        additionalLanguages: ['go', 'json'],
      },
```

Why behaviour-preserving: a Prism grammar that no code fence references cannot affect any rendered token. It does affect the client bundle -- the bash grammar is currently compiled into `site/build/assets/js/main.*.js` -- so this is a small bundle win with no output change.

Wider proposal (mentioned, not recommended): `'go'`, `'json'` and `'yaml'` are already in `prism-react-renderer@2.4.1`'s bundled grammar set (verified by enumerating `Prism.languages.*` assignments in its `dist/index.js`: ... go ... json ... yaml ...), so the whole key could go. I am **not** recommending that, because the `prismjs@1.30.0` component and the prism-react-renderer bundled copy of the same grammar are not guaranteed token-for-token identical, and a token-class difference would change rendered HTML -- exactly what this lane is not allowed to risk. If the lead wants it, gate it on a `diff -r site/build/` before and after.

Risk: negligible for the narrow version, non-zero for the wide one.

## S7 -- `scripts/site-build.sh`: two `cd` subshells where `--dir` already does the job -- OPTIONAL

`scripts/site-build.sh:14-15`

Before:

```bash
"$repo_root/scripts/site-sync.sh"
(cd "$repo_root/site" && mise exec -- pnpm install --frozen-lockfile)
(cd "$repo_root/site" && mise exec -- pnpm build)
```

After:

```bash
"$repo_root/scripts/site-sync.sh"
mise exec -- pnpm --dir "$repo_root/site" install --frozen-lockfile
mise exec -- pnpm --dir "$repo_root/site" build
```

Why behaviour-preserving: `pnpm --dir` sets the working directory for the install and for the lifecycle script it runs. `mise.toml`'s `[tasks.site]` (line 58) already uses exactly this form -- `mise exec -- pnpm --dir site start` -- so the phase ships both idioms for the same operation, in two files, three lines apart in review. Picking one is a readability win.

Risk: low but not zero, because `--dir` and `cd` are only *usually* interchangeable for lifecycle scripts. Verify with `mise run site-build` and confirm `site/build/` is regenerated. If the lead would rather not touch a green build script, the equally good fix is the reverse: change `[tasks.site]` to `scripts/site-sync.sh && (cd site && mise exec -- pnpm start)`. Either direction, pick one.

## S8 -- `scripts/site-sync.sh`: seven `echo` lines that are one heredoc -- OPTIONAL

`scripts/site-sync.sh:48-60`

Before:

```bash
  {
    echo "---"
    echo "title: \"$title\""
    echo "sidebar_position: $position"
    echo "slug: \"$slug\""
    echo "mdx:"
    echo "  format: md"
    echo "custom_edit_url: \"$edit_base/$edit_path\""
    echo "---"
    echo
    sed -E 's#\(docs/([A-Za-z0-9_-]+)\.md\)#(/\1)#g' "$repo_root/$src"
  } >"$out"
```

After (with S2 and S3 applied):

```bash
  {
    cat <<-FRONTMATTER
	---
	title: "$title"
	slug: "$slug"
	mdx:
	  format: md
	custom_edit_url: "$edit_base/$src"
	---

	FRONTMATTER
    sed -E 's#\(docs/([A-Za-z0-9_-]+)\.md\)#(/\1)#g' "$repo_root/$src"
  } >"$out"
```

Why behaviour-preserving: identical bytes on stdout; the front matter block becomes readable as the YAML it is. `scripts/release-selftest.sh` already uses heredocs, so this is house style.

Risk: low, but tab-indented `<<-` heredocs are fiddly under an editor that converts tabs. A plain unindented `<<FRONTMATTER` at column zero avoids that entirely and is what I would actually write. Verify with `mise run site-sync && head -10 site/docs/library.md`. Small payoff -- fine to skip.

The `sed -E` link rewrite (line 59) is already a single pass over a single pattern; nothing to collapse there.

## S9 -- `docs/building.md`: the `ForceTerminatePlugin` note is a paragraph where two sentences do -- OPTIONAL

`docs/building.md:89`

The note is real, hard-won, and non-obvious (the implementer lost time to it), so it should stay -- but roughly two thirds of its length is the diagnosis narrative rather than the fix. It currently runs about 130 words inside a section that is otherwise operational reference.

Suggested compression, keeping every actionable fact:

> If `pnpm --dir site build` fails and the log shows only "Client bundle compiled with errors therefore further build is impossible.", the real error was lost: `@docusaurus/core`'s `ForceTerminatePlugin` calls `process.exit(1)` synchronously right after logging it, and Node can drop the tail of a non-TTY stdout write first (`script` does not help). Re-run interactively without redirecting output, or with `NODE_OPTIONS="--require <script that delays process.exit by two setImmediate ticks>"`.

That is the symptom, the cause, and both workarounds in two sentences, about half the length.

Risk: none (prose only). Purely the lead's call on how much war story belongs in a build guide.

---

## Considered and rejected

- **S10 -- autogenerate the sidebar from the front matter positions. DO-NOT-APPLY.** The prompt asked only if the output would be identical; it would not. `sidebarItemsGenerator` walks every doc in the docs dir, so an autogenerated sidebar would include `index` (the synced `README.md` home page), which `site/sidebars.js:1-4` deliberately excludes. Docusaurus 3.10.2 has no front matter key that hides a doc from an autogenerated sidebar. Rejected -- it changes the rendered sidebar.
- **S11 -- drop `site/static/.nojekyll` as a no-op under `actions/deploy-pages`. DO-NOT-APPLY.** It is load-bearing for a reason unrelated to Jekyll: the implementer report records that an empty `site/static/` breaks the build, because webpack's CopyPlugin errors when its `site/static/**/*` glob matches zero files. It is also the only source of the `.nojekyll` in `site/build/` (grep finds no `.nojekyll` emitter in `@docusaurus/core`). Removing it breaks the build.
- **S12 -- drop `onBrokenLinks: 'throw'` as a default. DO-NOT-APPLY.** It is the 3.10.2 default, but the gate's value proposition rests on it. See the note under S4.
- **S13 -- drop the job-level `name:` keys in `.github/workflows/pages.yml:18,33`.** They duplicate the job ids, but `.github/workflows/ci.yml:13-54` does the same for all four jobs. DO-NOT-APPLY: consistency with the existing workflow beats two saved lines. Same verdict on `cache: true` at `pages.yml:25` -- it is very likely `jdx/mise-action@v4`'s default, but I could not verify that offline and the payoff is one line.
- **S14 -- prune `site/package.json`'s unused `docusaurus`, `clear`, `serve` scripts (only `start` and `build` are called).** DO-NOT-APPLY: they are the conventional Docusaurus set, cost nothing, and `docs/building.md:87` already references `docusaurus clear` in a debugging tip.
- **`scripts/check.sh` / `mise.toml` duplication.** Every gate step in this repo exists as both a `check.sh` subcommand and a `[tasks.*]` entry; `site-build` follows that pattern exactly (`check.sh:129-135, 169, 203`; `mise.toml:57-63`). Not duplication to remove -- it is the established shape. Same for the header comment in `site-sync.sh` overlapping `docs/building.md`'s prose: a script explaining itself and a guide explaining it to operators are different audiences.

## One observation outside this lane's remit

All 13 shell code fences in the published guides use ```` ```sh ````. Prism has no `sh` language and no `sh` alias (`prism-bash` registers `shell`, not `sh`), so every shell block on the site renders unhighlighted regardless of what `additionalLanguages` contains. Fixing it means editing `docs/*.md` fences, which changes the rendered site -- out of scope here, and flagged for the lead rather than proposed.
