# Phase 2 batch a -- gate triage

Lead triage of the three read-only lane reports (`a-code-review.md` REQUEST CHANGES 6 blocking / 6 advisory; `a-simplify.md` 4 apply-recommended / 4 optional; `a-security.md` PASS, 4 advisory). One fix commit: `fix(lib-resources-a): apply the review-gate findings`. QA runs after the fix commit.

## Fix list (apply ALL, one commit)

- **F1** (review 1): switch the six write-path struct fields (`InvoiceCreateRequest.CreateDate`, `InvoiceProfileCreateRequest.CreateDate`, `PaymentCreateRequest.Date`, `InvoiceLine.UnitCost`, `InvoiceLine.Amount`, `ItemCreateRequest.UnitCost`) from `omitempty` to `omitzero`. Add one assertion per affected create/update test that the unset field's key is ABSENT from the serialized body.
- **F2** (review 2): give `UpdateCheckoutLinkGateway` its own request struct carrying `entity_type` (fixed `"checkout_link"`) and `entity_id` (from the path argument) alongside the gateway fields; test asserts both land in the body.
- **F3** (review 3): drop `omitempty` from `PaymentOptionsRequest`'s three bools so `false` serializes (matches every Postman example); test asserts a `false` survives into the body.
- **F4** (review 4 + simplify 4): DELETE the dead `InvoiceListOptions.Sort` field and its copy in `All`. Callers use the `Sort(field, dir)` request option via the `extra` variadic. Name the removal in the CHANGELOG entry.
- **F5** (review 5): `RetainerUpdateRequest.Active` becomes `*bool` (partial-update semantics, matching the four sibling update structs); test asserts a partial update does not carry `"active": false`.
- **F6** (review 6 + security A2 + A3 + simplify 5): move the raw-bytes fetch out of `invoices.go` into `transport.go` and route it through the SAME retry/attempt loop as `do` (429/502/503/504 + `Retry-After`, `MaxDelay` cap). Send `Accept: application/pdf` for the PDF call and reject a 200 whose body is not a PDF (`Content-Type` check or `%PDF-` magic). Doc comment lists exactly what the raw path shares with the JSON path.
- **F7** (simplify 1): add the unexported `listOpts(search, page, perPage)` helper in `page.go`; the five `opts()` bodies delegate to it.
- **F8** (simplify 2): add `newPage[T](items, PageMeta)` in `page.go`; replace the five `Page[T]` literals.
- **F9** (simplify 3): add `(*Client).softDelete(ctx, path, key)` in `transport.go`; the four accounting `Delete` methods delegate to it. Wire bytes unchanged (existing delete tests prove it).
- **F10** (simplify 6): CHANGELOG: drop the unexported-helper bullet; fold the raw-fetch fact into the `Invoices.PDF` mention.
- **F11** (review 7): fix the `invoices.go` doc comment: `Invoices.ToggleGateways` does not exist; say `Invoices.Update` with `AllowedGatewayIDs`.
- **F12** (review 8): remove the dangling `doc.go` pointer in `payments.go`; state the INFERRED payments-family caveat inline in that doc comment instead.
- **F13** (review 9): retainer `Fee`/`ExcessRate` on the two write structs: `float64` -> `json.Number` (wire form stays a JSON number; callers pass exact decimals).
- **F14** (review 10 + review 2 pairing): checkout-link create/update: decode the envelope, then the flat shape; if neither yields an `ID`, return a decode error instead of echoing the caller's request.
- **F15** (review 12): drop `Page`/`PerPage` from `RetainerListOptions` (pagination unconfirmed; no `All` exists). CHANGELOG mention.
- **F16** (security A1, pattern-setting): add a central guard in `(*Client).resolve`: reject a library-built path whose parsed `ref.Path` contains `.`/`..` segments or whose `ref.RawQuery` is non-empty when the path was built from caller-supplied IDs -- concretely, validate string path segments (`AccountID`, checkout-link ids) via one unexported `pathSegment(string) error` helper used by the `*Path` builders before interpolation. Tests: an `AccountID` carrying `?`, `/`, or `..` returns an error, no request made.
- **F17** (security A4): doc-comment warning on the invoice `ShareLink` field/method: the link is credential-equivalent (unauthenticated access to the invoice); handle like a secret in logs/output.

## Explicit non-applies (do NOT do)

- simplify 7 (`writeBody` string-keyed envelope fn): rejected -- keeps typed envelope keys; marginal after F9.
- simplify 8 (`InvoicePresentationDefaults` rename): rejected -- batches b/c/d are naming against the same pattern; revisit repo-wide before v0.1.0 if at all.
- simplify 9-12: agreed DO-NOT-APPLY, do not re-derive.
- review 11 (spec 5.1 `/payments/` envelope callout): NOT yours -- batch d owns that spec section edit; the lead carries your docs evidence (flat `{"payment_options": ...}` per /api/online-payments) into batch d's triage so the callout lands once.

## Lane verdict reconciliation

No lane-vs-lane conflicts. Security PASS stands; its A1 is promoted to a fix (F16) by lead choice because it is pattern-setting for b/c/d. Review's six blockers are all real wire-behavior defects; simplify's four recommendations are behavior-preserving and land in the same commit so later batches inherit the shared seams.
