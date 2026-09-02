package auth

// DefaultScopes is the user:<object>:<read|write> scope set the CLI requests
// on `auth login` when the caller does not pass --scopes: every scope the
// FreshBooks developer portal actually offers a normal app, one entry per
// object and action.
//
// It is deliberately NOT the cross product of the object list published at
// https://www.freshbooks.com/api/scopes. That page is incomplete in both
// directions (Phase 7, live, 2026-09-02): three of its objects have no write
// scope at all, and it omits objects the portal does offer. Requesting a
// scope that does not exist fails the whole consent with "The requested
// scope is invalid, unknown, or malformed", so the default set has to match
// the portal, not the docs page.
//
// user:profile:read is always granted by FreshBooks regardless of what is
// requested, but it is listed explicitly so the authorization screen shows
// the CLI's full intent. Every scope requested must ALSO be enabled on the
// app in the developer portal, or consent fails the same way.
var DefaultScopes = buildDefaultScopes()

// readWriteScopeObjects are the objects the portal offers with both a read
// and a write scope: the documented list minus the three read-only objects
// below, plus the undocumented "uploads" (the library's three upload
// endpoints need it).
//
// The portal also offers "account" and "riskhub" (both read/write) and an
// mcp:* family. None of them is documented, no endpoint in this repo needs
// them, and an unnecessary scope on a consent screen is a cost, so they stay
// out of the default set; pass --scopes explicitly to request them.
var readWriteScopeObjects = []string{
	"bill_payments",
	"bill_vendors",
	"billable_items",
	"bills",
	"business",
	"clients",
	"credit_notes",
	"estimates",
	"expenses",
	"invoices",
	"journal_entries",
	"online_payments",
	"other_income",
	"payments",
	"projects",
	"retainers",
	"taxes",
	"teams",
	"time_entries",
	"uploads",
}

// readOnlyScopeObjects are the objects the portal offers with a read scope
// only. There is no user:profile:write, user:notifications:write, or
// user:reports:write to request; asking for one rejects the whole consent.
var readOnlyScopeObjects = []string{
	"notifications",
	"profile",
	"reports",
}

func buildDefaultScopes() []string {
	scopes := make([]string, 0, len(readWriteScopeObjects)*2+len(readOnlyScopeObjects))
	for _, obj := range readWriteScopeObjects {
		scopes = append(scopes, "user:"+obj+":read", "user:"+obj+":write")
	}
	for _, obj := range readOnlyScopeObjects {
		scopes = append(scopes, "user:"+obj+":read")
	}
	return scopes
}
