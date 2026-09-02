# Phase 5 gate triage (lead)

Inputs: `docs/phases/5/reports/{implementer,code-review,simplify,security}.md`. Verdicts: code review **REQUEST CHANGES** (R1 blocking, R2-R3 advisory), simplification 3 apply-recommended / 4 optional / 9 rejected, security **PASS** (A1-A5 advisory). The fix list is eleven small text and test edits, so the lead landed them in the main loop (Phase 4 lesson: one-line fixes cost less in the lead's hands than in another agent round) as one commit, `fix(release): apply the review-gate findings`.

## Applied

| Id | Finding | Action |
|---|---|---|
| R1 | Five files claim CycloneDX SBOMs; goreleaser's syft default is `spdx-json` (`dist/config.yaml` and the generated documents prove it) | Wording corrected to "SPDX 2.3 JSON SBOM" in `README.md`, `docs/building.md`, root and `mcp`/`cli` changelogs. Config unchanged. |
| R2 | `docs/library.md` still forward-references "Phase 2" | Sentence rewritten without the phase reference; caveat kept (business-family search spelling is docs-only). |
| R3 | `NewRootCmd` doc comment lists `docs` unconditionally | Comment names the `extraCommands` hook and the `docsgen` tag. |
| A2 | `contents: write` job checks out with persisted credentials | `persist-credentials: false` on the release job's checkout. |
| A4 | Getting-started HTTP example binds `0.0.0.0` | `127.0.0.1:8080` plus the plain-HTTP/TLS-proxy warning inline. |
| A5 | Unmatched asset glob would reach `gh release create` literally | `shopt -s failglob` in the publish step. |
| S1 | Hand-rolled containment loop | `slices.Contains`. |
| S2 | Four copy-pasted `resolveVersion` subtests per binary | Table-driven in both files, subtest names unchanged, files kept identical. |
| S3 | Four inline `${{ }}` interpolations in the publish step | `working-directory` plus relative globs (matches the goreleaser step above it). |
| S5 | `Page[T]` introduced twice in the lib changelog | Cross-reference in the Types bullet. |
| S6 | Gate paragraph in `docs/building.md` too long | The dirty-tree sentence split into its own paragraph. |

## Not applied (with reason)

- **A1** (no tag-creation ruleset; guard accepts any ancestor of `main`; `enforce_admins: false`): repository settings, not repository content. Added to the attended tag runbook in `docs/progress.md` as a pre-flight step (`scripts/branch-protection.sh` grows the ruleset in that step).
- **A3** (`usage` is unverified by mise and interprets `changelog-section.sh`): pre-existing; the repo's shell-script convention is `usage`. Recorded in the backlog; revisit if a checksum-verifiable `usage` pin appears.
- **S4** (table-drive five `strings.Contains` checks): below the threshold where a table helps.
- **S7** (three "once v0.1.0 tags ship" caveats go stale together): added to the tag runbook as a grep step.
- **S8-S16**: rejected by the lane itself with reasons recorded in `reports/simplify.md`; the lead agrees with each.

## Lane-vs-lane conflicts

None. The implementer's three reported deviations (D3 folded into F1, the `:(exclude)docs/phases/*/reports/*` pathspec, Q13/Q16/Q18 already resolved) were independently confirmed by code review and security.
