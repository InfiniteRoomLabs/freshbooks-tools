package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- test-only helper reading fixed testdata paths
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func mustLoad(t *testing.T, path string) *Collection {
	t.Helper()
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q) error = %v", path, err)
	}
	return c
}

func mustNormalize(t *testing.T, c *Collection) []Entry {
	t.Helper()
	entries, err := Normalize(c)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	return entries
}

// oneRequest builds a collection holding a single request nested under
// trail[0]/.../trail[n-1] (the top-level folder plus any subfolders), named
// name.
func oneRequest(req *Request, name string, trail ...string) *Collection {
	if len(trail) == 0 {
		panic("oneRequest: at least one trail segment (a top-level folder) is required")
	}
	item := Item{Name: name, Request: req}
	for i := len(trail) - 1; i >= 0; i-- {
		item = Item{Name: trail[i], Item: []Item{item}}
	}
	return &Collection{Item: []Item{item}}
}

// clientsList is the shared do-nothing fixture used by tests that just need
// some valid, minimal collection and don't care about its shape.
func clientsList() *Collection {
	return oneRequest(
		&Request{Method: "GET", URL: URL{Raw: "https://api.freshbooks.com/accounting/account/{{accountId}}/users/clients"}},
		"List Clients", "Clients",
	)
}

func keysOf(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Key
	}
	return out
}

func TestNormalizePathSegments(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"[happy] account var lower-camel already", "/accounting/account/{{accountId}}/systems", "/accounting/account/{accountId}/systems"},
		{"[edge] account var all lowercase", "/accounting/account/{{accountid}}/systems", "/accounting/account/{accountId}/systems"},
		{"[edge] account var pascal case", "/accounting/account/{{AccountID}}/systems", "/accounting/account/{accountId}/systems"},
		{"[happy] business var lower-camel", "/auth/api/v1/businesses/{{businessId}}", "/auth/api/v1/businesses/{businessId}"},
		{"[happy] snake_case var with uuid acronym", "/accounting/businesses/{{business_uuid}}/ledger_accounts", "/accounting/businesses/{businessUuid}/ledger_accounts"},
		{"[edge] invoiceid lowercase whole token", "/accounting/account/{{accountid}}/invoices/{{invoiceid}}", "/accounting/account/{accountId}/invoices/{invoiceId}"},
		{"[corner] hard-coded account id, non-numeric", "/accounting/account/MJx3p/users/clients", "/accounting/account/{accountId}/users/clients"},
		{"[corner] hard-coded numeric business id", "/auth/api/v1/users/business/685582", "/auth/api/v1/users/business/{businessId}"},
		{"[corner] other purely numeric segment", "/accounting/account/{{accountId}}/invoices/invoices/1234", "/accounting/account/{accountId}/invoices/invoices/{id}"},
		{"[sad] non-numeric segment after business is untouched", "/some/business/notanumber", "/some/business/notanumber"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePathSegments(tt.path)
			if got != tt.want {
				t.Errorf("normalizePathSegments(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestNormalizeVarName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"[happy] already lower-camel", "accountId", "accountId"},
		{"[edge] all lowercase glued", "accountid", "accountId"},
		{"[edge] pascal case", "AccountID", "accountId"},
		{"[happy] businessId passthrough", "businessId", "businessId"},
		{"[happy] invoiceid glued", "invoiceid", "invoiceId"},
		{"[corner] snake_case with uuid acronym", "business_uuid", "businessUuid"},
		{"[corner] single word, no acronym", "sku", "sku"},
		{"[corner] single word literally id", "id", "id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeVarName(tt.in)
			if got != tt.want {
				t.Errorf("normalizeVarName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestClassifyFamily(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"[happy] accounting", "/accounting/account/{accountId}/invoices/invoices", FamilyAccounting},
		{"[happy] ledger via businesses", "/accounting/businesses/{businessUuid}/ledger_accounts/accounts", FamilyLedger},
		{"[happy] ledger via ledger_accounts type taxonomy", "/accounting/ledger_accounts/types", FamilyLedger},
		{"[happy] business via projects", "/projects/business/{businessId}/projects", FamilyBusiness},
		{"[happy] business via timetracking", "/timetracking/business/{businessId}/time_entries", FamilyBusiness},
		{"[happy] business via comments", "/comments/business/{businessId}/project/{id}", FamilyBusiness},
		{"[happy] business via auth businesses", "/auth/api/v1/businesses/{businessId}/staffs", FamilyBusiness},
		{"[happy] auth other", "/auth/api/v1/users/me", FamilyAuth},
		{"[happy] events", "/events/account/{accountId}/events/callbacks", FamilyEvents},
		{"[happy] uploads", "/uploads/account/{accountId}/images", FamilyUploads},
		{"[happy] payments", "/payments/account/{accountId}/gateways", FamilyPayments},
		{"[sad] unknown", "/gateway/stripe/payment-method", FamilyUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyFamily(tt.path)
			if got != tt.want {
				t.Errorf("classifyFamily(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestNormalizeStringURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"[happy] with scheme", "https://api.freshbooks.com/accounting/account/{{accountId}}/invoices/invoices?include[]=lines"},
		{"[corner] scheme-less raw URL (two real collection entries have no scheme)", "api.freshbooks.com/accounting/account/{{accountId}}/invoices/invoices?include[]=lines"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := oneRequest(&Request{Method: "get", URL: URL{Raw: tt.raw}}, "Req", "Folder")
			entries := mustNormalize(t, c)
			if len(entries) != 1 {
				t.Fatalf("len(entries) = %d, want 1", len(entries))
			}
			e := entries[0]

			if e.Method != "GET" {
				t.Errorf("Method = %q, want GET", e.Method)
			}
			if e.Host != "api.freshbooks.com" {
				t.Errorf("Host = %q, want api.freshbooks.com", e.Host)
			}
			wantPath := "/accounting/account/{accountId}/invoices/invoices"
			if e.PathTemplate != wantPath {
				t.Errorf("PathTemplate = %q, want %q", e.PathTemplate, wantPath)
			}
			if len(e.Query) != 1 || e.Query[0].Name != "include[]" || e.Query[0].Value != "lines" {
				t.Errorf("Query = %+v", e.Query)
			}
			if e.Family != FamilyAccounting {
				t.Errorf("Family = %q, want %q", e.Family, FamilyAccounting)
			}
		})
	}
}

func TestNormalizeObjectURL(t *testing.T) {
	c := oneRequest(&Request{
		Method: "GET",
		URL: URL{
			Raw:        "https://api.freshbooks.com/accounting/account/{{accountId}}/items/items?search[sku]={{sku}}",
			FromObject: true,
			Query: []QueryParam{
				{Key: "search[sku]", Value: "{{sku}}", Description: "the SKU to search for"},
			},
		},
	}, "Req", "Folder")
	entries := mustNormalize(t, c)
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	q := entries[0].Query
	if len(q) != 1 {
		t.Fatalf("len(Query) = %d, want 1", len(q))
	}
	if q[0].Name != "search[sku]" || q[0].Value != "{sku}" || q[0].Description != "the SKU to search for" {
		t.Errorf("Query[0] = %+v", q[0])
	}
}

func TestNormalizeQueryUnescapeFallback(t *testing.T) {
	// "%zz" is not a valid percent-escape; QueryUnescape errors on it, and
	// the raw segment must survive rather than silently becoming an empty
	// name/value.
	c := oneRequest(&Request{
		Method: "GET",
		URL:    URL{Raw: "https://api.freshbooks.com/accounting/account/{{accountId}}/items/items?%zzbad=%zzalsobad"},
	}, "Req", "Folder")
	entries := mustNormalize(t, c)
	q := entries[0].Query
	if len(q) != 1 {
		t.Fatalf("len(Query) = %d, want 1", len(q))
	}
	if q[0].Name != "%zzbad" || q[0].Value != "%zzalsobad" {
		t.Errorf("Query[0] = %+v, want the raw segment preserved", q[0])
	}
}

func TestNormalizeWhitespaceStripping(t *testing.T) {
	c := oneRequest(&Request{
		Method: "GET",
		URL:    URL{Raw: "https://api.freshbooks.com/accounting/account/{{accountId}}/projects/tasks\n"},
	}, "List Tasks", "Projects", "Tasks")
	entries := mustNormalize(t, c)
	want := "/accounting/account/{accountId}/projects/tasks"
	if entries[0].PathTemplate != want {
		t.Errorf("PathTemplate = %q, want %q", entries[0].PathTemplate, want)
	}
}

func TestNormalizeInternalHostRewrite(t *testing.T) {
	c := oneRequest(&Request{
		Method: "POST",
		URL:    URL{Raw: "https://my.freshbooks.com/service/api/accounting/account/{{accountId}}/journal_entries/journal_entries"},
	}, "Add Journal Entry", "Accounting", "Journal Entries")
	entries := mustNormalize(t, c)
	e := entries[0]

	t.Run("[happy] host rewritten to public", func(t *testing.T) {
		if e.Host != "api.freshbooks.com" {
			t.Errorf("Host = %q", e.Host)
		}
	})
	t.Run("[happy] path strips /service/api prefix", func(t *testing.T) {
		want := "/accounting/account/{accountId}/journal_entries/journal_entries"
		if e.PathTemplate != want {
			t.Errorf("PathTemplate = %q, want %q", e.PathTemplate, want)
		}
	})
	t.Run("[happy] family is internal, not accounting", func(t *testing.T) {
		if e.Family != FamilyInternal {
			t.Errorf("Family = %q, want %q", e.Family, FamilyInternal)
		}
	})
}

func TestNormalizeFolderAndPathTrimming(t *testing.T) {
	c := oneRequest(
		&Request{Method: "GET", URL: URL{Raw: "https://api.freshbooks.com/auth/api/v1/businesses/{{businessId}}/staffs"}},
		" List Team Members ", "My Team ",
	)
	entries := mustNormalize(t, c)
	e := entries[0]

	if e.Folder != "My Team" {
		t.Errorf("Folder = %q, want %q", e.Folder, "My Team")
	}
	if e.Name != "List Team Members" {
		t.Errorf("Name = %q, want %q", e.Name, "List Team Members")
	}
	if e.Key != "My Team/List Team Members" {
		t.Errorf("Key = %q, want %q", e.Key, "My Team/List Team Members")
	}
}

func TestNormalizeNestedSubfolders(t *testing.T) {
	c := oneRequest(
		&Request{Method: "GET", URL: URL{Raw: "https://api.freshbooks.com/accounting/account/{{accountId}}/credits/credits"}},
		"List Credits", "Clients", "Credits",
	)
	entries := mustNormalize(t, c)
	e := entries[0]

	if e.Folder != "Clients" {
		t.Errorf("Folder = %q", e.Folder)
	}
	if len(e.Path) != 1 || e.Path[0] != "Credits" {
		t.Errorf("Path = %v", e.Path)
	}
	if e.Key != "Clients/Credits/List Credits" {
		t.Errorf("Key = %q", e.Key)
	}
}

func TestNormalizeExactDuplicatesCollapse(t *testing.T) {
	req := &Request{Method: "GET", URL: URL{Raw: "https://api.freshbooks.com/accounting/account/{{accountId}}/taxes/taxes/{{taxId}}"}}
	c := &Collection{Item: []Item{
		{Name: "Expenses", Item: []Item{
			{Name: "Single Tax", Request: req},
			{Name: "Single Tax", Request: req},
		}},
	}}
	entries := mustNormalize(t, c)
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1 (exact duplicate collapsed)", len(entries))
	}
	if entries[0].Duplicates != 2 {
		t.Errorf("Duplicates = %d, want 2", entries[0].Duplicates)
	}
	if entries[0].Key != "Expenses/Single Tax" {
		t.Errorf("Key = %q, want unchanged base key (unique base keys never get suffixed)", entries[0].Key)
	}
}

func TestNormalizeConflictingDuplicatesDisambiguateBothSides(t *testing.T) {
	c := &Collection{Item: []Item{
		{Name: "Expenses", Item: []Item{
			{Name: "Single Tax", Request: &Request{Method: "GET", URL: URL{Raw: "https://api.freshbooks.com/accounting/account/{{accountId}}/taxes/taxes/{{taxId}}"}}},
			{Name: "Single Tax", Request: &Request{Method: "DELETE", URL: URL{Raw: "https://api.freshbooks.com/accounting/account/{{accountId}}/taxes/taxes/{{taxId}}"}}},
		}},
	}}
	entries := mustNormalize(t, c)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2 (conflicting names disambiguated, not collapsed)", len(entries))
	}

	byKey := map[string]Entry{}
	for _, e := range entries {
		byKey[e.Key] = e
	}

	if _, ok := byKey["Expenses/Single Tax"]; ok {
		t.Errorf("plain base key %q must not survive a collision; got keys %v", "Expenses/Single Tax", keysOf(entries))
	}
	get, ok := byKey["Expenses/Single Tax (GET)"]
	if !ok {
		t.Fatalf("missing GET-suffixed key entry; got keys %v", keysOf(entries))
	}
	if get.Method != "GET" {
		t.Errorf("GET-suffixed entry Method = %q, want GET", get.Method)
	}
	del, ok := byKey["Expenses/Single Tax (DELETE)"]
	if !ok {
		t.Fatalf("missing DELETE-suffixed key entry; got keys %v", keysOf(entries))
	}
	if del.Method != "DELETE" {
		t.Errorf("DELETE-suffixed entry Method = %q, want DELETE", del.Method)
	}
}

func TestNormalizeConflictingDuplicatesFailWhenDisambiguationCollides(t *testing.T) {
	// Three "Single Tax" entries, all GET but with three different paths:
	// no method-suffix scheme can tell them apart, so Normalize must fail
	// loudly instead of picking one.
	c := &Collection{Item: []Item{
		{Name: "Expenses", Item: []Item{
			{Name: "Single Tax", Request: &Request{Method: "GET", URL: URL{Raw: "https://api.freshbooks.com/accounting/account/{{accountId}}/taxes/taxes/{{taxId}}"}}},
			{Name: "Single Tax", Request: &Request{Method: "GET", URL: URL{Raw: "https://api.freshbooks.com/accounting/account/{{accountId}}/taxes/taxes/other"}}},
			{Name: "Single Tax", Request: &Request{Method: "GET", URL: URL{Raw: "https://api.freshbooks.com/accounting/account/{{accountId}}/taxes/taxes/yet-another"}}},
		}},
	}}
	if _, err := Normalize(c); err == nil {
		t.Fatal("Normalize() error = nil, want an error for an unresolvable key collision")
	}
}

func TestNormalizeMissingTopLevelFolder(t *testing.T) {
	c := &Collection{Item: []Item{
		{Name: "Orphan Request", Request: &Request{Method: "GET", URL: URL{Raw: "https://api.freshbooks.com/auth/api/v1/users/me"}}},
	}}
	if _, err := Normalize(c); err == nil {
		t.Fatal("Normalize() error = nil, want an error for a request with no enclosing folder")
	}
}

func TestURLUnmarshalJSONInvalid(t *testing.T) {
	var u URL
	if err := json.Unmarshal([]byte(`{"raw": 5}`), &u); err == nil {
		t.Fatal("UnmarshalJSON() error = nil, want an error for a malformed url object")
	}
}

func TestNormalizeBadURL(t *testing.T) {
	c := oneRequest(&Request{Method: "GET", URL: URL{Raw: "://not a url"}}, "Req", "Folder")
	if _, err := Normalize(c); err == nil {
		t.Fatal("Normalize() error = nil, want an error for an unparseable URL")
	}
}

func TestNormalizeBodyAndResponses(t *testing.T) {
	body := &Body{Mode: "raw", Raw: `{"client":{"organization":"Acme"}}`}
	c := &Collection{Item: []Item{
		{Name: "Clients", Item: []Item{
			{
				Name: "New Client",
				Request: &Request{
					Method: "POST",
					URL:    URL{Raw: "https://api.freshbooks.com/accounting/account/{{accountId}}/users/clients"},
					Body:   body,
				},
				Response: []Response{
					{Name: "OK", Code: 200, Body: `{"response":{"result":{"client":{}}}}`},
				},
			},
		}},
	}}
	entries := mustNormalize(t, c)
	e := entries[0]

	if e.Body == nil || *e.Body != body.Raw {
		t.Errorf("Body = %v, want %q", e.Body, body.Raw)
	}
	if len(e.Responses) != 1 || e.Responses[0].Status != 200 || e.Responses[0].Name != "OK" {
		t.Errorf("Responses = %+v", e.Responses)
	}
}

func TestNormalizeNoBodyIsNil(t *testing.T) {
	entries := mustNormalize(t, clientsList())
	if entries[0].Body != nil {
		t.Errorf("Body = %v, want nil", entries[0].Body)
	}
}

func TestEntryJSONHasNoNullSlices(t *testing.T) {
	entries := mustNormalize(t, clientsList())
	data, err := json.Marshal(entries[0])
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	for _, field := range []string{`"path":null`, `"query":null`, `"responses":null`} {
		if strings.Contains(string(data), field) {
			t.Errorf("json output contains %q, want empty array instead of null: %s", field, data)
		}
	}
}

func TestWriteAndReadJSONRoundTrip(t *testing.T) {
	entries := mustNormalize(t, clientsList())

	path := filepath.Join(t.TempDir(), "inventory.json")
	if err := WriteJSON(path, entries); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	got, err := ReadJSON(path)
	if err != nil {
		t.Fatalf("ReadJSON() error = %v", err)
	}
	if len(got) != 1 || got[0].Key != entries[0].Key {
		t.Errorf("round-tripped entries = %+v, want %+v", got, entries)
	}
}

func TestWriteJSONIsSortedAndStable(t *testing.T) {
	c := &Collection{Item: []Item{
		{Name: "Zeta", Item: []Item{
			{Name: "Last", Request: &Request{Method: "GET", URL: URL{Raw: "https://api.freshbooks.com/auth/api/v1/users/me"}}},
		}},
		{Name: "Alpha", Item: []Item{
			{Name: "First", Request: &Request{Method: "GET", URL: URL{Raw: "https://api.freshbooks.com/auth/api/v1/users/me"}}},
		}},
	}}
	entries := mustNormalize(t, c)

	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.json")
	p2 := filepath.Join(dir, "b.json")
	if err := WriteJSON(p1, entries); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	// Re-normalize and write again to confirm byte-stable re-emission.
	entries2 := mustNormalize(t, c)
	if err := WriteJSON(p2, entries2); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	got, err := ReadJSON(p1)
	if err != nil {
		t.Fatalf("ReadJSON() error = %v", err)
	}
	if got[0].Key != "Alpha/First" || got[1].Key != "Zeta/Last" {
		t.Fatalf("entries not sorted by key: %v", keysOf(got))
	}

	if mustReadFile(t, p1) != mustReadFile(t, p2) {
		t.Error("re-emitting the same collection did not produce byte-identical output")
	}
}

// validFamilies are the only values classifyFamily/normalizeURL may ever
// assign to Entry.Family.
var validFamilies = map[string]bool{
	FamilyAccounting: true, FamilyBusiness: true, FamilyAuth: true,
	FamilyEvents: true, FamilyUploads: true, FamilyPayments: true,
	FamilyLedger: true, FamilyInternal: true, FamilyUnknown: true,
}

func TestLoadRealCollectionGolden(t *testing.T) {
	c := mustLoad(t, filepath.Join("testdata", "freshbooks.postman_collection.json"))
	entries := mustNormalize(t, c)

	t.Run("[happy] 213 leaf requests parsed, none dropped", func(t *testing.T) {
		total := 0
		for _, e := range entries {
			total += e.Duplicates
		}
		if total != 213 {
			t.Errorf("total leaf requests (sum of Duplicates) = %d, want 213", total)
		}
	})

	t.Run("[happy] per-folder leaf counts match the spec callout", func(t *testing.T) {
		want := map[string]int{
			"Authorization": 4, "Clients": 13, "Invoices": 50, "Expenses": 29,
			"Estimates": 8, "Time Tracking": 7, "Projects": 19, "My Team": 7,
			"Reports": 15, "Accounting": 19, "Uploader": 3, "Webhooks": 5,
			"Settings": 28, "Tokenization": 6,
		}
		got := map[string]int{}
		for _, e := range entries {
			got[e.Folder] += e.Duplicates
		}
		for folder, count := range want {
			if got[folder] != count {
				t.Errorf("folder %q leaf count = %d, want %d", folder, got[folder], count)
			}
		}
	})

	t.Run("[corner] Single Tax name collisions disambiguate both sides", func(t *testing.T) {
		for _, base := range []string{"Expenses/Single Tax", "Settings/Items and Services/Single Tax"} {
			var sawGet, sawDelete, sawPlain bool
			for _, e := range entries {
				switch e.Key {
				case base:
					sawPlain = true
				case base + " (GET)":
					sawGet = e.Method == "GET"
				case base + " (DELETE)":
					sawDelete = e.Method == "DELETE"
				}
			}
			if sawPlain {
				t.Errorf("%s: plain base key must not survive a collision", base)
			}
			if !sawGet || !sawDelete {
				t.Errorf("%s: sawGet=%v sawDelete=%v", base, sawGet, sawDelete)
			}
		}
	})

	t.Run("[happy] every entry is well-formed", func(t *testing.T) {
		seenKeys := make(map[string]bool, len(entries))
		for _, e := range entries {
			if !strings.HasPrefix(e.PathTemplate, "/") {
				t.Errorf("%s: PathTemplate %q does not start with /", e.Key, e.PathTemplate)
			}
			if e.Host == "" {
				t.Errorf("%s: Host is empty", e.Key)
			}
			if strings.Contains(e.PathTemplate, "freshbooks.com") {
				t.Errorf("%s: PathTemplate %q still contains a host", e.Key, e.PathTemplate)
			}
			if strings.Contains(e.PathTemplate, "{{") {
				t.Errorf("%s: PathTemplate %q still contains an unrewritten {{var}}", e.Key, e.PathTemplate)
			}
			if !validFamilies[e.Family] {
				t.Errorf("%s: Family %q is not one of the known constants", e.Key, e.Family)
			}
			if seenKeys[e.Key] {
				t.Errorf("duplicate Key %q in the final entry list", e.Key)
			}
			seenKeys[e.Key] = true
		}
	})

	t.Run("[happy] re-emitting the golden collection matches the committed inventory.json byte-for-byte", func(t *testing.T) {
		dir := t.TempDir()
		out := filepath.Join(dir, "inventory.json")
		if err := WriteJSON(out, entries); err != nil {
			t.Fatalf("WriteJSON() error = %v", err)
		}
		got := mustReadFile(t, out)
		want := mustReadFile(t, filepath.Join("testdata", "inventory.json"))
		if got != want {
			t.Error("re-emitted inventory.json does not match the committed testdata/inventory.json byte-for-byte")
		}
	})
}
