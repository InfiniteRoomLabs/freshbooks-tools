package auth

// DefaultScopes is the full user:<object>:<read|write> scope set the CLI
// requests on `auth login` when the caller does not pass --scopes: every
// object FreshBooks documents at https://www.freshbooks.com/api/scopes,
// both actions each. user:profile:read is always granted by FreshBooks
// regardless of what is requested, but it is listed explicitly so the
// authorization screen shows the CLI's full intent.
var DefaultScopes = buildDefaultScopes()

// scopeObjects are the ~22 objects https://www.freshbooks.com/api/scopes
// documents, each carrying both a read and a write scope.
var scopeObjects = []string{
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
	"notifications",
	"online_payments",
	"other_income",
	"payments",
	"profile",
	"projects",
	"reports",
	"retainers",
	"taxes",
	"teams",
	"time_entries",
}

func buildDefaultScopes() []string {
	scopes := make([]string, 0, len(scopeObjects)*2)
	for _, obj := range scopeObjects {
		scopes = append(scopes, "user:"+obj+":read", "user:"+obj+":write")
	}
	return scopes
}
