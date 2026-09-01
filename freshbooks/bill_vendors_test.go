package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestBillVendorsList(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] returns a page of vendors, decoding object tax defaults and the outstanding balance", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "bill_vendors_list"))
		page, err := c.BillVendors.List(ctx, acct, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Items[0].VendorName != "Example Supplies Co" {
			t.Fatalf("page = %+v", page)
		}
		v := page.Items[0]
		if len(v.TaxDefaults) != 1 || v.TaxDefaults[0].Name != "HST" || v.TaxDefaults[0].TaxID != 4001 {
			t.Fatalf("tax defaults = %+v", v.TaxDefaults)
		}
		if v.OutstandingBalance == nil || v.OutstandingBalance.Amount != "375.00" {
			t.Fatalf("outstanding balance = %+v", v.OutstandingBalance)
		}
	})

	t.Run("[sad] a 429 is ErrRateLimited", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusTooManyRequests, "accounting", "error_429"))
		if _, err := c.BillVendors.List(ctx, acct, nil); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestBillVendorsAll(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] auto-paginates across two pages", func(t *testing.T) {
		var calls int
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.URL.Query().Get("page") == "2" {
				serveFixture(t, http.StatusOK, "accounting", "bill_vendors_list_page2")(w, r)
				return
			}
			serveFixture(t, http.StatusOK, "accounting", "bill_vendors_list")(w, r)
		}))
		var got []BillVendor
		for v, err := range c.BillVendors.All(ctx, acct, nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, v)
		}
		if calls != 2 {
			t.Fatalf("calls = %d, want 2", calls)
		}
		if len(got) != 2 || got[0].VendorID != 5 || got[1].VendorID != 6 {
			t.Fatalf("got %d vendors: %+v", len(got), got)
		}
	})
}

func TestBillVendorsCreate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] posts the vendor payload, omitting an unset Is1099", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "bill_vendors_create")(w, r)
		}))
		v, err := c.BillVendors.Create(ctx, acct, &BillVendorRequest{VendorName: "Example Supplies Co"})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost {
			t.Fatalf("method = %q", gotMethod)
		}
		inner, _ := gotBody["bill_vendor"].(map[string]any)
		if inner["vendor_name"] != "Example Supplies Co" {
			t.Fatalf("body = %v", gotBody)
		}
		if _, ok := inner["is_1099"]; ok {
			t.Fatal("an unset Is1099 should be omitted")
		}
		if v.VendorID != 5 {
			t.Fatalf("vendor = %+v", v)
		}
	})

	t.Run("[happy] an explicit false Is1099 is sent, not omitted", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "bill_vendors_create")(w, r)
		}))
		is1099 := false
		if _, err := c.BillVendors.Create(ctx, acct, &BillVendorRequest{VendorName: "Example Supplies Co", Is1099: &is1099}); err != nil {
			t.Fatal(err)
		}
		inner, _ := gotBody["bill_vendor"].(map[string]any)
		v, ok := inner["is_1099"]
		if !ok || v != false {
			t.Fatalf("body = %v, want is_1099 explicitly false", gotBody)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.BillVendors.Create(ctx, acct, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestBillVendorsUpdate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] puts the vendor payload", func(t *testing.T) {
		var gotPath, gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			serveFixture(t, http.StatusOK, "accounting", "bill_vendors_create")(w, r)
		}))
		if _, err := c.BillVendors.Update(ctx, acct, 5, &BillVendorRequest{VendorName: "Example Supplies Co"}); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/bill_vendors/bill_vendors/5" || gotMethod != http.MethodPut {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.BillVendors.Update(ctx, acct, 5, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestBillVendorsDelete(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] soft-deletes via a vis_state PUT, not a real DELETE", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "bill_vendors_delete")(w, r)
		}))
		if err := c.BillVendors.Delete(ctx, acct, 5); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q, want PUT", gotMethod)
		}
		inner, _ := gotBody["bill_vendor"].(map[string]any)
		if inner["vis_state"] != float64(VisStateDeleted) {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] a 403 is ErrForbidden", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"response": {"errors": [{"errno": 1000, "message": "forbidden"}]}}`))
		}))
		if err := c.BillVendors.Delete(ctx, acct, 5); !errors.Is(err, ErrForbidden) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestBillVendorsRejectUnsafeAccountID(t *testing.T) {
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
			if _, err := c.BillVendors.List(ctx, tc.acct, nil); err == nil {
				t.Fatal("want an error")
			}
			if called {
				t.Fatal("a request was made with an unsafe account id")
			}
		})
	}
}
