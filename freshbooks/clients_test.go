package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestClientsList(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] returns a page of clients", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "accounting", "clients_list")(w, r)
		}))
		page, err := c.Clients.List(ctx, acct, &ClientListOptions{Include: []string{"recent_contacts"}})
		if err != nil {
			t.Fatal(err)
		}
		if gotQuery != "include%5B%5D=recent_contacts" {
			t.Fatalf("query = %q", gotQuery)
		}
		if len(page.Items) != 1 || page.Items[0].Organization != "Example Signs Co" {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[sad] a 429 is ErrRateLimited", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusTooManyRequests, "accounting", "error_429"))
		if _, err := c.Clients.List(ctx, acct, nil); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestClientsAll(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] auto-paginates until a short page", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "clients_list"))
		var got []Customer
		for cl, err := range c.Clients.All(ctx, acct, nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, cl)
		}
		if len(got) != 1 {
			t.Fatalf("got %d clients", len(got))
		}
	})
}

func TestClientsGet(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] fetches by id", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "accounting", "clients_get")(w, r)
		}))
		cl, err := c.Clients.Get(ctx, acct, 55001)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/users/clients/55001" {
			t.Fatalf("path = %q", gotPath)
		}
		if cl.FirstName != "Alex" || cl.Email != "client@example.com" {
			t.Fatalf("client = %+v", cl)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Clients.Get(ctx, acct, 999); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestClientsCreate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] posts the client payload, including a shipping address", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "clients_create")(w, r)
		}))
		firstName, lastName, email := "Jordan", "NewClient", "newclient@example.com"
		street, city, province, code, country := "2 Example Ave.", "Example City", "California", "94001", "United States"
		cl, err := c.Clients.Create(ctx, acct, &ClientWriteRequest{
			FirstName:        &firstName,
			LastName:         &lastName,
			Email:            &email,
			ShippingStreet:   &street,
			ShippingCity:     &city,
			ShippingProvince: &province,
			ShippingCode:     &code,
			ShippingCountry:  &country,
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost {
			t.Fatalf("method = %q", gotMethod)
		}
		inner, _ := gotBody["client"].(map[string]any)
		if inner["email"] != "newclient@example.com" {
			t.Fatalf("body = %v", gotBody)
		}
		if inner["s_street"] != street || inner["s_city"] != city || inner["s_country"] != country {
			t.Fatalf("shipping address missing from body: %v", gotBody)
		}
		if _, ok := inner["pref_email"]; ok {
			t.Fatal("an unset PrefEmail should be omitted")
		}
		if cl.ID != 55002 {
			t.Fatalf("client = %+v", cl)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Clients.Create(ctx, acct, nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a validation error", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		email := "not-an-email"
		if _, err := c.Clients.Create(ctx, acct, &ClientWriteRequest{Email: &email}); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestClientsUpdate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] puts the changed fields", func(t *testing.T) {
		var gotPath, gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			serveFixture(t, http.StatusOK, "accounting", "clients_update")(w, r)
		}))
		firstName := "Alexandra"
		cl, err := c.Clients.Update(ctx, acct, 55001, &ClientWriteRequest{FirstName: &firstName})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/users/clients/55001" || gotMethod != http.MethodPut {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if cl.FirstName != "Alexandra" {
			t.Fatalf("client = %+v", cl)
		}
	})

	t.Run("[happy] an explicit false PrefEmail is sent, not omitted", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "clients_update")(w, r)
		}))
		prefEmail := false
		if _, err := c.Clients.Update(ctx, acct, 55001, &ClientWriteRequest{PrefEmail: &prefEmail}); err != nil {
			t.Fatal(err)
		}
		inner, _ := gotBody["client"].(map[string]any)
		v, ok := inner["pref_email"]
		if !ok || v != false {
			t.Fatalf("body = %v, want pref_email explicitly false", gotBody)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Clients.Update(ctx, acct, 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestClientsRemoveAllSecondaryContacts(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] sends an empty contacts array", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "clients_remove_contacts")(w, r)
		}))
		cl, err := c.Clients.RemoveAllSecondaryContacts(ctx, acct, 55001)
		if err != nil {
			t.Fatal(err)
		}
		inner, _ := gotBody["client"].(map[string]any)
		contacts, ok := inner["contacts"].([]any)
		if !ok || len(contacts) != 0 {
			t.Fatalf("body = %v", gotBody)
		}
		if len(cl.Contacts) != 0 {
			t.Fatalf("client = %+v", cl)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Clients.RemoveAllSecondaryContacts(ctx, acct, 999); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestClientsRejectUnsafeAccountID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		acct AccountID
	}{
		{"[sad] a path separator", "a/b"},
		{"[sad] a query delimiter", "a?b"},
		{"[sad] a fragment delimiter", "a#b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			if _, err := c.Clients.Get(ctx, tc.acct, 1); err == nil {
				t.Fatal("want an error")
			}
			if called {
				t.Fatal("a request was made with an unsafe account id")
			}
		})
	}
}
