# Phase 10 gate -- fix lane

Branch `phase-10/docs-site`, four checkpoint commits on top of `5b0504f`, applying `docs/phases/10/triage.md` F1-F14. Nothing under "Not applied" was touched. No push, no Pages enablement, no deploy. `mise run check` green on a clean tree after every checkpoint.

## Status

| F | Source | Status | Commit |
|---|---|---|---|
| F1 | A1, R6, S1 | done | `60f692c` |
| F2 | R1, R2 | done | `79a3547` |
| F3 | R3, A2 | done | `79a3547` |
| F4 | R4, A3 | done | `79a3547` |
| F5 | R9, A5 | done | `79a3547` |
| F6 | R5 | done | `79a3547` |
| F7 | R7, S4 | done | `e04a2bd` |
| F8 | R8 | done (landed in the F1 commit, see below) | `60f692c` |
| F9 | R10 | done | `e04a2bd` |
| F10 | R11, S2 | done | `e04a2bd` |
| F11 | S3 | done | `e04a2bd` |
| F12 | S6 | done, but its premise was wrong -- see below | checkpoint 4 (this commit) |
| F13 | S7 | done | checkpoint 4 (this commit) |
| F14 | A6 | done (covered by F1's note) | `60f692c` |

## F1 -- the blocker

### The policy is live

`site/pnpm-workspace.yaml` is now the single policy file, committed so it reaches CI as well as this machine:

```
minimumReleaseAge: 10080
minimumReleaseAgeStrict: true
strictDepBuilds: true
verifyStoreIntegrity: true
strictStorePkgContentCheck: true
trustPolicy: no-downgrade
trustPolicyExclude: [semver@6.3.1]
strictPeerDependencies: true
autoInstallPeers: false
blockExoticSubdeps: true
peerDependencyRules: {ignoreMissing: [search-insights]}
overrides: {fastq: 1.20.1}
allowBuilds: {core-js: true}
```

`onlyBuiltDependencies` is gone (R6/S1: inert, and its duplicate comment invited edits to the wrong key). One comment block, not two.

**Proof 1 -- effective config:**

```
$ pnpm --dir site config get minimumReleaseAge
10080
```

**Proof 2 -- the gate actually fires.** Scratch directory with nothing but a bare `package.json` and a copy of the repo's `site/pnpm-workspace.yaml`:

```
$ pnpm add --lockfile-only @types/node@26.4.1
[ERR_PNPM_NO_MATURE_MATCHING_VERSION] 1 version does not meet the minimumReleaseAge constraint:
  @types/node@26.4.1 was published at 2026-09-01T20:11:22.146Z, within the
  minimumReleaseAge cutoff (2026-08-28T01:58:39.193Z)
```

That is the same package the security lane used to prove the gate was *off*, now rejected.

### The re-resolve

`rm -rf site/node_modules site/pnpm-lock.yaml && pnpm --dir site install`. It failed loudly three times before it passed, which is the control working:

1. `[ERR_PNPM_TRUST_DOWNGRADE] semver@6.3.1`
2. `[ERR_PNPM_TRUST_DOWNGRADE] fastq@1.20.2`
3. `[ERR_PNPM_PEER_DEP_ISSUES] missing peer search-insights`

Each was resolved by pinning or by an exact-version exception, never by relaxing a policy. `minimumReleaseAge` was not touched.

**semver@6.3.1** -- excluded by exact version. Registry evidence: published 2023-07-10, the terminal release of the 6.x maintenance line, carrying registry signatures but no provenance attestation, while four 7.5.x releases (7.5.1 through 7.5.4, 2023-05-12 to 2023-07-07) published earlier that year do. A publish-date no-downgrade check therefore reads the older major as a downgrade. It is the only version satisfying `@babel/core`'s `semver@^6.3.1`, so there is nothing to pin to.

**fastq** -- pinned to 1.20.1, *not* excluded. This one is worth the lead's attention. The release-age gate pushes off 1.20.3 (2026-08-29, inside the window), and the newest mature release, **1.20.2 (2026-08-28), is the one release in that line with no trust evidence at all**:

| version | published | trust evidence |
|---|---|---|
| 1.19.1 | 2025-02-26 | none |
| 1.20.0 | 2025-12-23 | provenance + trusted publisher |
| 1.20.1 | 2025-12-23 | provenance + trusted publisher |
| **1.20.2** | **2026-08-28** | **none** |
| 1.20.3 | 2026-08-29 | provenance + trusted publisher |

A single release published outside the maintainer's normal signed pipeline, between two that were not, is the exact shape `trustPolicy` exists to catch. Excluding it would have been the wrong move, so the pin goes back to 1.20.1 (`@nodelib/fs.walk` asks for `^1.6.0`, so the pin is inside every declared range). Worth revisiting once 1.20.3 matures past the window.

**search-insights** -- one scoped `peerDependencyRules.ignoreMissing`, not `strictPeerDependencies: false`. `@algolia/autocomplete-plugin-algolia-insights` declares `search-insights` as a hard peer (`@docsearch/react` declares the same peer *optional*). Both reach the tree only through the `theme-search-algolia` that `preset-classic` bundles non-optionally, and `themeConfig.algolia` is never set, so the subtree is inert in the build. `search-insights` is Algolia click analytics, which D7 forbids installing. Declaring it missing on purpose is the honest resolution.

### The 18 versions, before and after

Cutoff at install time: **2026-08-28T02:02Z** (now minus 7 days).

| package | was | now | published (now) | age |
|---|---|---|---|---|
| `baseline-browser-mapping` | 2.11.20 | 2.11.20 | 2026-08-27T23:41:49Z | 7d 2h |
| `@jridgewell/sourcemap-codec` | 1.6.0 | 1.6.0 | 2026-08-28T01:14:47Z | 7d 0h |
| `fastq` | 1.20.3 | **1.20.1** | 2025-12-23T07:58:58Z | 254d |
| `cssdb` | 8.11.0 | 8.10.0 | 2026-08-15T15:05:55Z | 19d |
| `webpack` | 5.110.3 | 5.110.1 | 2026-08-27T20:04:50Z | 7d 5h |
| `@types/node` | 26.4.1 | 26.4.0 | 2026-08-27T00:15:56Z | 8d 1h |
| `@types/node` | 26.4.1 | 17.0.45 (2nd locked copy) | 2022-06-15T23:02:27Z | 1541d |
| `electron-to-chromium` | 1.5.420 | 1.5.415 | 2026-08-25T22:09:50Z | 9d 3h |
| `joi` | 17.13.7 | 17.13.6 | 2026-08-19T15:13:21Z | 15d |
| `fast-uri` | 3.1.7 | 3.1.6 | 2026-08-23T01:42:00Z | 12d |
| `@jsonjoy.com/fs-node-to-fsa` | 4.69.1 | 4.68.1 | 2026-08-10T13:04:22Z | 24d |
| `@jsonjoy.com/fs-node-utils` | 4.69.1 | 4.68.1 | 2026-08-10T13:07:28Z | 24d |
| `@jsonjoy.com/fs-print` | 4.69.1 | 4.68.1 | 2026-08-10T13:05:35Z | 24d |
| `@jsonjoy.com/fs-core` | 4.69.1 | 4.68.1 | 2026-08-10T13:03:04Z | 24d |
| `memfs` | 4.69.1 | 4.68.1 | 2026-08-10T13:07:45Z | 24d |
| `@jsonjoy.com/fs-node` | 4.69.1 | 4.68.1 | 2026-08-10T13:04:31Z | 24d |
| `@jsonjoy.com/fs-snapshot` | 4.69.1 | 4.68.1 | 2026-08-10T13:05:40Z | 24d |
| `@jsonjoy.com/fs-fsa` | 4.69.1 | 4.68.1 | 2026-08-10T13:03:13Z | 24d |
| `@jsonjoy.com/fs-node-builtins` | 4.69.1 | 4.68.1 | 2026-08-10T13:04:19Z | 24d |

Every one is now outside the window. Two of them (`baseline-browser-mapping`, `@jridgewell/sourcemap-codec`) kept their version simply because a day passed and they aged out; the other sixteen moved to an older release. The whole `@jsonjoy.com` burst -- nine sibling packages plus `memfs` published in an eight-minute window, the finding's headline -- is off the tree.

Not content with the 18, I swept **every** locked version independently against `registry.npmjs.org`:

```
locked name@version pairs: 1150   unique names: 1062
cutoff: 2026-08-28T02:03:21Z
fetch errors: 0
versions with no registry time entry: 0
versions younger than 7 days: 0
```

pnpm agrees: a later `pnpm --dir site install` prints `Lockfile passes supply-chain policies`.

### Lockfile settings block and package count

```yaml
lockfileVersion: '9.0'

settings:
  autoInstallPeers: false
  excludeLinksFromLockfile: false

overrides:
  fastq: 1.20.1
```

`autoInstallPeers: false`, which is the written record that the policy applied during this install (it read `true` before).

Package count: **1150** (1150 `packages:` entries, 1150 `integrity:` lines, 1150 `snapshots:` entries), down from 1151. The implementer report's 690 was wrong (A6/F14) and is corrected with a dated note.

### Documents corrected

`docs/phases/10/plan.md` -- a `> **STATE AS OF 2026-09-04**` callout after the Stage-1 paragraph (the machine-wide hardening was inert; `ERR_PNPM_IGNORED_BUILDS` was pnpm 11's own default, not the rc firing) and after D3 (the allow-list lives in `pnpm-workspace.yaml`; the two forced pins).

`docs/phases/10/reports/implementer.md` -- three dated callouts, at the package count (690 -> 1150), at the first-install error (misattributed to `strict-dep-builds` from the rc), and at "both clear the 7-day gate" (true of the four direct dependencies, false for the other ~1147). Plus the F8 entry under "Where reality disagreed".

## F12 -- applied, but the triage's premise was wrong

The simplification lane's out-of-lane observation said Prism has no `sh` language and no `sh` alias, so all 13 shell blocks rendered unhighlighted. **That is not true.** `prismjs@1.30.0`'s `components/prism-bash.js` ends with:

```js
Prism.languages.sh = Prism.languages.bash;
Prism.languages.shell = Prism.languages.bash;
```

`additionalLanguages: ['bash', ...]` loads that component, which registers the alias, so `sh` fences were already highlighted. Measured on the pre-F12 build of `site/build/getting-started.html`: the first shell block carried **16** `class="token ..."` spans.

F12 was applied anyway, because there is a real reason for it that is not the stated one: the fence should name the grammar the config actually asks for, rather than depend on an alias that a future `prismjs` could drop. It is not a fix for unhighlighted output, and the lead should know that "rendered output changes for the better" was decided on a false report.

**Evidence after F12** (`site/build/getting-started.html`):

```
code block languages: ['bash', 'bash', 'go', 'bash', 'bash', 'bash', 'json', 'bash']
token spans in the first bash block: 16
distinct token classes: ['assign-left', 'builtin', 'operator', 'plain']
remaining language-sh blocks: 0
```

```html
<pre tabindex="0" class="prism-code language-bash codeBlock_eq9Z thin-scrollbar" ...>
  <code class="codeBlockLines_m2FF"><div class="token-line" style="color:#bfc7d5">
  <span class="token builtin class-name" style="color:rgb(255, 203, 107)">export</span>
  <span class="token plain"> </span><span class="token assign-left ...
```

Token span count is identical before and after (16 and 16), which is the direct confirmation that `sh` was already being highlighted. `additionalLanguages` keeps `bash` (still required -- it is what loads the grammar) and drops `yaml`, for which no fence exists.

## Where else the triage was wrong

- **`packageManagerStrict` does not exist in pnpm 11.15.1.** F1 listed it; a grep of `pnpm.mjs` finds zero occurrences of it in any case form (compare `minimumReleaseAge`: 106, `trustPolicy`: 83, `blockExoticSubdeps`: 7). `pnpm config get packageManagerStrict` echoes back whatever the YAML says, because that command reads the file rather than the schema -- so it proves nothing on its own. The key was **omitted**: writing an inert setting into the policy file is exactly the mistake R6/S1 asked us to undo with `onlyBuiltDependencies`. The `packageManager` pin in `site/package.json` and the `pnpm` pin in `mise.toml` are unaffected and still agree at 11.15.1. `blockExoticSubdeps`, the key F1 flagged as uncertain, *is* accepted and is set.
- **F1 predicted only `strictPeerDependencies` conflicts.** The two that actually blocked the install first were `trustPolicy` downgrades, which no lane anticipated -- and one of them (`fastq@1.20.2`) is a finding in its own right.
- **F8 landed one checkpoint early**, in the F1 commit rather than the F7-F11 one, because it edits the same implementer-report section as F1's and F14's corrections. No content difference.

## Gate

`mise run check`, full run, clean tree, after the final checkpoint:

```
coverage-gate: <repo root>/freshbooks/coverage.out total = 91.9% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo root>/mcp/coverage.out total = 92.1% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo root>/cli/coverage.out total = 91.6% (floor 90%)
coverage-gate: PASS
...
== site-build ==
site-sync: regenerated site/docs/ (8 pages)
Already up to date
Done in 534ms using pnpm v11.15.1
$ docusaurus build
[INFO] [en] Creating an optimized production build...
[webpackbar] i Compiling Client
[webpackbar] i Compiling Server
[webpackbar] v Server: Compiled successfully in 1.46s
[webpackbar] v Client: Compiled successfully in 1.88s
[SUCCESS] Generated static files in "build".
site-build: OK
check.sh: all OK
```

`actionlint .github/workflows/*.yml`: exit 0, no output, after the F3/F4/F5/F6 edits. `shellcheck -S warning` clean on both changed scripts. `usage lint` was not run and does not apply: neither `scripts/site-sync.sh` nor `scripts/site-build.sh` takes arguments, and neither carries a `usage` shebang (the code-review lane confirmed that is the right call for this repo). `scripts/redaction-check.sh` clean before every commit; ASCII sweep clean on every file touched.

Sidebar re-verified after F11 removed `sidebar_position` -- the same seven entries in the same order: Getting started, Authentication, Library, MCP server, CLI reference, Building, Agentic transformation.

## For the lead, beyond this branch

The security lane's machine-level note stands and this branch cannot fix it: `~/.npmrc` and `~/.config/pnpm/rc` are dead for every pnpm 11 project on this machine, and the global `CLAUDE.md` still documents them as the policy surface. It also lists `package-manager-strict`, which pnpm 11.15.1 no longer has at all. Both are worth raising with the operator.
