# Phase 10 gate triage (lead, 2026-09-04)

Inputs: `docs/phases/10/reports/{implementer,code-review,simplify,security}.md`. Verdicts: code review **APPROVE** (R1-R11 advisory), simplification S1-S4 apply, S5-S9 optional, S10-S14 rejected, security **BLOCK** (A1 blocking, A2-A6 advisory). One opus fix agent, four checkpoint commits `fix(site): apply the review-gate findings (Fa-Fb)`.

## The blocker

**A1.** pnpm 11 reads pnpm-specific settings from `pnpm-workspace.yaml`, not from `.npmrc`/`~/.config/pnpm/rc`. The machine policy therefore never applied; the plan and the implementer report both asserted it did. Eighteen locked versions are inside the 7-day quarantine. The fix carries the policy with the repo. This is also a machine-level finding for the operator (every pnpm 11 project on the machine), recorded in the lead's memory and to be raised with Wes; not fixable from this repo.

## Fix order

| F | Source | Action |
|---|---|---|
| F1 | A1, R6, S1 | `site/pnpm-workspace.yaml` becomes the single policy file: `minimumReleaseAge: 10080`, `minimumReleaseAgeStrict: true`, `strictDepBuilds: true`, `verifyStoreIntegrity: true`, `strictStorePkgContentCheck: true`, `trustPolicy: no-downgrade`, `strictPeerDependencies: true`, `autoInstallPeers: false`, `packageManagerStrict: true`, `blockExoticSubdeps: true` (if pnpm 11 accepts the key; else record), plus `allowBuilds: { core-js: true }` with ONE comment; drop `onlyBuiltDependencies`. Then `rm -rf site/node_modules site/pnpm-lock.yaml && pnpm --dir site install`: every one of the 18 young versions must resolve to a mature one or the install must fail loudly; if Docusaurus 3.10.2's ranges cannot be satisfied inside the window, pin the previous Docusaurus patch and record it. Never relax `minimumReleaseAge`. Confirm `pnpm --dir site config get minimumReleaseAge` prints `10080` and the lockfile `settings:` block shows `autoInstallPeers: false`. Correct `docs/phases/10/plan.md` lines 5 and 11 and `docs/phases/10/reports/implementer.md` lines 18, 24, 29 with a dated `> **STATE AS OF 2026-09-04**` note (the policy was inert; the count is 1151 packages, not 690). `strictPeerDependencies` may surface peer conflicts: resolve them by pinning, never by turning it off. |
| F2 | R1, R2 | `scripts/site-sync.sh` link rewrite: anchors preserved (`(docs/<x>.md#frag)` -> `(/<x>#frag)`), and the alternation restricted to the seven published names; a `docs/<other>.md` link is left as is so `onBrokenMarkdownLinks: 'throw'` names the real source. |
| F3 | R3, A2 | `pages.yml`: `concurrency: { group: pages, cancel-in-progress: false }`. |
| F4 | R4, A3 | `pages.yml` `paths`: add `scripts/site-build.sh`, `scripts/check.sh`, `mise.toml`, `.github/workflows/pages.yml`. |
| F5 | R9, A5 | pnpm store cache in `ci.yml`'s `repo-wide` job and in `pages.yml`'s build job: `actions/cache` pinned by SHA (resolve the current release with `gh api`), path `~/.local/share/pnpm/store` (or `pnpm store path`), key `pnpm-${{ hashFiles('site/pnpm-lock.yaml') }}`. `docs/building.md` distinguishes warm-local from cold-CI build times. |
| F6 | R5 | `ci.yml` repo-wide comment lists `site-build`. |
| F7 | R7, S4 | `site/docusaurus.config.js`: remove `i18n`, `docs.path`, `colorMode.defaultMode`, `colorMode.disableSwitch`, the two `undefined` analytics keys (replace with a one-line "no gtag/googleAnalytics keys: D7" comment); keep `respectPrefersColorScheme`, `onBrokenLinks`, `markdown.hooks.onBrokenMarkdownLinks`, `trailingSlash`, `sidebarPath`. |
| F8 | R8 | Record in `docs/phases/10/reports/implementer.md` "reality disagreed": dark mode means available and following the OS preference, not forced; no config change. |
| F9 | R10 | `mise.toml` `[tasks.site]` installs (`pnpm --dir site install --frozen-lockfile`) before `start`; `docs/building.md` block order matches. |
| F10 | R11, S2 | `scripts/site-sync.sh` table: `|` delimiter, drop the duplicate fifth column. |
| F11 | S3 | Drop the inert `sidebar_position` front matter and the `docs/building.md` sentence about it; `site/sidebars.js` stays the single order. |
| F12 | simplify out-of-lane, S6 | `scripts/site-sync.sh` rewrites fenced ```` ```sh ```` to ```` ```bash ```` on copy (Prism has no `sh` alias; the guides are not edited); `additionalLanguages` keeps `bash`, drops `yaml`. Lead decision: rendered output changes for the better. |
| F13 | S7 | `scripts/site-build.sh` uses `pnpm --dir site ...` like the mise task (one idiom); verify with a build. |
| F14 | A6 | Package count in the implementer report corrected (covered by F1's note). |

Checkpoints: F1 (alone: the lockfile re-resolve is the risky one; full gate after), F2-F6, F7-F11, F12-F14. Full gate after each.

## Not applied

- **A4** (four known advisories, none reachable from the published site; no audit step): recorded as backlog item 18 -- a weekly non-blocking `pnpm audit --prod --audit-level high` workflow; not a gate step because `image-size` has no fixed version.
- **S5, S8, S9**: optional prose/config trims below the threshold; S9's note stays as the reviewer found it valuable.
- **S10-S14**: rejected by the lane with reasons the lead agrees with.

## Lane-vs-lane

R3 = A2, R4 = A3, R9 = A5, R6 = S1, R11 = S2. Security's A1 is the finding no other lane saw and the only one that reaches beyond this repository.
