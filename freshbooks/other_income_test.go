package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestOtherIncomeCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts a nested other_income body", func(t *testing.T) {
		var gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "other_income", "create")(w, r)
		}))

		got, err := c.OtherIncome.Create(ctx, "ACM123", &OtherIncomeCreateRequest{
			Amount: Money{Amount: "113.00", Code: "USD"}, Date: "2019-04-20", Source: "Etsy",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/other_incomes/other_incomes" {
			t.Fatalf("path = %q", gotPath)
		}
		income, ok := gotBody["other_income"].(map[string]any)
		if !ok || income["source"] != "Etsy" {
			t.Fatalf("body = %v", gotBody)
		}
		if got.IncomeID != 2852 || got.Amount.Amount != "113.00" || len(got.Taxes) != 1 {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.OtherIncome.Create(ctx, "ACM123", nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestOtherIncomeList(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "other_income", "list"))

	t.Run("[happy] returns a paginated page", func(t *testing.T) {
		page, err := c.OtherIncome.List(ctx, "ACM123", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 2 || page.Total != 2 || page.PerPage != 15 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[edge] a null note and payment_type", func(t *testing.T) {
		page, err := c.OtherIncome.List(ctx, "ACM123", nil)
		if err != nil {
			t.Fatal(err)
		}
		if page.Items[1].Note != "" || page.Items[1].PaymentType != "" {
			t.Fatalf("got = %+v", page.Items[1])
		}
	})

	t.Run("[happy] a Search filter is applied", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "other_income", "list")(w, r)
		}))
		if _, err := c.OtherIncome.List(ctx, "ACM123", &OtherIncomeListOptions{Search: Search{"category_name": "online_sales"}}); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "search%5Bcategory_name%5D=online_sales" {
			t.Fatalf("query = %q", gotQuery)
		}
	})
}

func TestOtherIncomeAll(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] walks a single page", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "other_income", "list"))
		var got []OtherIncome
		for income, err := range c.OtherIncome.All(ctx, "ACM123", nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, income)
		}
		if len(got) != 2 {
			t.Fatalf("got %d records", len(got))
		}
	})
}

func TestOtherIncomeUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts to the item path", func(t *testing.T) {
		var gotPath, gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			serveFixture(t, http.StatusOK, "other_income", "update")(w, r)
		}))
		source := "Squarespace Site"
		got, err := c.OtherIncome.Update(ctx, "ACM123", 2122, &OtherIncomeUpdateRequest{Source: &source})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/other_incomes/other_incomes/2122" || gotMethod != http.MethodPut {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if got.IncomeID != 2122 {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.OtherIncome.Update(ctx, "ACM123", 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestOtherIncomeDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] PUTs vis_state 1 to the same path Update uses", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "other_income", "delete")(w, r)
		}))
		got, err := c.OtherIncome.Delete(ctx, "ACM123", 2122)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/other_incomes/other_incomes/2122" || gotMethod != http.MethodPut {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		income := gotBody["other_income"].(map[string]any)
		if income["vis_state"].(float64) != float64(VisStateDeleted) {
			t.Fatalf("body = %v", gotBody)
		}
		if got.VisState != VisStateDeleted {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] a 404 on an unknown record", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		_, err := c.OtherIncome.Delete(ctx, "ACM123", 999)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestOtherIncomeRejectUnsafeAccountID(t *testing.T) {
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
			if _, err := c.OtherIncome.Create(ctx, tc.acct, &OtherIncomeCreateRequest{}); err == nil {
				t.Fatal("want an error")
			}
			if _, err := c.OtherIncome.List(ctx, tc.acct, nil); err == nil {
				t.Fatal("want an error")
			}
			if _, err := c.OtherIncome.Update(ctx, tc.acct, 1, &OtherIncomeUpdateRequest{}); err == nil {
				t.Fatal("want an error")
			}
			if called {
				t.Fatal("a request was made with an unsafe account id")
			}
		})
	}
}
