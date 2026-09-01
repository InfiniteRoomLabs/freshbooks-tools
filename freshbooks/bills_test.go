package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestBillsList(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] returns a page of bills", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "accounting", "bills_list")(w, r)
		}))
		page, err := c.Bills.List(ctx, acct, &BillListOptions{Page: 1})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/bills/bills" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(page.Items) != 1 || page.Items[0].VendorID != 5 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Bills.List(ctx, acct, nil); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestBillsAll(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] auto-paginates until a short page", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "bills_list"))
		var got []Bill
		for b, err := range c.Bills.All(ctx, acct, nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, b)
		}
		if len(got) != 1 {
			t.Fatalf("got %d bills", len(got))
		}
	})
}

func TestBillsCreate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] posts the bill payload", func(t *testing.T) {
		var gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			serveFixture(t, http.StatusOK, "accounting", "bills_create")(w, r)
		}))
		bill, err := c.Bills.Create(ctx, acct, &BillCreateRequest{
			VendorID:  5,
			IssueDate: NewDate(fixedClock()),
			Lines: []BillLineRequest{
				{Description: "Widgets", Quantity: 3, UnitCost: Money{Amount: "125.00", Code: "USD"}, CategoryID: 65773},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost {
			t.Fatalf("method = %q", gotMethod)
		}
		if bill.ID != 7 || len(bill.Lines) != 1 {
			t.Fatalf("bill = %+v", bill)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Bills.Create(ctx, acct, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestBillsArchive(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] puts vis_state 2", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "bills_archive")(w, r)
		}))
		bill, err := c.Bills.Archive(ctx, acct, 7)
		if err != nil {
			t.Fatal(err)
		}
		inner, _ := gotBody["bill"].(map[string]any)
		if inner["vis_state"] != float64(VisStateArchived) {
			t.Fatalf("body = %v", gotBody)
		}
		if bill.VisState != VisStateArchived {
			t.Fatalf("bill = %+v", bill)
		}
	})
}

func TestBillsDelete(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] puts vis_state 1, not a real DELETE", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "bills_delete")(w, r)
		}))
		if err := c.Bills.Delete(ctx, acct, 7); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q, want PUT", gotMethod)
		}
		inner, _ := gotBody["bill"].(map[string]any)
		if inner["vis_state"] != float64(VisStateDeleted) {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] a 422 is ErrValidation", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		if err := c.Bills.Delete(ctx, acct, 7); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}
