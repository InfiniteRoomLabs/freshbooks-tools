package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"testing"
)

const testBusinessUUID = BusinessUUID("00000000-0000-4000-8000-000000000001")

func TestLedgerAccountsCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts to the business-scoped path and decodes the flat data envelope", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusCreated, "ledger_accounts", "create")(w, r)
		}))

		got, err := c.LedgerAccounts.Create(ctx, testBusinessUUID, &LedgerAccountCreateRequest{
			Name: "Store Cash", Number: "1002", SubType: "Cash & Bank",
		})
		if err != nil {
			t.Fatal(err)
		}
		wantPath := "/accounting/businesses/" + string(testBusinessUUID) + "/ledger_accounts/accounts"
		if gotPath != wantPath || gotMethod != http.MethodPost {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if gotBody["name"] != "Store Cash" {
			t.Fatalf("body = %v", gotBody)
		}
		if got.UUID != "86b37b85-be3e-41a5-83a5-ec3f2b2b406c" || got.Name != "Store Cash" {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.LedgerAccounts.Create(ctx, testBusinessUUID, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestLedgerAccountsList(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "ledger_accounts", "list"))

	t.Run("[happy] returns every account with no pagination envelope", func(t *testing.T) {
		got, err := c.LedgerAccounts.List(ctx, testBusinessUUID)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Name != "Cash" || got[1].ParentAccount == nil || *got[1].ParentAccount != got[0].UUID {
			t.Fatalf("got = %+v", got)
		}
	})
}

func TestLedgerAccountsGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] returns one account by UUID", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "ledger_accounts", "get")(w, r)
		}))
		got, err := c.LedgerAccounts.Get(ctx, testBusinessUUID, "e17f9556-c99f-4fdb-ad8b-089ac4798bae")
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/businesses/"+string(testBusinessUUID)+"/ledger_accounts/accounts/e17f9556-c99f-4fdb-ad8b-089ac4798bae" {
			t.Fatalf("path = %q", gotPath)
		}
		if got.UUID != "e17f9556-c99f-4fdb-ad8b-089ac4798bae" || got.AutoCreated != true {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] a 404", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error": "Requested resource could not be found."}`)
		}))
		_, err := c.LedgerAccounts.Get(ctx, testBusinessUUID, "missing")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestLedgerAccountsUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts the full account, including sub_accounts, and decodes the result", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "ledger_accounts", "update")(w, r)
		}))
		got, err := c.LedgerAccounts.Update(ctx, testBusinessUUID, "12e355dd-eb0f-40a6-bedb-5b1e1cf38be6", &LedgerAccountUpdateRequest{
			UUID: "12e355dd-eb0f-40a6-bedb-5b1e1cf38be6", Name: "Store Cash", Number: "1001", Type: "asset", SubType: "Cash & Bank",
			SubAccounts: []string{},
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q", gotMethod)
		}
		if _, ok := gotBody["sub_accounts"]; !ok {
			t.Fatalf("body = %v, want sub_accounts present", gotBody)
		}
		if got.Number != "1001" {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.LedgerAccounts.Update(ctx, testBusinessUUID, "x", nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestLedgerAccountsTaxonomy(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] Types has no scope ID in its path and decodes the taxonomy objects", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "ledger_accounts", "types")(w, r)
		}))
		got, err := c.LedgerAccounts.Types(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/ledger_accounts/types" {
			t.Fatalf("path = %q", gotPath)
		}
		// Phase 7 (live): the entries are one-key objects, not the bare
		// strings the Phase 2 fixture guessed, and the income type is
		// spelled "income", not "revenue".
		if len(got) != 5 || got[0].Name != "asset" {
			t.Fatalf("got = %+v", got)
		}
		var names []string
		for _, ty := range got {
			names = append(names, ty.Name)
		}
		if !slices.Contains(names, "income") {
			t.Fatalf("names = %v, want the live taxonomy including income", names)
		}
	})

	t.Run("[happy] SubTypes decodes the id, type, name and base number", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "ledger_accounts", "sub_types"))
		got, err := c.LedgerAccounts.SubTypes(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d sub-types", len(got))
		}
		// id is a bare JSON number on the wire (Phase 7, live), not the
		// quoted string the Phase 2 fixture carried.
		if got[0].ID != 1 || got[0].Type != "asset" || got[0].Name != "Cash & Bank" || got[0].BaseNumber != "1000" {
			t.Fatalf("got[0] = %+v", got[0])
		}
	})

	t.Run("[happy] SubType by id decodes the same object the list returns", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "ledger_accounts", "sub_type")(w, r)
		}))
		got, err := c.LedgerAccounts.SubType(ctx, "1")
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/ledger_accounts/sub_types/1" {
			t.Fatalf("path = %q", gotPath)
		}
		if got.ID != 1 || got.Name != "Cash & Bank" || got.BaseNumber != "1000" {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] SubType rejects an unsafe id", func(t *testing.T) {
		called := false
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		if _, err := c.LedgerAccounts.SubType(ctx, "a/b"); err == nil {
			t.Fatal("want an error")
		}
		if called {
			t.Fatal("a request was made with an unsafe id")
		}
	})
}

func TestLedgerAccountsRejectUnsafeSegments(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		biz  BusinessUUID
		id   string
	}{
		{"[sad] a path separator in the business UUID", "a/b", "x"},
		{"[sad] a query delimiter in the business UUID", "a?b", "x"},
		{"[sad] a fragment delimiter in the business UUID", "a#b", "x"},
		{"[sad] a path separator in the account UUID", testBusinessUUID, "a/b"},
		{"[sad] a query delimiter in the account UUID", testBusinessUUID, "a?b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			if _, err := c.LedgerAccounts.Get(ctx, tc.biz, tc.id); err == nil {
				t.Fatal("want an error")
			}
			if called {
				t.Fatal("a request was made with an unsafe segment")
			}
		})
	}
}
