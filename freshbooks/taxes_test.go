package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestTaxesList(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] returns a page of taxes", func(t *testing.T) {
		var gotPath, gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
			serveFixture(t, http.StatusOK, "accounting", "taxes_list")(w, r)
		}))

		page, err := c.Taxes.List(ctx, acct, &TaxListOptions{Search: Search{"name": "HST"}, Page: 2, PerPage: 10})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/taxes/taxes" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotQuery != "page=2&per_page=10&search%5Bname%5D=HST" {
			t.Fatalf("query = %q", gotQuery)
		}
		if len(page.Items) != 2 || page.Total != 2 {
			t.Fatalf("page = %+v", page)
		}
		if page.Items[0].Name != "HST" || page.Items[0].Amount != "13" {
			t.Fatalf("tax = %+v", page.Items[0])
		}
	})

	t.Run("[edge] nil options sends no query", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "accounting", "taxes_list")(w, r)
		}))
		if _, err := c.Taxes.List(ctx, acct, nil); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "" {
			t.Fatalf("query = %q, want empty", gotQuery)
		}
	})

	t.Run("[sad] a 429 maps to ErrRateLimited", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusTooManyRequests, "accounting", "error_429"))
		if _, err := c.Taxes.List(ctx, acct, nil); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTaxesAll(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] auto-paginates until a short page", func(t *testing.T) {
		var calls int
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			serveFixture(t, http.StatusOK, "accounting", "taxes_list")(w, r)
		}))
		var got []Tax
		for tax, err := range c.Taxes.All(ctx, acct, &TaxListOptions{PerPage: 2}) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, tax)
		}
		// pages == 1 in the fixture, so All stops after the first call.
		if calls != 1 {
			t.Fatalf("calls = %d", calls)
		}
		if len(got) != 2 {
			t.Fatalf("got %d taxes", len(got))
		}
	})
}

func TestTaxesGet(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] fetches by id", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "accounting", "taxes_get")(w, r)
		}))
		tax, err := c.Taxes.Get(ctx, acct, 4001)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/taxes/taxes/4001" {
			t.Fatalf("path = %q", gotPath)
		}
		if tax.ID != 4001 || tax.Name != "HST" || tax.Compound {
			t.Fatalf("tax = %+v", tax)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Taxes.Get(ctx, acct, 999); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTaxesCreate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] posts the tax payload", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "taxes_create")(w, r)
		}))
		tax, err := c.Taxes.Create(ctx, acct, &TaxCreateRequest{Name: "RST"})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost {
			t.Fatalf("method = %q", gotMethod)
		}
		inner, _ := gotBody["tax"].(map[string]any)
		if inner["name"] != "RST" {
			t.Fatalf("body = %v", gotBody)
		}
		if _, ok := inner["amount"]; ok {
			t.Fatal("an unset optional should be omitted")
		}
		if tax.Name != "RST" {
			t.Fatalf("tax = %+v", tax)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Taxes.Create(ctx, acct, nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a duplicate name is ErrValidation", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		if _, err := c.Taxes.Create(ctx, acct, &TaxCreateRequest{Name: "HST"}); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTaxesUpdate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] puts the changed fields", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "taxes_update")(w, r)
		}))
		name := "HST Updated"
		compound := true
		tax, err := c.Taxes.Update(ctx, acct, 4001, &TaxUpdateRequest{Name: &name, Compound: &compound})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/taxes/taxes/4001" || gotMethod != http.MethodPut {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		inner, _ := gotBody["tax"].(map[string]any)
		if inner["name"] != "HST Updated" || inner["compound"] != true {
			t.Fatalf("body = %v", gotBody)
		}
		if tax.Amount != "14" {
			t.Fatalf("tax = %+v", tax)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Taxes.Update(ctx, acct, 4001, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestTaxesDelete(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] issues a real DELETE, not a vis_state PUT", func(t *testing.T) {
		var gotMethod, gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"response": {}}`))
		}))
		if err := c.Taxes.Delete(ctx, acct, 4001); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodDelete {
			t.Fatalf("method = %q, want DELETE", gotMethod)
		}
		if gotPath != "/accounting/account/ACM123/taxes/taxes/4001" {
			t.Fatalf("path = %q", gotPath)
		}
	})

	t.Run("[sad] a 403 is ErrForbidden", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"response": {"errors": [{"errno": 1000, "message": "forbidden"}]}}`))
		}))
		if err := c.Taxes.Delete(ctx, acct, 4001); !errors.Is(err, ErrForbidden) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTaxesRejectUnsafeAccountID(t *testing.T) {
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
			if _, err := c.Taxes.Get(ctx, tc.acct, 1); err == nil {
				t.Fatal("want an error")
			}
			if called {
				t.Fatal("a request was made with an unsafe account id")
			}
		})
	}
}
