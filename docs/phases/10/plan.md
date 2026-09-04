# Work order: Phase 10 implementer (Docusaurus docs site)

Dispatch: `Agent(subagent_type: "general-purpose", model: "sonnet", name: "phase-10-impl")`. Unattended; no push, no Pages deploy by the implementer.

Stage 1 (lead, 2026-09-04): `main` @ `2ebd467` green, tree clean, `implemented 213, todo 0`. pnpm 11.15.1 (mise-managed) with the machine-wide hardening (`minimum-release-age=10080` strict, `block-exotic-subdeps`, `strict-dep-builds`, `verify-deps-before-run=error`, `prefer-frozen-lockfile`). `@docusaurus/core` latest is **3.10.2**, published 2026-07-10 (55 days old, clears the 7-day gate). GitHub Pages is not enabled on the repository yet (the lead enables it with `build_type=workflow` at ship). Node 24.14.0 locally.

> **STATE AS OF 2026-09-04** -- the "machine-wide hardening" above was inert. pnpm 11 reads its
> settings from `pnpm-workspace.yaml`, not from `.npmrc` / `~/.config/pnpm/rc`, so none of
> `minimum-release-age`, `block-exotic-subdeps`, `strict-dep-builds`, `verify-deps-before-run` or
> `prefer-frozen-lockfile` applied to this install (the `ERR_PNPM_IGNORED_BUILDS` on `core-js` that
> looked like the policy firing is pnpm 11's own default). The first lockfile therefore resolved 18
> versions younger than 7 days, 10 of them about 27 hours old. Gate finding A1; fixed by moving the
> whole policy into the committed `site/pnpm-workspace.yaml`, where it also reaches CI.

## Decisions

- **D1 -- layout.** The site lives in `site/` (a pnpm package, not a Go module; `go.work` untouched). `site/docs/` is generated, gitignored, and never hand-edited: `scripts/site-sync.sh` (`usage` shebang, `mise run site-sync`) copies `README.md` -> `site/docs/index.md` and `docs/{getting-started,authentication,library,mcp,cli,building,agentic-transformation}.md` -> `site/docs/<name>.md`, prepending front matter (`title`, `sidebar_position`, `slug`, and `format: md` so Docusaurus parses them as CommonMark, not MDX -- the guides contain `{`/`<` sequences and `docs/cli.md` is 8k generated lines). Links between guides are rewritten from `docs/<x>.md` / `(docs/...)` to the site slugs by the sync script; nothing in `docs/*.md` or `README.md` changes. `docs/phases/`, `docs/progress.md`, `docs/superpowers/` are NOT published.
- **D2 -- Docusaurus.** `@docusaurus/core` and `@docusaurus/preset-classic` pinned exactly at `3.10.2`, `react`/`react-dom` at the versions the preset's peer range wants (exact, and older than 7 days -- check `npm view <pkg> time` before pinning). Classic preset, docs-only mode (`routeBasePath: '/'`), dark mode on, no blog, no search plugin (no external service, no extra dependency in v1), `onBrokenLinks: 'throw'`, `onBrokenMarkdownLinks: 'throw'`, `trailingSlash: false`, `url: https://infiniteroomlabs.github.io`, `baseUrl: /freshbooks-tools/`, `organizationName`/`projectName` set, edit links to the GitHub source paths. Sidebar generated from the front matter positions: Getting started, Authentication, Library, MCP server, CLI reference, Building, Agentic transformation.
- **D3 -- install hardening.** First `pnpm install` will fail loudly under the machine policy; read the error. Native build scripts get allow-listed per package in `site/package.json` `pnpm.onlyBuiltDependencies` (expect none for Docusaurus 3; if one appears, name it and why). Never set `dangerously-allow-all-builds`, never relax `minimum-release-age`; if a transitive package is younger than 7 days, pick the previous Docusaurus patch and record it. `pnpm-lock.yaml` committed; `site/node_modules` and `site/build` gitignored; `packageManager` field pins pnpm `11.15.1`.

> **STATE AS OF 2026-09-04** -- D3 is superseded on two points. The build allow-list lives in
> `site/pnpm-workspace.yaml` `allowBuilds` (pnpm 11 ignores `package.json`'s `pnpm` field, and
> `onlyBuiltDependencies` is inert there); `core-js` is allow-listed after reading its postinstall.
> The release-age gate is carried by the same file rather than by the machine rc -- see the note
> above. `minimumReleaseAge` was never relaxed: the two dependencies it forced off their newest
> release are pinned instead (`fastq` to 1.20.1, and `semver@6.3.1` carries an exact-version
> `trustPolicyExclude`), both documented in that file.
- **D4 -- tasks and gate.** `mise.toml` `[tools]` gains `node = "24.14.0"` and `pnpm = "11.15.1"`; tasks `site-sync`, `site` (`pnpm --dir site start` after sync), `site-build` (sync then `pnpm --dir site install --frozen-lockfile && pnpm --dir site build`). The build joins `scripts/check.sh`'s repo-wide steps only if a warm build stays under 60s locally (measure it; report the number); otherwise it runs in CI only (D5).
- **D5 -- deploy.** `.github/workflows/pages.yml`: `on: push: branches: [main] paths: [docs/**, README.md, site/**, scripts/site-sync.sh]` plus `workflow_dispatch`; `permissions: contents: read` at the top; `build` job (checkout `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1`, `jdx/mise-action@c2a87611a18de5b3828c5652fe268e992400cb5c # v4.3.0` with `install: true`, `mise run site-build`, `actions/configure-pages@45bfe0192ca1faeb007ade9deae92b16b8254a0d # v6.0.0`, `actions/upload-pages-artifact@fc324d3547104276b827a68afc52ff2a11cc49c9 # v5.0.0` with `path: site/build`); `deploy` job (`needs: build`, `permissions: pages: write, id-token: write`, `environment: github-pages`, `actions/deploy-pages@368f82528645a54fb793d4d04e342629a3f51346 # v5.0.1`). `actionlint` must pass. The `ci.yml` jobs are untouched.
- **D6 -- docs.** `docs/building.md` gains a "Docs site" section (sync, dev server, build, how the Pages deploy triggers); `README.md` gets one line under Docs pointing at `https://infiniteroomlabs.github.io/freshbooks-tools/` (the lead confirms it renders at ship); root `CHANGELOG.md` `[Unreleased]` `### Added` line. `docs/cli.md` is never edited (generated; the drift test guards it).
- **D7 -- hygiene.** No telemetry: Docusaurus's own `reportWebVitals`/analytics off, no external scripts or fonts in the config (system font stack), no third-party CDN. No operator strings anywhere in `site/`. ASCII-only, unwrapped Markdown in anything you write.

## Rules

pnpm only (never npm/npx/yarn; `pnpm dlx` for one-offs). Stage and commit in separate Bash calls; `scripts/redaction-check.sh` before every commit; conventional commits (`feat(site): ...`, `chore(mise): ...`, `chore(ci): ...`, `docs: ...`); checkpoint commits per decision; `mise run check` green on a clean tree at the end (the gate must not become slower than 60s wall-clock for the site step if it is wired in). No push, no Pages enablement, no deploy. Context budget: never cat large files; build output to a log, read the tail.

## Reporting

`docs/phases/10/reports/implementer.md` (commit it), `SendMessage` to `team-lead` (full report in `message`), and the same as final text: the dependency tree summary (`pnpm list --depth 0`, the count of packages in the lockfile, any allow-listed build script and why), the first-install error and how it was resolved, the warm and cold build times, the sidebar as rendered (page list), the `pnpm --dir site build` tail, the gate tail with the three `coverage-gate:` lines, `git log --oneline main..phase-10/docs-site`, `git status --porcelain`, and anything where reality disagreed with D1-D7.
