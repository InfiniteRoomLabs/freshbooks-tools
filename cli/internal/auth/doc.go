// Package auth will hold the freshbooks CLI's login flow: the loopback
// PKCE listener on an ephemeral self-signed TLS certificate, the
// paste-the-URL fallback, and the credential file built on the freshbooks
// library's auth.FileStore. Phase 4 fills this in; see
// docs/authentication.md.
package auth
