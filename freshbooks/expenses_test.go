package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestExpensesList(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] returns a page of expenses", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "expenses_list"))
		page, err := c.Expenses.List(ctx, acct, &ExpenseListOptions{Search: Search{"clientid": "0"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Items[0].Vendor != "Example Fuel Co" {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[happy] decodes the Phase 8 convergence fields", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "expenses_list"))
		page, err := c.Expenses.List(ctx, acct, nil)
		if err != nil {
			t.Fatal(err)
		}
		exp := page.Items[0]
		if !exp.Billable {
			t.Fatal("Billable = false, want true")
		}
		if exp.AccountingSystemID != "ACM123" {
			t.Fatalf("AccountingSystemID = %q", exp.AccountingSystemID)
		}
		if exp.LegacyAccountID != nil {
			t.Fatalf("LegacyAccountID = %v, want nil (captured null)", exp.LegacyAccountID)
		}
		if exp.BackgroundJobID != nil || exp.ConverseProjectID != nil || exp.ModernProjectID != nil || exp.ExtAccountID != nil {
			t.Fatalf("a captured-null field decoded non-nil: %+v", exp)
		}
		if exp.BillMatches == nil || len(exp.BillMatches) != 0 {
			t.Fatalf("BillMatches = %v, want a non-nil empty slice", exp.BillMatches)
		}
		if exp.Version != "2026-08-22 04:32:55.000000" {
			t.Fatalf("Version = %q", exp.Version)
		}
	})

	t.Run("[sad] a 429 is ErrRateLimited", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusTooManyRequests, "accounting", "error_429"))
		if _, err := c.Expenses.List(ctx, acct, nil); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestExpensesAll(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] auto-paginates until a short page", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "expenses_list"))
		var got []Expense
		for e, err := range c.Expenses.All(ctx, acct, nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, e)
		}
		if len(got) != 1 {
			t.Fatalf("got %d expenses", len(got))
		}
	})
}

func TestExpensesGet(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] fetches by id", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "accounting", "expenses_get")(w, r)
		}))
		exp, err := c.Expenses.Get(ctx, acct, 1825574)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/expenses/expenses/1825574" {
			t.Fatalf("path = %q", gotPath)
		}
		if exp.TaxName1 != "HST" || exp.TaxPercent1 != "13" {
			t.Fatalf("expense = %+v", exp)
		}
		if exp.Attachment == nil || exp.Attachment.AttachmentID != 8668 || exp.Attachment.ID != 8668 {
			t.Fatalf("attachment = %+v", exp.Attachment)
		}
		if exp.Category == nil || exp.Category.Category != "Gas" {
			t.Fatalf("category = %+v", exp.Category)
		}
		if exp.TransactionID == nil || *exp.TransactionID != 900123 {
			t.Fatalf("transactionid = %v", exp.TransactionID)
		}
		if exp.IsDuplicate == nil || !*exp.IsDuplicate {
			t.Fatalf("isduplicate = %v", exp.IsDuplicate)
		}
		if exp.FromBulkImport == nil || *exp.FromBulkImport {
			t.Fatalf("from_bulk_import = %v", exp.FromBulkImport)
		}
		if exp.ProfileID != nil {
			t.Fatalf("profileid = %v, want nil", exp.ProfileID)
		}
		if exp.Billable {
			t.Fatal("Billable = true, want false (this fixture's captured value)")
		}
		if exp.ExtInvoiceID != 0 || exp.ExtSystemID != 0 {
			t.Fatalf("ExtInvoiceID/ExtSystemID = %d/%d, want 0/0", exp.ExtInvoiceID, exp.ExtSystemID)
		}
		if exp.PotentialBillPayment {
			t.Fatal("PotentialBillPayment = true, want false")
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Expenses.Get(ctx, acct, 999); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestExpensesCreate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] posts the expense payload, categoryid as an int", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "expenses_create")(w, r)
		}))
		categoryID := int64(65679)
		exp, err := c.Expenses.Create(ctx, acct, &ExpenseWriteRequest{
			Amount:     &Money{Amount: "79.73", Code: "USD"},
			Date:       func() *Date { d := NewDate(fixedClock()); return &d }(),
			CategoryID: &categoryID,
			Vendor:     "Example Fuel Co",
		})
		if err != nil {
			t.Fatal(err)
		}
		inner, _ := gotBody["expense"].(map[string]any)
		if inner["categoryid"] != float64(65679) {
			t.Fatalf("body = %v", gotBody)
		}
		if _, ok := inner["projectid"]; ok {
			t.Fatal("an unset ProjectID should be omitted")
		}
		if exp.ExpenseID != 1825575 {
			t.Fatalf("expense = %+v", exp)
		}
	})

	t.Run("[happy] TaxPercent1 and MarkupPercent are sent as strings", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "expenses_create")(w, r)
		}))
		taxPercent1, markup := "13", "20"
		if _, err := c.Expenses.Create(ctx, acct, &ExpenseWriteRequest{
			Amount:        &Money{Amount: "79.73", Code: "USD"},
			TaxPercent1:   &taxPercent1,
			MarkupPercent: &markup,
		}); err != nil {
			t.Fatal(err)
		}
		inner, _ := gotBody["expense"].(map[string]any)
		if inner["taxPercent1"] != "13" || inner["markup_percent"] != "20" {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Expenses.Create(ctx, acct, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestExpensesUpdate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] puts the changed fields", func(t *testing.T) {
		var gotPath, gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			serveFixture(t, http.StatusOK, "accounting", "expenses_update")(w, r)
		}))
		exp, err := c.Expenses.Update(ctx, acct, 1825574, &ExpenseWriteRequest{Vendor: "Updated Fuel Co"})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/expenses/expenses/1825574" || gotMethod != http.MethodPut {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if exp.Vendor != "Updated Fuel Co" {
			t.Fatalf("expense = %+v", exp)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Expenses.Update(ctx, acct, 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestExpensesDelete(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] soft-deletes via vis_state 1, not 0", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"response": {}}`))
		}))
		if err := c.Expenses.Delete(ctx, acct, 1825574); err != nil {
			t.Fatal(err)
		}
		inner, _ := gotBody["expense"].(map[string]any)
		if inner["vis_state"] != float64(VisStateDeleted) {
			t.Fatalf("body = %v, want vis_state 1", gotBody)
		}
	})

	t.Run("[sad] a 403 is ErrForbidden", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"response": {"errors": [{"errno": 1000, "message": "forbidden"}]}}`))
		}))
		if err := c.Expenses.Delete(ctx, acct, 1); !errors.Is(err, ErrForbidden) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestExpensesSummaries(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] returns bucketed totals", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "accounting", "expenses_summaries")(w, r)
		}))
		summaries, err := c.Expenses.Summaries(ctx, acct)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/expenses/summaries" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(summaries) != 3 || summaries[0].ID != "grand_total" || summaries[0].Amounts[0].Amount != "159.46" {
			t.Fatalf("summaries = %+v", summaries)
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.Expenses.Summaries(ctx, acct); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestExpensesVendors(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] returns distinct vendor names", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "accounting", "expenses_vendors")(w, r)
		}))
		vendors, err := c.Expenses.Vendors(ctx, acct)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/expenses/vendors" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(vendors) != 2 || vendors[0] != "Example Fuel Co" {
			t.Fatalf("vendors = %v", vendors)
		}
	})

	t.Run("[edge] walks every page of the paginated vendor list", func(t *testing.T) {
		// Phase 7 (live): the endpoint paginates at 15 per page, so a
		// single-request implementation truncates silently.
		var pages []string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			page := r.URL.Query().Get("page")
			pages = append(pages, page)
			w.Header().Set("Content-Type", "application/json")
			body := `{"response":{"result":{"page":1,"pages":2,"per_page":1,"total":2,"vendors":[{"vendor":"First Vendor"}]}}}`
			if page == "2" {
				body = `{"response":{"result":{"page":2,"pages":2,"per_page":1,"total":2,"vendors":[{"vendor":"Second Vendor"}]}}}`
			}
			_, _ = w.Write([]byte(body))
		}))
		vendors, err := c.Expenses.Vendors(ctx, acct)
		if err != nil {
			t.Fatal(err)
		}
		if len(vendors) != 2 || vendors[0] != "First Vendor" || vendors[1] != "Second Vendor" {
			t.Fatalf("vendors = %v", vendors)
		}
		if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
			t.Fatalf("requested pages = %v", pages)
		}
	})

	t.Run("[edge] stops on an empty page rather than looping forever", func(t *testing.T) {
		calls := 0
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"response":{"result":{"page":1,"pages":9,"per_page":15,"total":0,"vendors":[]}}}`))
		}))
		vendors, err := c.Expenses.Vendors(ctx, acct)
		if err != nil {
			t.Fatal(err)
		}
		if len(vendors) != 0 {
			t.Fatalf("vendors = %v", vendors)
		}
		// Non-nil: the MCP tool and the CLI command marshal this value
		// directly, and a nil slice would put null on the wire where the
		// pre-Phase-7 decode of "vendors": [] produced [].
		if vendors == nil {
			t.Fatal("vendors = nil, want an empty non-nil slice")
		}
		encoded, err := json.Marshal(vendors)
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != "[]" {
			t.Fatalf("json.Marshal(vendors) = %s, want []", encoded)
		}
		if calls != 1 {
			t.Fatalf("calls = %d, want 1", calls)
		}
	})

	t.Run("[sad] surfaces the transport error", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthenticated"}`))
		}))
		if _, err := c.Expenses.Vendors(ctx, acct); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestExpensesCreateRecurring(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] posts the recurring-expense payload", func(t *testing.T) {
		var gotPath, gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			serveFixture(t, http.StatusOK, "accounting", "expenses_recurring_create")(w, r)
		}))
		profile, err := c.Expenses.CreateRecurring(ctx, acct, &ExpenseProfileCreateRequest{
			Frequency: "m",
			StartDate: NewDate(fixedClock()),
			Amount:    Money{Amount: "100.00", Code: "USD"},
			Vendor:    "Example Recurring Vendor",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/expense_profiles/expense_profiles" || gotMethod != http.MethodPost {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if profile.ProfileID != 4242 {
			t.Fatalf("profile = %+v", profile)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Expenses.CreateRecurring(ctx, acct, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestExpensesRejectUnsafeAccountID(t *testing.T) {
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
			if _, err := c.Expenses.Get(ctx, tc.acct, 1); err == nil {
				t.Fatal("want an error")
			}
			if called {
				t.Fatal("a request was made with an unsafe account id")
			}
		})
	}
}
