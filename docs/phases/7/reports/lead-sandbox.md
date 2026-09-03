# Phase 7 lead decision -- account and write policy (2026-09-02)

The authorized account is IRL's real books (one membership, role `owner`). **No sandbox exists.** Write facts G, H, I, J2, K, L, M, R are **DEFERRED**: do not create, update, or delete anything against this account. Read facts are authorized with the 45 scopes granted on 2026-09-02 (token valid 12h; `freshbooks auth token` refreshes through the store).

## Scope reality (portal vs docs), for the CLI default-scope fix

The developer portal's pickable scope list is NOT the docs page's 22 objects x {read, write}:

- Read-only objects in the portal: `profile`, `notifications`, `reports` (no `:write` exists). The CLI's default set requests `user:profile:write`, `user:notifications:write`, `user:reports:write`, and FreshBooks answers the consent with "The requested scope is invalid, unknown, or malformed" for the whole request. **This is the shipped bug**; the app-registration explanation in `lead-stage1.md` finding 1 is only half the story (both hold: a scope must exist AND be enabled on the app).
- Objects the portal has that the docs page does not list: `account` (read/write), `uploads` (read/write), `riskhub` (read/write), plus `mcp:*` scopes. `uploads` matters (the lib has three upload endpoints); `account` and `riskhub` are undocumented and not requested.
- Everything else matches the docs list, including `business` (read/write).

The 45-scope set that was granted: all 22 documented objects with `read`, `write` for all but `profile`/`notifications`/`reports`, plus `account:read/write` and `uploads:read/write`.

## Fix order for the implementer (lib/CLI, its own commit)

- `cli/internal/auth/scopes.go`: the default set becomes the grantable set -- drop the three nonexistent `:write` scopes, add `user:uploads:read` and `user:uploads:write`; keep `account`/`riskhub` out with a comment naming them. `TestDefaultScopes` pins the new count (43) and asserts the three absent strings. Update the docs header sentence added in `5411fce` and `docs/getting-started.md` step 3 to say the docs page's object list is incomplete and the portal is the authority.
- `docs/authentication.md`: one paragraph on scopes: per-object read/write, the three read-only objects, the undocumented `uploads`, and "must be enabled on the app".
- Spec section 3: a `> **STATE AS OF 2026-09-02 (Phase 7, live)**` callout with the portal scope list.
