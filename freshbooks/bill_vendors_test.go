package freshbooks

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestBillVendorsList(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] returns a page of vendors", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "bill_vendors_list"))
		page, err := c.BillVendors.List(ctx, acct, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Items[0].VendorName != "Example Supplies Co" {
			t.Fatalf("page = %+v", page)
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

	t.Run("[happy] auto-paginates until a short page", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "bill_vendors_list"))
		var got []BillVendor
		for v, err := range c.BillVendors.All(ctx, acct, nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, v)
		}
		if len(got) != 1 {
			t.Fatalf("got %d vendors", len(got))
		}
	})
}

func TestBillVendorsCreate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] posts the vendor payload", func(t *testing.T) {
		var gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			serveFixture(t, http.StatusOK, "accounting", "bill_vendors_create")(w, r)
		}))
		v, err := c.BillVendors.Create(ctx, acct, &BillVendorRequest{VendorName: "Example Supplies Co"})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost {
			t.Fatalf("method = %q", gotMethod)
		}
		if v.VendorID != 5 {
			t.Fatalf("vendor = %+v", v)
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
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			serveFixture(t, http.StatusOK, "accounting", "bill_vendors_delete")(w, r)
		}))
		if err := c.BillVendors.Delete(ctx, acct, 5); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q, want PUT", gotMethod)
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
